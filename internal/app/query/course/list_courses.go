package course

import (
	"context"

	"thanawy-backend/internal/domain/course"
)

// ListCoursesQuery represents the query to list courses
type ListCoursesQuery struct {
	Status       *course.CourseStatus
	Level        *course.CourseLevel
	Language     *string
	CategoryID   *string
	InstructorID *string
	IsFeatured   *bool
	IsTrending   *bool
	IsNew        *bool
	SearchQuery  *string
	Page         int
	Limit        int
}

// ListCoursesHandler handles the list courses query
type ListCoursesHandler struct {
	service *course.CourseService
}

// NewListCoursesHandler creates a new list courses handler
func NewListCoursesHandler(service *course.CourseService) *ListCoursesHandler {
	return &ListCoursesHandler{service: service}
}

// Handle executes the list courses query
func (h *ListCoursesHandler) Handle(ctx context.Context, query ListCoursesQuery) ([]*course.Course, int, error) {
	filter := course.CourseFilter{
		Status:       query.Status,
		Level:        query.Level,
		Language:     query.Language,
		CategoryID:   query.CategoryID,
		InstructorID:  query.InstructorID,
		IsFeatured:   query.IsFeatured,
		IsTrending:   query.IsTrending,
		IsNew:        query.IsNew,
		SearchQuery:  query.SearchQuery,
		Page:         query.Page,
		Limit:        query.Limit,
	}
	
	return h.service.ListCourses(ctx, filter)
}
