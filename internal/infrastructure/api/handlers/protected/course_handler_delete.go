package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// DeleteCourse deletes a course
func (h *CourseRESTHandler) DeleteCourse(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		var idBody struct {
			ID string `json:"id"`
		}
		if err := c.ShouldBindJSON(&idBody); err == nil && idBody.ID != "" {
			id = idBody.ID
		}
	}
	if id == "" {
		api_response.Error(c, http.StatusBadRequest, "Course ID is required")
		return
	}

	err := h.db.WithContext(c.Request.Context()).Delete(&models.LmsCourse{}, "id = ?", id).Error
	if err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to delete course", err)
		return
	}

	api_response.Success(c, gin.H{"message": "Course deleted successfully"})
}
