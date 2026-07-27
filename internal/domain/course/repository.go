package course

import (
	"context"
)

// Repository defines the interface for course data persistence
type Repository interface {
	// Course operations
	CreateCourse(ctx context.Context, course *Course) error
	GetCourseByID(ctx context.Context, id string) (*Course, error)
	GetCourseBySlug(ctx context.Context, slug string) (*Course, error)
	UpdateCourse(ctx context.Context, course *Course) error
	DeleteCourse(ctx context.Context, id string) error
	ListCourses(ctx context.Context, filter CourseFilter) ([]*Course, int, error)
	
	// Section operations
	CreateSection(ctx context.Context, section *Section) error
	GetSectionByID(ctx context.Context, id string) (*Section, error)
	UpdateSection(ctx context.Context, section *Section) error
	DeleteSection(ctx context.Context, id string) error
	ListSections(ctx context.Context, courseID string) ([]*Section, error)
	ReorderSections(ctx context.Context, courseID string, sectionIDs []string) error
	
	// Lesson operations
	CreateLesson(ctx context.Context, lesson *Lesson) error
	GetLessonByID(ctx context.Context, id string) (*Lesson, error)
	UpdateLesson(ctx context.Context, lesson *Lesson) error
	DeleteLesson(ctx context.Context, id string) error
	ListLessons(ctx context.Context, sectionID string) ([]*Lesson, error)
	ReorderLessons(ctx context.Context, sectionID string, lessonIDs []string) error
	
	// Enrollment operations
	CreateEnrollment(ctx context.Context, enrollment *Enrollment) error
	GetEnrollment(ctx context.Context, courseID, userID string) (*Enrollment, error)
	UpdateEnrollmentProgress(ctx context.Context, enrollment *Enrollment) error
	ListEnrollments(ctx context.Context, filter EnrollmentFilter) ([]*Enrollment, int, error)
	
	// Pricing operations
	CreatePricing(ctx context.Context, pricing *Pricing) error
	GetPricing(ctx context.Context, courseID string) (*Pricing, error)
	UpdatePricing(ctx context.Context, pricing *Pricing) error
	DeletePricing(ctx context.Context, courseID string) error
	
	// Category operations
	AddCourseCategory(ctx context.Context, courseID, categoryID string) error
	RemoveCourseCategory(ctx context.Context, courseID, categoryID string) error
	ListCourseCategories(ctx context.Context, courseID string) ([]*Category, error)
	
	// Instructor operations
	AddCourseInstructor(ctx context.Context, courseID, instructorID string, role string) error
	RemoveCourseInstructor(ctx context.Context, courseID, instructorID string) error
	ListCourseInstructors(ctx context.Context, courseID string) ([]*Instructor, error)
	
	// Review operations
	CreateReview(ctx context.Context, review *Review) error
	GetReview(ctx context.Context, courseID, userID string) (*Review, error)
	UpdateReview(ctx context.Context, review *Review) error
	ListReviews(ctx context.Context, courseID string) ([]*Review, error)
	
	// Certificate operations
	CreateCertificate(ctx context.Context, certificate *Certificate) error
	GetCertificate(ctx context.Context, courseID, userID string) (*Certificate, error)
	ListCertificates(ctx context.Context, userID string) ([]*Certificate, error)
}

// CourseFilter defines filters for listing courses
type CourseFilter struct {
	Status      *CourseStatus
	Level       *CourseLevel
	Language    *string
	CategoryID  *string
	InstructorID *string
	IsFeatured  *bool
	IsTrending  *bool
	IsNew       *bool
	SearchQuery *string
	Page        int
	Limit       int
}

// EnrollmentFilter defines filters for listing enrollments
type EnrollmentFilter struct {
	CourseID *string
	UserID   *string
	Status   *string
	Page     int
	Limit    int
}
