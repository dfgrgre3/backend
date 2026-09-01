package protected

import (
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
		api_response.Error(c, http.StatusNotFound, "Course not found")
		return
	}

	api_response.Success(c, gin.H{"message": "Course deleted successfully", "success": true})
}
