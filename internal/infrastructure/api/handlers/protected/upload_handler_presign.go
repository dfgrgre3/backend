package protected

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/storage"

	"github.com/gin-gonic/gin"
)

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
