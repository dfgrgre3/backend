package protected

import (
	"net/http"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// =============================================================
// Lesson Exam Link Endpoints (hexagonal Course/Section/Lesson model)
// =============================================================

// LinkExamRequest represents the REST request body for linking an exam to a lesson.
type LinkExamRequest struct {
	ExamID string `json:"examId" binding:"required"`
}

// LinkLessonExam links a lesson to an exam (one exam per lesson; linking a
// new one replaces any existing link).
func (h *CourseRESTHandler) LinkLessonExam(c *gin.Context) {
	lessonID := c.Param("lessonId")
	parsedLessonID, err := uuid.Parse(lessonID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid lesson ID")
		return
	}

	var req LinkExamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if _, err := uuid.Parse(req.ExamID); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid exam ID")
		return
	}

	lesson, err := h.courseService.LinkExam(parsedLessonID, req.ExamID)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to link exam: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"lesson": lesson})
}

// UnlinkLessonExam removes a lesson's exam link.
func (h *CourseRESTHandler) UnlinkLessonExam(c *gin.Context) {
	lessonID := c.Param("lessonId")
	parsedLessonID, err := uuid.Parse(lessonID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid lesson ID")
		return
	}

	if err := h.courseService.UnlinkExam(parsedLessonID); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to unlink exam: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"message": "Exam unlinked successfully"})
}
