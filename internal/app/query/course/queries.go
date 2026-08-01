package course

// GetCourseQuery represents a query to get a course
type GetCourseQuery struct {
	ID   string
	Slug string
}

// ListCoursesQuery represents a query to list courses
type ListCoursesQuery struct {
	Page         int
	PageSize     int
	Limit        int
	CategoryID   *string
	Level        *string
	Language     *string
	InstructorID *string
	Search       *string
	SearchQuery  *string
	SortBy       string
	SortOrder    string
	Status       *string
	IsFeatured   *bool
	IsTrending   *bool
	IsNew        *bool
}

// GetEnrollmentQuery represents a query to get user enrollment
type GetEnrollmentQuery struct {
	UserID   string
	CourseID string
}
