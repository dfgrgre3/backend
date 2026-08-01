package course

import (
	"context"
	"fmt"
)

// UpdateProgressHandler handles progress update commands
type UpdateProgressHandler struct {
	// Add dependencies as needed
}

// NewUpdateProgressHandler creates a new UpdateProgressHandler
func NewUpdateProgressHandler() *UpdateProgressHandler {
	return &UpdateProgressHandler{}
}

// Handle handles the update progress command
func (h *UpdateProgressHandler) Handle(ctx context.Context, cmd UpdateProgressCommand) error {
	return fmt.Errorf("not implemented")
}
