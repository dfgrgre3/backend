package courseservice

// GetCourseQuery represents a query to get a course
type GetCourseQuery struct {
	ID   string
	Slug string
}

// ListCoursesQuery represents a query to list courses
type ListCoursesQuery struct {
	Page            int
	PageSize        int
	Limit           int
	CategoryID      *string
	Level           *string
	Language        *string
	InstructorID    *string
	Search          *string
	SearchQuery     *string
	SortBy          string
	SortOrder       string
	Status          *string
	IsFeatured      *bool
	IsTrending      *bool
	IsNew           *bool
	Tags            []string
	PriceType       *string
	MinPrice        *float64
	MaxPrice        *float64
	HasCertificate  *bool
	PublishedAfter  *string
	PublishedBefore *string
}

// SearchCoursesQuery represents an advanced search query for courses
type SearchCoursesQuery struct {
	ListCoursesQuery
	Query           string   `json:"query"`           // Full-text search query
	CategoryIDs     []string `json:"categoryIds"`     // Multiple categories
	InstructorIDs   []string `json:"instructorIds"`   // Multiple instructors
	ExcludeCourseID *string  `json:"excludeCourseId"` // Exclude a specific course
	SortBy          string   `json:"sortBy"`          // Sort field: title, created_at, rating, price, popularity
	SortOrder       string   `json:"sortOrder"`       // asc or desc
	Page            int      `json:"page"`
	Limit           int      `json:"limit"`
}

// GetEnrollmentQuery represents a query to get user enrollment
type GetEnrollmentQuery struct {
	UserID   string
	CourseID string
}
