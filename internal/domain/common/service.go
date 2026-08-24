package models

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// parseUUID is a helper to convert string to uuid.UUID with error handling
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

var (
	ErrCourseNotFound      = errors.New("course not found")
	ErrSectionNotFound     = errors.New("section not found")
	ErrLessonNotFound      = errors.New("lesson not found")
	ErrEnrollmentNotFound  = errors.New("enrollment not found")
	ErrInvalidStatus       = errors.New("invalid status transition")
	ErrDuplicateEnrollment = errors.New("user already enrolled")
)

// Service defines the business logic interface for courses.
//
// CourseService (its implementation) has its methods split across several
// files in this package (all sharing package models and the CourseService
// receiver), grouped by area: course_service.go (construction, course
// CRUD/workflow), course_service_sections_lessons.go,
// course_service_enrollment.go, course_service_pricing_taxonomy.go
// (pricing/category/instructor), course_service_review_certificate.go and
// course_service_versioning.go (versioning/cloning + canTransition).
type Service interface {
	// Course lifecycle
	CreateCourse(ctx context.Context, course *Course) (*Course, error)
	GetCourse(ctx context.Context, id string) (*Course, error)
	UpdateCourse(ctx context.Context, course *Course) (*Course, error)
	DeleteCourse(ctx context.Context, id string) error
	ListCourses(ctx context.Context, filter CourseFilter) ([]*Course, int, error)

	// Course workflow
	SubmitForReview(ctx context.Context, courseID string) error
	ApproveCourse(ctx context.Context, courseID string, reviewerID string, notes string) error
	RejectCourse(ctx context.Context, courseID string, reviewerID string, reason string) error
	ArchiveCourse(ctx context.Context, courseID string) error
	UnarchiveCourse(ctx context.Context, courseID string) error

	// Section management
	CreateSection(ctx context.Context, courseID string, section *Section) (*Section, error)
	UpdateSection(ctx context.Context, section *Section) (*Section, error)
	DeleteSection(ctx context.Context, sectionID string) error
	ListSections(ctx context.Context, courseID uuid.UUID) ([]*Section, error)
	ReorderSections(ctx context.Context, courseID string, sectionIDs []string) error

	// Lesson management
	CreateLesson(ctx context.Context, sectionID string, lesson *Lesson) (*Lesson, error)
	UpdateLesson(ctx context.Context, lesson *Lesson) (*Lesson, error)
	DeleteLesson(ctx context.Context, lessonID string) error
	ListLessons(ctx context.Context, sectionID uuid.UUID) ([]*Lesson, error)
	ReorderLessons(ctx context.Context, sectionID string, lessonIDs []string) error

	// DomainEnrollment management
	EnrollUser(ctx context.Context, courseID, userID string) (*DomainEnrollment, error)
	GetEnrollment(ctx context.Context, courseID, userID string) (*DomainEnrollment, error)
	UpdateProgress(ctx context.Context, enrollment *DomainEnrollment) error
	CompleteCourse(ctx context.Context, courseID, userID string) error
	ListEnrollments(ctx context.Context, filter EnrollmentFilter) ([]*DomainEnrollment, int, error)

	// Pricing management
	SetPricing(ctx context.Context, courseID string, pricing *Pricing) (*Pricing, error)
	GetPricing(ctx context.Context, courseID string) (*Pricing, error)

	// DomainCategory management
	AddCategory(ctx context.Context, courseID, categoryID string) error
	RemoveCategory(ctx context.Context, courseID, categoryID string) error

	// Instructor management
	AddInstructor(ctx context.Context, courseID, instructorID string, role string) error
	RemoveInstructor(ctx context.Context, courseID, instructorID string) error

	// Review management
	CreateReview(ctx context.Context, review *Review) (*Review, error)
	UpdateReview(ctx context.Context, review *Review) (*Review, error)
	GetReviews(ctx context.Context, courseID string) ([]*Review, error)

	// Certificate management
	IssueCertificate(ctx context.Context, courseID, userID string) (*Certificate, error)
	GetCertificate(ctx context.Context, courseID, userID string) (*Certificate, error)
	GetUserCertificates(ctx context.Context, userID string) ([]*Certificate, error)

	// Course versioning & cloning
	CloneCourse(ctx context.Context, courseID string, newTitle string) (*Course, error)
	CreateVersion(ctx context.Context, courseID string, userID string) (*DomainCourseVersion, error)
	ListVersions(ctx context.Context, courseID string) ([]*DomainCourseVersion, error)
	RestoreVersion(ctx context.Context, courseID string, versionNumber int, userID string) (*Course, error)
	GetChangelog(ctx context.Context, courseID string) ([]*CourseChangelog, error)
}
