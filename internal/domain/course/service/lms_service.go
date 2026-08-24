package courseservice

import (
	"errors"
	"fmt"
	models "thanawy-backend/internal/domain/common"
	courserepo "thanawy-backend/internal/infrastructure/persistence/repositories"

	"github.com/google/uuid"
)

var (
	CourseStatusDraft       = models.CourseStatusDraft
	CourseStatusUnderReview = models.CourseStatusUnderReview
	CourseStatusPublished   = models.CourseStatusPublished
	CourseStatusArchived    = models.CourseStatusArchived
	CourseStatusRejected    = models.CourseStatusRejected
)

// LmsService contains business logic for the LMS module.
//
// Its methods are split across several files in this package (all sharing
// the courseservice package and the LmsService receiver), grouped by area:
// this file (course CRUD), lms_service_workflow.go, lms_service_sections_lessons.go,
// lms_service_enrollment.go, lms_service_interaction.go, lms_service_media.go,
// lms_service_assignments.go, lms_service_certificates.go, lms_service_quizzes.go,
// lms_service_pricing.go, lms_service_bundles.go, lms_service_taxonomy.go,
// lms_service_instructors.go, lms_service_versioning.go and lms_service_reviews.go.
type LmsService struct {
	repo *courserepo.LmsRepository
}

// NewLmsService creates a new LmsService.
func NewLmsService(repo *courserepo.LmsRepository) *LmsService {
	return &LmsService{repo: repo}
}

// ----------------------------
// Course
// ----------------------------

// CreateCourse creates a new course.
func (s *LmsService) CreateCourse(title, slug string, instructorID uuid.UUID) (*models.LmsCourse, error) {
	if title == "" || slug == "" {
		return nil, errors.New("title and slug are required")
	}
	course := &models.LmsCourse{
		Title:               title,
		Slug:                slug,
		PrimaryInstructorID: instructorID,
		Status:              models.CourseStatusDraft,
	}
	if err := s.repo.CreateCourse(course); err != nil {
		return nil, fmt.Errorf("failed to create course: %w", err)
	}
	return course, nil
}

// GetCourse retrieves a course by ID.
func (s *LmsService) GetCourse(id uuid.UUID) (*models.LmsCourse, error) {
	return s.repo.GetCourseByID(id)
}

// GetCourseBySlug retrieves a course by slug.
func (s *LmsService) GetCourseBySlug(slug string) (*models.LmsCourse, error) {
	return s.repo.GetCourseBySlug(slug)
}

// UpdateCourse updates a course.
func (s *LmsService) UpdateCourse(course *models.LmsCourse) error {
	return s.repo.UpdateCourse(course)
}

// DeleteCourse soft-deletes a course.
func (s *LmsService) DeleteCourse(id uuid.UUID) error {
	return s.repo.DeleteCourse(id)
}

// ListCourses returns paginated courses.
func (s *LmsService) ListCourses(page, pageSize int, status, level, language string) ([]models.LmsCourse, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return s.repo.ListCourses(page, pageSize, status, level, language)
}
