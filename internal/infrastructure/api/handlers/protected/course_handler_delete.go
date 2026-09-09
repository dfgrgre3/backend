package protected

import (
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DeleteCourse deletes a course
func (h *CourseRESTHandler) DeleteCourse(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		var idBody struct {
			ID string `json:"id"`
		}
		if err := c.ShouldBindJSON(&idBody); err == nil && idBody.ID != "" {
			id = strings.TrimSpace(idBody.ID)
		}
	}
	if id == "" {
		api_response.Error(c, http.StatusBadRequest, "Course ID is required")
		return
	}

	// Validate UUID format before querying the database
	courseUUID, err := uuid.Parse(id)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid course ID format")
		return
	}

	result := h.db.WithContext(c.Request.Context()).Delete(&models.LmsCourse{}, "id = ?", courseUUID)
	if result.Error != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to delete course", result.Error)
		return
	}
	if result.RowsAffected == 0 {
		// The admin catalog still contains legacy Subject records in some
		// installations. Keep the compatibility DELETE endpoint useful while
		// those records are being migrated.
		var legacy models.Subject
		legacyResult := h.db.WithContext(c.Request.Context()).First(&legacy, "id = ?", id)
		if legacyResult.Error == nil {
			DeleteSubject(c)
			return
		}

		// DELETE is intentionally idempotent: a slow UI can retry after the
		// first request already committed. Treat an already-absent course as a
		// successful delete instead of surfacing a misleading 404.
		if legacyResult.Error == gorm.ErrRecordNotFound {
			api_response.Success(c, gin.H{"message": "Course already deleted", "success": true})
			return
		}
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to verify course", legacyResult.Error)
		return
	}

	api_response.Success(c, gin.H{"message": "Course deleted successfully", "success": true})
}
