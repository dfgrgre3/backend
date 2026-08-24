package repositories

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Type aliases for domain types
type Course = models.Course
type CourseFilter = models.CourseFilter
type CourseStatus = models.CourseStatus
type CourseLevel = models.CourseLevel
type Section = models.Section
type Lesson = models.Lesson
type LessonType = models.LessonType
type AvailabilityType = models.AvailabilityType
type Enrollment = models.DomainEnrollment
type EnrollmentFilter = models.EnrollmentFilter
type Pricing = models.Pricing
type PricingType = models.DomainPricingType
type Category = models.DomainCategory
type Instructor = models.Instructor
type Review = models.Review
type Certificate = models.Certificate
type CourseVersion = models.DomainCourseVersion
type CourseChangelog = models.CourseChangelog
type PGStringArray = models.PGStringArray

// Helper function to parse UUID
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// GormRepository implements the Repository interface using GORM.
//
// Its methods are split across several files in this package (all sharing
// package repositories and the GormRepository receiver), grouped by area:
// this file (type aliases/construction), course_repository_course.go,
// course_repository_section.go, course_repository_lesson.go,
// course_repository_enrollment.go, course_repository_pricing.go,
// course_repository_category_instructor.go,
// course_repository_review_certificate.go,
// course_repository_versioning.go and course_repository_convert.go
// (domain<->model conversion helpers).
type GormRepository struct {
	repo *LmsRepository
}

// NewGormRepository creates a new GormRepository
func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{
		repo: NewLmsRepository(db),
	}
}
