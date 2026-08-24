package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	apiresponse "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

func BulkDeleteInstructors(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	var input struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		apiresponse.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(input.IDs) == 0 {
		apiresponse.Error(c, http.StatusBadRequest, "IDs are required")
		return
	}
	if len(input.IDs) > maxBulkDeleteIDs {
		apiresponse.Error(c, http.StatusBadRequest, "Too many IDs")
		return
	}

	result := database.Where("id IN ? AND role = ?", input.IDs, models.RoleTeacher).Delete(&models.User{})
	if result.Error != nil {
		apiresponse.Error(c, http.StatusInternalServerError, "Failed to delete instructors")
		return
	}

	apiresponse.Success(c, gin.H{
		"deleted":      result.RowsAffected > 0,
		"deletedCount": result.RowsAffected,
	})
}
