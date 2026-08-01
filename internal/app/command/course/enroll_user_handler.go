package course

import (
	"context"
	"fmt"
)

// EnrollUserHandler handles user enrollment commands
type EnrollUserHandler struct {
	// Add dependencies as needed
}

// NewEnrollUserHandler creates a new EnrollUserHandler
func NewEnrollUserHandler() *EnrollUserHandler {
	return &EnrollUserHandler{}
}

// Handle handles the enroll user command
func (h *EnrollUserHandler) Handle(ctx context.Context, cmd EnrollUserCommand) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}
