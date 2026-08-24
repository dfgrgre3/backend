package protected

import (
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// TeachingDeleteCourse deletes a course.
func TeachingDeleteCourse(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	courseID := strings.TrimSpace(c.Param("id"))
	if courseID == "" {
		api_response.Error(c, http.StatusBadRequest, "Course ID is required")
		return
	}

	result := database.Where("id = ? AND instructor_id = ?", courseID, userID).Delete(&models.Subject{})
	if result.Error != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete course")
		return
	}
	if result.RowsAffected == 0 {
		api_response.Error(c, http.StatusNotFound, "Course not found or access denied")
		return
	}

	api_response.Success(c, gin.H{"deleted": true})
}
