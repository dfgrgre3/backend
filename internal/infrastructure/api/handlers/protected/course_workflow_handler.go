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
		Notes string `json:"notes"`
	}
	_ = c.ShouldBindJSON(&req)

	reviewerID, ok := currentUserID(c)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	courseUUID, _ := uuid.Parse(id)
	reviewerUUID, _ := uuid.Parse(reviewerID)
	err := h.courseService.ApproveCourse(courseUUID, reviewerUUID, req.Notes)
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
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	reviewerID, ok := currentUserID(c)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	courseUUID, _ := uuid.Parse(id)
	reviewerUUID, _ := uuid.Parse(reviewerID)
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

	userID, _ := currentUserID(c)
	courseUUID, _ := uuid.Parse(id)
	userUUID, _ := uuid.Parse(userID)
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

// ListCourseReviewComments returns the workflow review comments of a course
func (h *CourseRESTHandler) ListCourseReviewComments(c *gin.Context) {
	courseUUID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid course id")
		return
	}

	comments, err := h.courseService.ListReviewComments(courseUUID)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to list review comments: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"comments": comments})
}

// AddCourseReviewComment adds a workflow review comment to a course
func (h *CourseRESTHandler) AddCourseReviewComment(c *gin.Context) {
	courseUUID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid course id")
		return
	}

	var req struct {
		Comment string `json:"comment" binding:"required"`
		Status  string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}
	if req.Status == "" {
		req.Status = "pending"
	}

	reviewerID, ok := currentUserID(c)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	reviewerUUID, _ := uuid.Parse(reviewerID)

	comment, err := h.courseService.AddReviewComment(courseUUID, reviewerUUID, req.Comment, req.Status)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to add review comment: "+err.Error())
		return
	}

	api_response.Created(c, gin.H{"comment": comment})
}
