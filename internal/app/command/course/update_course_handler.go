package course

import (
	"context"
	"fmt"
)

// UpdateCourseHandler handles course update commands
type UpdateCourseHandler struct {
	// Add dependencies as needed
}

// NewUpdateCourseHandler creates a new UpdateCourseHandler
func NewUpdateCourseHandler() *UpdateCourseHandler {
	return &UpdateCourseHandler{}
}

// Handle handles the update course command
func (h *UpdateCourseHandler) Handle(ctx context.Context, cmd UpdateCourseCommand) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}
