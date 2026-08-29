package protected

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

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
		api_response.Error(c, http.StatusMethodNotAllowed, msgMethodNotAllowed)
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
		Category:  category,
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
	// Cap the request body before any multipart parsing happens (the first
	// c.PostForm call below already triggers Gin's multipart parse) — see
	// the identical fix/rationale in upload_handler_simple.go's Upload().
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxChunkSize+1<<20)

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
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to save chunk", err)
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

	// Re-validate the extension against the session's declared category.
	// meta comes from a JSON blob written to storage at init time; treat it
	// as untrusted input, same as any client-supplied value.
	category := meta.Category
	if category == "" {
		category = "any"
	}
	ext := strings.ToLower(filepath.Ext(meta.FileName))
	if !isExtAllowed(ext, category) {
		api_response.Error(c, http.StatusBadRequest, fmt.Sprintf("File extension '%s' is not allowed for category '%s'", ext, category))
		return
	}

	folder := resolveFolder(meta.Context)
	finalFilename := buildFilename(folder, ext)

	url, err := assembleChunkedFile(ctx, finalFilename, meta, chunks, category, ext)
	if err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to assemble file", err)
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
