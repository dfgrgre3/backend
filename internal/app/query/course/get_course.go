package course

import (
	"context"

	"thanawy-backend/internal/domain/course"
)

// GetCourseQuery represents the query to get a course
type GetCourseQuery struct {
	ID   string
	Slug string
}

// GetCourseHandler handles the get course query
type GetCourseHandler struct {
	service *course.CourseService
}

// NewGetCourseHandler creates a new get course handler
func NewGetCourseHandler(service *course.CourseService) *GetCourseHandler {
	return &GetCourseHandler{service: service}
}

// Handle executes the get course query
func (h *GetCourseHandler) Handle(ctx context.Context, query GetCourseQuery) (*course.Course, error) {
	if query.ID != "" {
		return h.service.GetCourse(ctx, query.ID)
	}
	if query.Slug != "" {
		// Would need a GetBySlug method in service
		// For now, return not found
		return nil, course.ErrCourseNotFound
	}
	return nil, course.ErrCourseNotFound
}
