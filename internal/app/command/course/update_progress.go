package course

import (
	"context"

	"thanawy-backend/internal/domain/course"
)

// UpdateProgressCommand represents the command to update enrollment progress
type UpdateProgressCommand struct {
	CourseID string
	UserID   string
	Progress float64
}

// UpdateProgressHandler handles the update progress command
type UpdateProgressHandler struct {
	service *course.CourseService
}

// NewUpdateProgressHandler creates a new update progress handler
func NewUpdateProgressHandler(service *course.CourseService) *UpdateProgressHandler {
	return &UpdateProgressHandler{service: service}
}

// Handle executes the update progress command
func (h *UpdateProgressHandler) Handle(ctx context.Context, cmd UpdateProgressCommand) error {
	enrollment, err := h.service.GetEnrollment(ctx, cmd.CourseID, cmd.UserID)
	if err != nil {
		return err
	}
	
	enrollment.Progress = cmd.Progress
	
	// Auto-complete if progress is 100%
	if cmd.Progress >= 100 {
		return h.service.CompleteCourse(ctx, cmd.CourseID, cmd.UserID)
	}
	
	return h.service.UpdateProgress(ctx, enrollment)
}
