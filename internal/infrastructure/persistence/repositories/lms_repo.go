package repositories

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LmsRepository handles all DB operations for the LMS module.
//
// Its methods are split across several files in this package (all sharing
// the repositories package and the LmsRepository receiver), grouped by area:
// this file (course CRUD), lms_repo_sections.go, lms_repo_lessons.go,
// lms_repo_pricing.go, lms_repo_bundles.go, lms_repo_enrollment.go,
// lms_repo_interaction.go, lms_repo_media.go, lms_repo_assignments.go,
// lms_repo_certificates.go, lms_repo_quizzes.go, lms_repo_taxonomy.go,
// lms_repo_instructors.go, lms_repo_versioning.go and lms_repo_reviews.go.
type LmsRepository struct {
	db *gorm.DB
}

// NewLmsRepository creates a new LmsRepository.
func NewLmsRepository(db *gorm.DB) *LmsRepository {
	return &LmsRepository{db: db}
}

// ----------------------------
// Course CRUD
// ----------------------------

// CreateCourse creates a new course with optional relations.
func (r *LmsRepository) CreateCourse(course *models.LmsCourse) error {
	return r.db.Create(course).Error
}

// GetCourseByID fetches a course by ID with sections, lessons, pricings, instructors.
func (r *LmsRepository) GetCourseByID(id uuid.UUID) (*models.LmsCourse, error) {
	var c models.LmsCourse
	err := r.db.
		Preload("Sections.Lessons.Attachments").
		Preload("Sections.Lessons.Subtitles").
		Preload("Sections.Lessons.Quizzes").
		Preload("Pricings").
		Preload("Instructors").
		First(&c, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetCourseBySlug fetches a course by slug.
func (r *LmsRepository) GetCourseBySlug(slug string) (*models.LmsCourse, error) {
	var c models.LmsCourse
	err := r.db.
		Preload("Sections.Lessons").
		Preload("Pricings").
		First(&c, "slug = ?", slug).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpdateCourse updates a course.
func (r *LmsRepository) UpdateCourse(course *models.LmsCourse) error {
	return r.db.Save(course).Error
}

// UpdateCourseStatus updates only the status field.
func (r *LmsRepository) UpdateCourseStatus(id uuid.UUID, status models.CourseStatus) error {
	return r.db.Model(&models.LmsCourse{}).Where("id = ?", id).Update("status", status).Error
}

// DeleteCourse soft-deletes a course.
func (r *LmsRepository) DeleteCourse(id uuid.UUID) error {
	result := r.db.Delete(&models.LmsCourse{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListCourses returns paginated courses with optional filters.
func (r *LmsRepository) ListCourses(page, pageSize int, status string, level string, language string) ([]models.LmsCourse, int64, error) {
	var courses []models.LmsCourse
	var total int64

	q := r.db.Model(&models.LmsCourse{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if level != "" {
		q = q.Where("level = ?", level)
	}
	if language != "" {
		q = q.Where("language = ?", language)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := q.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&courses).Error; err != nil {
		return nil, 0, err
	}
	return courses, total, nil
}
