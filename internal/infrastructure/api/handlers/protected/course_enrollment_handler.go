package protected

import (
	"net/http"

	command "thanawy-backend/internal/domain/course/service"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// =============================================================
// Enrollment Endpoints
// =============================================================

// EnrollUser enrolls a user in a course
func (h *CourseRESTHandler) EnrollUser(c *gin.Context) {
	var req struct {
		CourseID string `json:"courseId" binding:"required"`
		UserID   string `json:"userId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	cmd := command.EnrollUserCommand{
		CourseID: req.CourseID,
		UserID:   req.UserID,
	}

	enrollment, err := h.enrollUserHandler.Handle(c.Request.Context(), cmd)
	if err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to enroll user", err)
		return
	}

	api_response.Created(c, gin.H{"enrollment": enrollment})
}

// ListEnrollments lists all enrollments for a course
func (h *CourseRESTHandler) ListEnrollments(c *gin.Context) {
	courseID := c.Param("id")
	parsedCourseID, err := uuid.Parse(courseID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid course ID")
		return
	}

	enrollments, err := h.courseService.ListCourseEnrollments(parsedCourseID)
	if err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to list enrollments", err)
		return
	}

	api_response.Success(c, gin.H{"enrollments": enrollments})
}

// GetEnrollment retrieves a single user's enrollment in a course
func (h *CourseRESTHandler) GetEnrollment(c *gin.Context) {
	courseID := c.Param("id")
	userID := c.Param("userId")

	query := command.GetEnrollmentQuery{
		CourseID: courseID,
		UserID:   userID,
	}

	enrollment, err := h.getEnrollmentHandler.Handle(c.Request.Context(), query)
	if err != nil {
		api_response.Error(c, http.StatusNotFound, "Enrollment not found")
		return
	}

	api_response.Success(c, gin.H{"enrollment": enrollment})
}

// UpdateProgress updates enrollment progress
func (h *CourseRESTHandler) UpdateProgress(c *gin.Context) {
	courseID := c.Param("id")
	userID := c.Param("userId")

	var req struct {
		Progress float64 `json:"progress" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	cmd := command.UpdateProgressCommand{
		CourseID: courseID,
		UserID:   userID,
		Progress: req.Progress,
	}

	err := h.updateProgressHandler.Handle(c.Request.Context(), cmd)
	if err != nil {
		api_response.ErrorDetail(c, http.StatusInternalServerError, "Failed to update progress", err)
		return
	}

	// Fetch updated enrollment
	query := command.GetEnrollmentQuery{
		CourseID: courseID,
		UserID:   userID,
	}
	enrollment, _ := h.getEnrollmentHandler.Handle(c.Request.Context(), query)

	api_response.Success(c, gin.H{"enrollment": enrollment})
}
