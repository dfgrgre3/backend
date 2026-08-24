package protected

import (
	"strings"
)

// File-upload endpoints (simple multipart, presigned URL, chunked sessions)
// and their shared configuration.
//
// Handlers are split across several files in this package (all sharing
// package protected), grouped by area: this file (constants, allowed-type
// tables, request/response types), upload_handler_simple.go (Upload),
// upload_handler_presign.go (PresignUpload), upload_handler_delete.go
// (DeleteUpload), upload_handler_chunked.go (chunked-session endpoints),
// upload_handler_chunked_assembly.go (chunk metadata/streaming/assembly)
// and upload_handler_helpers.go (shared validation helpers).

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
	Category  string `json:"category"`
}

type chunkedChunkEntry struct {
	index int
	path  string
}
