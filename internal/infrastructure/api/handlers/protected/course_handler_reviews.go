package protected

import (
	"net/http"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetReviews retrieves reviews for a course
func (h *CourseRESTHandler) GetReviews(c *gin.Context) {
	id := c.Param("id")

	courseID, err := uuid.Parse(id)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid course ID")
		return
	}

	reviews, err := h.courseService.ListReviews(courseID, "")
	if err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to get course reviews", err)
		return
	}

	api_response.Success(c, gin.H{"reviews": reviews})
}
