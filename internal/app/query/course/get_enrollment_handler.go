package course

import (
	"context"
	"fmt"
)

// GetEnrollmentHandler handles enrollment queries
type GetEnrollmentHandler struct {
	// Add dependencies as needed
}

// NewGetEnrollmentHandler creates a new GetEnrollmentHandler
func NewGetEnrollmentHandler() *GetEnrollmentHandler {
	return &GetEnrollmentHandler{}
}

// Handle handles the get enrollment query
func (h *GetEnrollmentHandler) Handle(ctx context.Context, query GetEnrollmentQuery) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}
