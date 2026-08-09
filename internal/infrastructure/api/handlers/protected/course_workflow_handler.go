package protected

import (
	"net/http"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SubmitForReview submits a course for review
func (h *CourseRESTHandler) SubmitForReview(c *gin.Context) {
	id := c.Param("id")

	courseUUID, _ := uuid.Parse(id)
	err := h.courseService.SubmitForReview(courseUUID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Failed to submit for review: "+err.Error())
		return
	}

	// Log audit
	h.logAudit(c, "STATUS_CHANGE", "course", id, gin.H{"action": "submit_for_review"})

	api_response.Success(c, gin.H{
		"message": "Course submitted for review",
		"status":  string(models.CourseStatusUnderReview),
	})
}

// ApproveCourse approves a course
func (h *CourseRESTHandler) ApproveCourse(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		ReviewerID string `json:"reviewerId" binding:"required"`
		Notes      string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	var err error
	courseUUID, _ := uuid.Parse(id)
	reviewerUUID, _ := uuid.Parse(req.ReviewerID)
	err = h.courseService.ApproveCourse(courseUUID, reviewerUUID, req.Notes)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Failed to approve course: "+err.Error())
		return
	}

	// Log audit
	h.logAudit(c, "STATUS_CHANGE", "course", id, gin.H{
		"action": "approve",
		"notes":  req.Notes,
	})

	api_response.Success(c, gin.H{
		"message": "Course approved",
		"status":  string(models.CourseStatusPublished),
	})
}

// RejectCourse rejects a course
func (h *CourseRESTHandler) RejectCourse(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		ReviewerID string `json:"reviewerId" binding:"required"`
		Reason     string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	courseUUID, _ := uuid.Parse(id)
	reviewerUUID, _ := uuid.Parse(req.ReviewerID)
	err := h.courseService.RejectCourse(courseUUID, reviewerUUID, req.Reason)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Failed to reject course: "+err.Error())
		return
	}

	// Log audit
	h.logAudit(c, "STATUS_CHANGE", "course", id, gin.H{
		"action": "reject",
		"reason": req.Reason,
	})

	api_response.Success(c, gin.H{
		"message": "Course rejected",
		"status":  string(models.CourseStatusDraft),
	})
}

// ArchiveCourse archives a course
func (h *CourseRESTHandler) ArchiveCourse(c *gin.Context) {
	id := c.Param("id")

	courseUUID, _ := uuid.Parse(id)
	userUUID, _ := uuid.Parse("00000000-0000-0000-0000-000000000000")
	err := h.courseService.ArchiveCourse(courseUUID, userUUID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Failed to archive course: "+err.Error())
		return
	}

	// Log audit
	h.logAudit(c, "STATUS_CHANGE", "course", id, gin.H{"action": "archive"})

	api_response.Success(c, gin.H{
		"message": "Course archived",
		"status":  string(models.CourseStatusArchived),
	})
}

// UnarchiveCourse unarchives a course
func (h *CourseRESTHandler) UnarchiveCourse(c *gin.Context) {
	id := c.Param("id")

	courseUUID, _ := uuid.Parse(id)
	err := h.courseService.UnarchiveCourse(courseUUID)

	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Failed to unarchive course: "+err.Error())
		return
	}

	// Log audit
	h.logAudit(c, "STATUS_CHANGE", "course", id, gin.H{"action": "unarchive"})

	api_response.Success(c, gin.H{
		"message": "Course unarchived",
		"status":  string(models.CourseStatusDraft),
	})
}
