package protected

import (
	"net/http"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// =============================================================
// Course Instructor Endpoints (multi-teacher assignment)
// =============================================================

// AddInstructorRequest represents the REST request body for assigning an
// instructor to a course.
type AddInstructorRequest struct {
	InstructorID string `json:"instructorId" binding:"required"`
	Role         string `json:"role"`
}

// ListCourseInstructors lists every instructor assigned to a course.
func (h *CourseRESTHandler) ListCourseInstructors(c *gin.Context) {
	courseID := c.Param("id")
	parsedCourseID, err := uuid.Parse(courseID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid course ID")
		return
	}

	instructors, err := h.courseService.ListInstructors(parsedCourseID)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to list instructors: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"instructors": instructors})
}

// AddCourseInstructor assigns an instructor to a course with a role.
func (h *CourseRESTHandler) AddCourseInstructor(c *gin.Context) {
	courseID := c.Param("id")
	parsedCourseID, err := uuid.Parse(courseID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid course ID")
		return
	}

	var req AddInstructorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	instructorID, err := uuid.Parse(req.InstructorID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid instructor ID")
		return
	}

	role := req.Role
	if role == "" {
		role = "INSTRUCTOR"
	}

	if err := h.courseService.AddInstructor(parsedCourseID, instructorID, role, nil); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to add instructor: "+err.Error())
		return
	}

	instructors, err := h.courseService.ListInstructors(parsedCourseID)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to list instructors: "+err.Error())
		return
	}

	api_response.Created(c, gin.H{"instructors": instructors})
}

// RemoveCourseInstructor removes an instructor from a course.
func (h *CourseRESTHandler) RemoveCourseInstructor(c *gin.Context) {
	courseID := c.Param("id")
	parsedCourseID, err := uuid.Parse(courseID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid course ID")
		return
	}

	instructorID := c.Param("instructorId")
	parsedInstructorID, err := uuid.Parse(instructorID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid instructor ID")
		return
	}

	if err := h.courseService.RemoveInstructor(parsedCourseID, parsedInstructorID); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to remove instructor: "+err.Error())
		return
	}

	api_response.Success(c, gin.H{"message": "Instructor removed successfully"})
}
