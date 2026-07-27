package course

import (
	"context"

	"thanawy-backend/internal/domain/course"
)

// EnrollUserCommand represents the command to enroll a user in a course
type EnrollUserCommand struct {
	CourseID string
	UserID   string
}

// EnrollUserHandler handles the enroll user command
type EnrollUserHandler struct {
	service *course.CourseService
}

// NewEnrollUserHandler creates a new enroll user handler
func NewEnrollUserHandler(service *course.CourseService) *EnrollUserHandler {
	return &EnrollUserHandler{service: service}
}

// Handle executes the enroll user command
func (h *EnrollUserHandler) Handle(ctx context.Context, cmd EnrollUserCommand) (*course.Enrollment, error) {
	return h.service.EnrollUser(ctx, cmd.CourseID, cmd.UserID)
}
