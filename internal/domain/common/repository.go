package models

import (
	"context"
)

// DomainRepository defines the interface for course data persistence
type DomainRepository interface {
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

	// DomainEnrollment operations
	CreateEnrollment(ctx context.Context, enrollment *DomainEnrollment) error
	GetEnrollment(ctx context.Context, courseID, userID string) (*DomainEnrollment, error)
	UpdateEnrollmentProgress(ctx context.Context, enrollment *DomainEnrollment) error
	ListEnrollments(ctx context.Context, filter EnrollmentFilter) ([]*DomainEnrollment, int, error)

	// Pricing operations
	CreatePricing(ctx context.Context, pricing *Pricing) error
	GetPricing(ctx context.Context, courseID string) (*Pricing, error)
	UpdatePricing(ctx context.Context, pricing *Pricing) error
	DeletePricing(ctx context.Context, courseID string) error

	// DomainCategory operations
	AddCourseCategory(ctx context.Context, courseID, categoryID string) error
	RemoveCourseCategory(ctx context.Context, courseID, categoryID string) error
	ListCourseCategories(ctx context.Context, courseID string) ([]*DomainCategory, error)

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

	// Course versioning & cloning
	CloneCourse(ctx context.Context, courseID string, newTitle string) (*Course, error)
	CreateVersion(ctx context.Context, courseID string, userID string) (*DomainCourseVersion, error)
	ListVersions(ctx context.Context, courseID string) ([]*DomainCourseVersion, error)
	RestoreVersion(ctx context.Context, courseID string, versionNumber int, userID string) (*Course, error)
	GetChangelog(ctx context.Context, courseID string) ([]*CourseChangelog, error)
}

// CourseFilter defines filters for listing courses
type CourseFilter struct {
	Status       *CourseStatus
	Level        *CourseLevel
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

// EnrollmentFilter defines filters for listing enrollments
type EnrollmentFilter struct {
	CourseID *string
	UserID   *string
	Status   *string
	Page     int
	Limit    int
}
