package course

import (
	"context"

	"thanawy-backend/internal/domain/course"
)

// GetEnrollmentQuery represents the query to get enrollment
type GetEnrollmentQuery struct {
	CourseID string
	UserID   string
}

// GetEnrollmentHandler handles the get enrollment query
type GetEnrollmentHandler struct {
	service *course.CourseService
}

// NewGetEnrollmentHandler creates a new get enrollment handler
func NewGetEnrollmentHandler(service *course.CourseService) *GetEnrollmentHandler {
	return &GetEnrollmentHandler{service: service}
}

// Handle executes the get enrollment query
func (h *GetEnrollmentHandler) Handle(ctx context.Context, query GetEnrollmentQuery) (*course.Enrollment, error) {
	return h.service.GetEnrollment(ctx, query.CourseID, query.UserID)
}
