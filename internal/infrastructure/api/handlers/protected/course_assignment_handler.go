package protected

import (
	"net/http"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// =============================================================
// Course Assignment Endpoints (hexagonal Course/Section/Lesson model)
// =============================================================

// CreateAssignmentRequest represents the REST request body for creating a
// course assignment.
type CreateAssignmentRequest struct {
	Title       string  `json:"title" binding:"required"`
	Description *string `json:"description"`
	DueDate     *int64  `json:"dueDate"`
	MaxScore    float64 `json:"maxScore"`
}

// ListCourseAssignments lists every assignment in a course's catalog.
func (h *CourseRESTHandler) ListCourseAssignments(c *gin.Context) {
	courseID := c.Param("id")
	parsedCourseID, err := uuid.Parse(courseID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid course ID")
		return
	}

	assignments, err := h.courseService.ListCourseAssignments(parsedCourseID)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to list assignments: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"assignments": assignments})
}

// CreateCourseAssignment adds a new assignment to a course's catalog (unlinked
// to any lesson until LinkLessonAssignment is called).
func (h *CourseRESTHandler) CreateCourseAssignment(c *gin.Context) {
	courseID := c.Param("id")
	parsedCourseID, err := uuid.Parse(courseID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid course ID")
		return
	}

	var req CreateAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	maxScore := req.MaxScore
	if maxScore <= 0 {
		maxScore = 100
	}

	var dueDate *time.Time
	if req.DueDate != nil {
		t := time.Unix(*req.DueDate, 0)
		dueDate = &t
	}

	assignment, err := h.courseService.CreateAssignment(parsedCourseID, req.Title, req.Description, dueDate, maxScore)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create assignment: "+err.Error())
		return
	}

	api_response.Created(c, gin.H{"assignment": assignment})
}

// DeleteCourseAssignment removes an assignment from a course's catalog.
func (h *CourseRESTHandler) DeleteCourseAssignment(c *gin.Context) {
	assignmentID := c.Param("assignmentId")
	parsedID, err := uuid.Parse(assignmentID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid assignment ID")
		return
	}

	if err := h.courseService.DeleteAssignment(parsedID); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete assignment: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"message": "Assignment deleted successfully"})
}

// LinkAssignmentRequest represents the REST request body for linking an
// assignment to a lesson.
type LinkAssignmentRequest struct {
	LessonID string `json:"lessonId" binding:"required"`
}

// LinkAssignment links an assignment from the course's catalog to a lesson
// (one assignment per lesson; linking a new one replaces any existing link
// on that assignment).
func (h *CourseRESTHandler) LinkAssignment(c *gin.Context) {
	assignmentID := c.Param("assignmentId")
	parsedAssignmentID, err := uuid.Parse(assignmentID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid assignment ID")
		return
	}

	var req LinkAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	lessonID, err := uuid.Parse(req.LessonID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid lesson ID")
		return
	}

	assignment, err := h.courseService.LinkAssignment(parsedAssignmentID, lessonID)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to link assignment: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"assignment": assignment})
}

// UnlinkAssignment removes an assignment's lesson link (the assignment
// itself stays in the course's catalog). Routed by assignment ID, since the
// link is stored on the LmsAssignment row (lesson_id), not on the lesson.
func (h *CourseRESTHandler) UnlinkAssignment(c *gin.Context) {
	assignmentID := c.Param("assignmentId")
	parsedID, err := uuid.Parse(assignmentID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid assignment ID")
		return
	}

	if err := h.courseService.UnlinkAssignment(parsedID); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to unlink assignment: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"message": "Assignment unlinked successfully"})
}
