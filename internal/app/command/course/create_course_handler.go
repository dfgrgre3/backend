package course

import (
	"context"
	"fmt"
)

// CreateCourseHandler handles course creation commands
type CreateCourseHandler struct {
	// Add dependencies as needed
}

// NewCreateCourseHandler creates a new CreateCourseHandler
func NewCreateCourseHandler() *CreateCourseHandler {
	return &CreateCourseHandler{}
}

// Handle handles the create course command
func (h *CreateCourseHandler) Handle(ctx context.Context, cmd CreateCourseCommand) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}
