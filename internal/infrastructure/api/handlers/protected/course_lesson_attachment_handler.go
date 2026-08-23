package protected

import (
	"net/http"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// =============================================================
// Lesson Attachment Endpoints (hexagonal Course/Section/Lesson model)
// =============================================================

// AddAttachmentRequest represents the REST request body for adding a lesson attachment.
// The raw file itself is uploaded separately via the generic /api/admin/upload
// endpoint first; this only records the resulting metadata.
type AddAttachmentRequest struct {
	Title    string `json:"title" binding:"required"`
	FileURL  string `json:"fileUrl" binding:"required"`
	FileType string `json:"fileType"`
	FileSize *int64 `json:"fileSize"`
}

// AddLessonAttachment adds an attachment to a lesson.
func (h *CourseRESTHandler) AddLessonAttachment(c *gin.Context) {
	lessonID := c.Param("lessonId")

	var req AddAttachmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	parsedLessonID, err := uuid.Parse(lessonID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid lesson ID")
		return
	}

	attachment, err := h.courseService.AddAttachment(parsedLessonID, req.Title, req.FileURL, req.FileType, req.FileSize)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to add attachment: "+err.Error())
		return
	}

	api_response.Created(c, gin.H{"attachment": attachment})
}

// DeleteLessonAttachment deletes a lesson attachment.
func (h *CourseRESTHandler) DeleteLessonAttachment(c *gin.Context) {
	attachmentID := c.Param("attachmentId")

	parsedID, err := uuid.Parse(attachmentID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid attachment ID")
		return
	}

	if err := h.courseService.DeleteAttachment(parsedID); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete attachment: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"message": "Attachment deleted successfully"})
}
