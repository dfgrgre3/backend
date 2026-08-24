package protected

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"
	"thanawy-backend/internal/infrastructure/storage"

	"github.com/gin-gonic/gin"
)

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
