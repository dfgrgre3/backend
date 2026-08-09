package protected

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"encoding/json"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"
	"thanawy-backend/internal/infrastructure/storage"

	"bytes"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ─── Constants ────────────────────────────────────────────────────────────────

const (
	maxSimpleUploadSize = 50 << 20  // 50 MB
	maxChunkSize        = 100 << 20 // 100 MB per chunk
)

// ─── Allowed Types ─────────────────────────────────────────────────────────────

var allowedUploadCategories = map[string]uploadCategory{
	"image": {
		exts:  []string{".jpg", ".jpeg", ".jfif", ".png", ".gif", ".webp", ".avif", ".svg", ".heic", ".heif"},
		mimes: []string{"image/jpeg", "image/png", "image/gif", "image/webp", "image/avif", "image/svg+xml", "image/heic", "image/heif"},
	},
	"video": {
		exts:  []string{".mp4", ".m4v", ".mov", ".avi", ".mkv", ".webm", ".ts", ".flv"},
		mimes: []string{"video/mp4", "video/quicktime", "video/x-msvideo", "video/x-matroska", "video/webm", "video/x-m4v", "video/mp2t", "video/x-flv"},
	},
	"audio": {
		exts:  []string{".mp3", ".wav", ".ogg", ".aac", ".m4a"},
		mimes: []string{"audio/mpeg", "audio/wav", "audio/ogg", "audio/aac", "audio/mp4"},
	},
	"document": {
		exts:  []string{".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt"},
		mimes: []string{"application/pdf", "application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/vnd.ms-excel", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "application/vnd.ms-powerpoint", "application/vnd.openxmlformats-officedocument.presentationml.presentation", "text/plain"},
	},
	"any": {
		// Superset of every category: everything listed above is accepted.
		exts:  []string{".jpg", ".jpeg", ".jfif", ".png", ".gif", ".webp", ".avif", ".svg", ".heic", ".heif", ".mp4", ".m4v", ".mov", ".avi", ".mkv", ".webm", ".ts", ".flv", ".mp3", ".wav", ".ogg", ".aac", ".m4a", ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt"},
		mimes: []string{},
	},
}

// Default MIME types used when the client omits an explicit Content-Type
// (browsers leave file.type empty for several container formats, e.g. .mkv).
var extDefaultMime = map[string]string{
	".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".jfif": "image/jpeg",
	".png": "image/png", ".gif": "image/gif", ".webp": "image/webp",
	".avif": "image/avif", ".svg": "image/svg+xml",
	".heic": "image/heic", ".heif": "image/heif",
	".mp4": "video/mp4", ".m4v": "video/x-m4v", ".mov": "video/quicktime",
	".avi": "video/x-msvideo", ".mkv": "video/x-matroska", ".webm": "video/webm",
	".ts": "video/mp2t", ".flv": "video/x-flv",
	".mp3": "audio/mpeg", ".wav": "audio/wav", ".ogg": "audio/ogg",
	".aac": "audio/aac", ".m4a": "audio/mp4",
	".pdf": "application/pdf", ".doc": "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".ppt":  "application/vnd.ms-powerpoint",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".txt":  "text/plain",
}

func defaultMimeForExt(ext string) string {
	if mime, ok := extDefaultMime[strings.ToLower(ext)]; ok {
		return mime
	}
	return "application/octet-stream"
}

// Folders in storage bucket by context
var uploadFolders = map[string]string{
	"avatar":       "avatars",
	"cover":        "covers",
	"book":         "books",
	"book_cover":   "books/covers",
	"lesson":       "lessons",
	"exam":         "exams",
	"attachment":   "attachments",
	"announcement": "announcements",
	"blog":         "blog",
	"general":      "uploads",
}

type uploadCategory struct {
	exts  []string
	mimes []string
}

// ─── Request/Response types ────────────────────────────────────────────────────

type presignRequest struct {
	FileName    string `json:"fileName" binding:"required"`
	ContentType string `json:"contentType"`
	FileSize    int64  `json:"fileSize"`
	Context     string `json:"context"`  // avatar, cover, book, lesson, etc.
	Category    string `json:"category"` // image, video, audio, document, any
}

type chunkedInitRequest struct {
	FileName  string `json:"fileName" binding:"required"`
	FileType  string `json:"fileType" binding:"required"`
	FileSize  int64  `json:"fileSize" binding:"required"`
	ChunkSize int    `json:"chunkSize"`
	Context   string `json:"context"`
	Category  string `json:"category"`
}

type chunkedMergeRequest struct {
	UploadID string `json:"uploadId" binding:"required"`
}

type chunkedChunkMetadata struct {
	FileName  string `json:"fileName"`
	FileType  string `json:"fileType"`
	FileSize  int64  `json:"fileSize"`
	ChunkSize int    `json:"chunkSize"`
	Context   string `json:"context"`
}

type chunkedChunkEntry struct {
	index int
	path  string
}

// ─── Upload (simple multipart) ──────────────────────────────────────────────────

// Upload handles simple multipart file upload (up to 50MB)
func Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "No file uploaded (field: 'file')")
		return
	}

	if file.Size > maxSimpleUploadSize {
		api_response.Error(c, http.StatusBadRequest, fmt.Sprintf("File too large. Max size is %dMB. Use /upload/chunked for larger files.", maxSimpleUploadSize>>20))
		return
	}

	uploadCtx := c.DefaultPostForm("context", "general")
	category := c.DefaultPostForm("category", "any")

	folder := resolveFolder(uploadCtx)
	ext := strings.ToLower(filepath.Ext(file.Filename))

	if !isExtAllowed(ext, category) {
		api_response.Error(c, http.StatusBadRequest, fmt.Sprintf("File extension '%s' is not allowed for category '%s'", ext, category))
		return
	}

	f, err := file.Open()
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to open uploaded file")
		return
	}
	defer f.Close()

	// Magic number validation for non-SVG files
	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		contentType = defaultMimeForExt(ext)
	}
	if ext != ".svg" && ext != ".txt" {
		allowedMimes := getMimesForCategory(category)
		if len(allowedMimes) > 0 {
			isValid, detectedMime, valErr := db.ValidateMagicNumber(f, allowedMimes)
			if valErr != nil || !isValid {
				api_response.Error(c, http.StatusBadRequest, fmt.Sprintf("File content does not match declared type. Detected: %s", detectedMime))
				return
			}
		}
	}

	filename := buildFilename(folder, ext)
	url, err := storage.GlobalStorage.Upload(c.Request.Context(), filename, f, file.Size, contentType)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to upload file to storage: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{
		"fileUrl":  url,
		"fileKey":  filename,
		"fileName": file.Filename,
		"fileSize": file.Size,
		"mimeType": contentType,
	})
}

// ─── Presign Upload ─────────────────────────────────────────────────────────────

// PresignUpload generates a pre-signed URL for direct browser-to-S3 upload
func PresignUpload(c *gin.Context) {
	var req presignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "fileName is required")
		return
	}

	category := req.Category
	if category == "" {
		category = "any"
	}
	uploadCtx := req.Context
	if uploadCtx == "" {
		uploadCtx = "general"
	}

	ext := strings.ToLower(filepath.Ext(req.FileName))
	if !isExtAllowed(ext, category) {
		api_response.Error(c, http.StatusBadRequest, fmt.Sprintf("File extension '%s' is not allowed for category '%s'", ext, category))
		return
	}

	if req.ContentType == "" {
		req.ContentType = defaultMimeForExt(ext)
	}

	// File size guard (optional, 0 means not checked)
	if req.FileSize > 0 && req.FileSize > maxSimpleUploadSize {
		api_response.Error(c, http.StatusBadRequest, fmt.Sprintf("File too large for presign. Max size is %dMB. Use /upload/chunked.", maxSimpleUploadSize>>20))
		return
	}

	folder := resolveFolder(uploadCtx)
	filename := buildFilename(folder, ext)

	presignedURL, err := storage.GlobalStorage.GeneratePresignedUploadURL(
		c.Request.Context(),
		filename,
		req.ContentType,
		15*time.Minute,
	)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to generate upload URL")
		return
	}

	publicURL, _ := storage.GlobalStorage.GetURL(c.Request.Context(), filename)
	api_response.Success(c, gin.H{
		"uploadUrl": presignedURL,
		"fileKey":   filename,
		"publicUrl": publicURL,
		"expiresIn": 900, // 15 minutes in seconds
	})
}

// ─── Delete Uploaded File ────────────────────────────────────────────────────────

// DeleteUpload deletes a previously uploaded file from storage
func DeleteUpload(c *gin.Context) {
	var req struct {
		FileKey string `json:"fileKey" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "fileKey is required")
		return
	}

	// Security: only allow deletion of files in expected folders
	allowedPrefixes := []string{"avatars/", "covers/", "books/", "lessons/", "exams/", "attachments/", "announcements/", "blog/", "uploads/", "temp/"}
	allowed := false
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(req.FileKey, prefix) {
			allowed = true
			break
		}
	}
	if !allowed {
		api_response.Error(c, http.StatusForbidden, "File key is not within an allowed storage path")
		return
	}

	if err := storage.GlobalStorage.Delete(c.Request.Context(), req.FileKey); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete file: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"deleted": req.FileKey})
}

// ─── Chunked Upload ─────────────────────────────────────────────────────────────

// UploadChunked handles multi-part chunked upload sessions
// POST   /upload/chunked  → initiate session, returns uploadId
// PUT    /upload/chunked  → upload a single chunk
// PATCH  /upload/chunked  → merge all chunks into final file
func UploadChunked(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodPost:
		handleChunkedInit(c)
	case http.MethodPut:
		handleChunkedPutChunk(c)
	case http.MethodPatch:
		handleChunkedMerge(c)
	default:
		api_response.Error(c, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// POST /upload/chunked → initiate a new chunked upload session
func handleChunkedInit(c *gin.Context) {
	var req chunkedInitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// Validate extension
	ext := strings.ToLower(filepath.Ext(req.FileName))
	category := req.Category
	if category == "" {
		category = "any"
	}
	if !isExtAllowed(ext, category) {
		api_response.Error(c, http.StatusBadRequest, fmt.Sprintf("File extension '%s' is not allowed for category '%s'", ext, category))
		return
	}

	if req.FileType == "" {
		req.FileType = defaultMimeForExt(ext)
	}

	uploadID := uuid.New().String()
	meta := chunkedChunkMetadata{
		FileName:  req.FileName,
		FileType:  req.FileType,
		FileSize:  req.FileSize,
		ChunkSize: req.ChunkSize,
		Context:   req.Context,
	}
	metaBytes, _ := json.Marshal(meta)

	_, err := storage.GlobalStorage.Upload(c.Request.Context(), fmt.Sprintf("temp/%s/meta.json", uploadID), bytes.NewReader(metaBytes), int64(len(metaBytes)), "application/json")
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to initialize upload session")
		return
	}

	api_response.Success(c, gin.H{
		"uploadId":  uploadID,
		"expiresIn": 86400, // 24 hours in seconds
	})
}

// PUT /upload/chunked → upload a single chunk (multipart form)
func handleChunkedPutChunk(c *gin.Context) {
	uploadID := c.PostForm("uploadId")
	chunkIndexStr := c.PostForm("chunkIndex")

	if uploadID == "" || chunkIndexStr == "" {
		api_response.Error(c, http.StatusBadRequest, "uploadId and chunkIndex are required")
		return
	}

	// Verify session exists
	_, err := getChunkMetadata(c.Request.Context(), uploadID)
	if err != nil {
		api_response.Error(c, http.StatusNotFound, "Upload session not found or expired")
		return
	}

	file, err := c.FormFile("chunk")
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "No chunk file in request (field: 'chunk')")
		return
	}

	if file.Size > maxChunkSize {
		api_response.Error(c, http.StatusBadRequest, fmt.Sprintf("Chunk too large. Max chunk size is %dMB", maxChunkSize>>20))
		return
	}

	f, err := file.Open()
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to open chunk")
		return
	}
	defer f.Close()

	chunkPath := fmt.Sprintf("temp/%s/%s", uploadID, chunkIndexStr)
	_, err = storage.GlobalStorage.Upload(c.Request.Context(), chunkPath, f, file.Size, "application/octet-stream")
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to save chunk: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{
		"chunkIndex": chunkIndexStr,
		"uploadId":   uploadID,
		"saved":      true,
	})
}

// PATCH /upload/chunked → merge all chunks into final file
func handleChunkedMerge(c *gin.Context) {
	var req chunkedMergeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "uploadId is required")
		return
	}

	ctx := c.Request.Context()
	meta, err := getChunkMetadata(ctx, req.UploadID)
	if err != nil {
		api_response.Error(c, http.StatusNotFound, "Upload session not found or expired")
		return
	}

	chunks, err := getSortedChunkEntries(ctx, req.UploadID)
	if err != nil || len(chunks) == 0 {
		api_response.Error(c, http.StatusBadRequest, "No chunks found for this upload session")
		return
	}

	folder := resolveFolder(meta.Context)
	ext := strings.ToLower(filepath.Ext(meta.FileName))
	finalFilename := buildFilename(folder, ext)

	url, err := assembleChunkedFile(ctx, finalFilename, meta, chunks)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to assemble file: "+err.Error())
		return
	}

	// Cleanup in background
	go cleanupChunkSession(req.UploadID, chunks)

	api_response.Success(c, gin.H{
		"fileUrl":  url,
		"fileKey":  finalFilename,
		"fileName": meta.FileName,
		"fileSize": meta.FileSize,
		"mimeType": meta.FileType,
	})
}

// ─── Upload Status ──────────────────────────────────────────────────────────────

// GetUploadStatus returns the current status of a chunked upload session
func GetUploadStatus(c *gin.Context) {
	uploadID := c.Param("uploadId")
	if uploadID == "" {
		api_response.Error(c, http.StatusBadRequest, "uploadId is required")
		return
	}

	ctx := c.Request.Context()
	meta, err := getChunkMetadata(ctx, uploadID)
	if err != nil {
		api_response.Error(c, http.StatusNotFound, "Upload session not found or expired")
		return
	}

	chunks, _ := getSortedChunkEntries(ctx, uploadID)

	api_response.Success(c, gin.H{
		"uploadId":       uploadID,
		"fileName":       meta.FileName,
		"fileSize":       meta.FileSize,
		"fileType":       meta.FileType,
		"uploadedChunks": len(chunks),
		"status":         "in_progress",
	})
}

// ─── Helpers ─────────────────────────────────────────────────────────────────────

func resolveFolder(ctx string) string {
	if folder, ok := uploadFolders[ctx]; ok {
		return folder
	}
	return "uploads"
}

func buildFilename(folder, ext string) string {
	return fmt.Sprintf("%s/%s%s", folder, uuid.New().String(), ext)
}

func isExtAllowed(ext, category string) bool {
	if category == "" {
		category = "any"
	}
	cat, ok := allowedUploadCategories[category]
	if !ok {
		// Unknown category → fall back to "any"
		cat = allowedUploadCategories["any"]
	}
	for _, allowed := range cat.exts {
		if allowed == ext {
			return true
		}
	}
	// "any" category is the superset — also check there
	if category != "any" {
		anyCat := allowedUploadCategories["any"]
		for _, allowed := range anyCat.exts {
			if allowed == ext {
				return true
			}
		}
	}
	return false
}

func getMimesForCategory(category string) []string {
	if cat, ok := allowedUploadCategories[category]; ok {
		return cat.mimes
	}
	return nil
}

func getChunkMetadata(ctx context.Context, uploadID string) (chunkedChunkMetadata, error) {
	var meta chunkedChunkMetadata
	rc, err := storage.GlobalStorage.Download(ctx, fmt.Sprintf("temp/%s/meta.json", uploadID))
	if err != nil {
		return meta, err
	}
	defer rc.Close()
	val, err := io.ReadAll(rc)
	if err != nil {
		return meta, err
	}
	json.Unmarshal(val, &meta)
	return meta, nil
}

func getSortedChunkEntries(ctx context.Context, uploadID string) ([]chunkedChunkEntry, error) {
	prefix := fmt.Sprintf("temp/%s/", uploadID)
	files, err := storage.GlobalStorage.List(ctx, prefix)
	if err != nil {
		return nil, err
	}

	var entries []chunkedChunkEntry
	for _, path := range files {
		parts := strings.Split(path, "/")
		if len(parts) < 3 {
			continue
		}
		idx, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			continue
		}
		entries = append(entries, chunkedChunkEntry{index: idx, path: path})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].index < entries[j].index
	})
	return entries, nil
}

// streamReader reads multiple chunk files sequentially as one stream
type streamReader struct {
	chunks        []chunkedChunkEntry
	currentIdx    int
	currentReader io.ReadCloser
	downloadFn    func(path string) (io.ReadCloser, error)
}

func (r *streamReader) Read(p []byte) (int, error) {
	for {
		if r.currentReader == nil {
			if r.currentIdx >= len(r.chunks) {
				return 0, io.EOF
			}
			rc, err := r.downloadFn(r.chunks[r.currentIdx].path)
			if err != nil {
				return 0, fmt.Errorf("failed to open chunk %d: %w", r.currentIdx, err)
			}
			r.currentReader = rc
		}
		n, err := r.currentReader.Read(p)
		if err == io.EOF {
			_ = r.currentReader.Close()
			r.currentReader = nil
			r.currentIdx++
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (r *streamReader) Close() error {
	if r.currentReader != nil {
		err := r.currentReader.Close()
		r.currentReader = nil
		return err
	}
	return nil
}

func assembleChunkedFile(ctx context.Context, finalFilename string, meta chunkedChunkMetadata, chunks []chunkedChunkEntry) (string, error) {
	stream := &streamReader{
		chunks: chunks,
		downloadFn: func(path string) (io.ReadCloser, error) {
			return storage.GlobalStorage.Download(ctx, path)
		},
	}
	defer stream.Close()

	url, err := storage.GlobalStorage.Upload(ctx, finalFilename, stream, meta.FileSize, meta.FileType)
	if err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}
	return url, nil
}

func cleanupChunkSession(uploadID string, chunks []chunkedChunkEntry) {
	bgCtx := context.Background()
	for _, chunk := range chunks {
		_ = storage.GlobalStorage.Delete(bgCtx, chunk.path)
	}
	_ = storage.GlobalStorage.Delete(bgCtx, fmt.Sprintf("temp/%s/meta.json", uploadID))
}
