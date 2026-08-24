package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// BulkStatusChange changes the status of multiple courses.
//
// PERFORMANCE: previously issued one GetCourseHandler.Handle call plus one
// UPDATE per course id — an N+1 pattern. Now does a single existence-check
// query (WHERE id IN (...)) followed by a single batched UPDATE covering
// every existing course at once.
func (h *CourseRESTHandler) BulkStatusChange(c *gin.Context) {
	var req struct {
		CourseIDs []string `json:"courseIds" binding:"required,min=1"`
		Status    string   `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	// Validate status
	validStatuses := map[string]bool{
		"draft":          true,
		"pending_review": true,
		"published":      true,
		"archived":       true,
	}
	if !validStatuses[req.Status] {
		api_response.Error(c, http.StatusBadRequest, "Invalid status: "+req.Status)
		return
	}

	updated := []string{}
	failed := []string{}

	// Parse and de-dupe valid UUIDs first; anything unparsable fails fast
	// without touching the database.
	validIDs := make([]string, 0, len(req.CourseIDs))
	for _, courseIDStr := range req.CourseIDs {
		if _, err := uuid.Parse(courseIDStr); err != nil {
			failed = append(failed, courseIDStr)
			continue
		}
		validIDs = append(validIDs, courseIDStr)
	}

	if len(validIDs) > 0 {
		var existingIDs []string
		if err := h.db.Model(&models.LmsCourse{}).Where("id IN ?", validIDs).Pluck("id", &existingIDs).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to look up courses")
			return
		}
		existing := make(map[string]bool, len(existingIDs))
		for _, id := range existingIDs {
			existing[id] = true
		}

		if len(existingIDs) > 0 {
			if err := h.db.Model(&models.LmsCourse{}).Where("id IN ?", existingIDs).Update("status", req.Status).Error; err != nil {
				api_response.Error(c, http.StatusInternalServerError, "Failed to update course status")
				return
			}
		}

		for _, courseIDStr := range validIDs {
			if existing[courseIDStr] {
				updated = append(updated, courseIDStr)
			} else {
				failed = append(failed, courseIDStr)
			}
		}
	}

	api_response.Success(c, gin.H{
		"message": "Bulk status change completed",
		"updated": updated,
		"failed":  failed,
		"total":   len(updated),
	})
}
