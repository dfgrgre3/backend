package repositories

import (
	models "thanawy-backend/internal/domain/common"
	"time"

	"github.com/google/uuid"
)

// ----------------------------
// Enrollment
// ----------------------------

func (r *LmsRepository) CreateEnrollment(e *models.LmsEnrollment) error {
	return r.db.Create(e).Error
}

func (r *LmsRepository) GetEnrollment(courseID, userID uuid.UUID) (*models.LmsEnrollment, error) {
	var e models.LmsEnrollment
	err := r.db.Where("course_id = ? AND user_id = ?", courseID, userID).First(&e).Error
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *LmsRepository) UpdateEnrollmentProgress(courseID, userID uuid.UUID, progress float64) error {
	return r.db.Model(&models.LmsEnrollment{}).
		Where("course_id = ? AND user_id = ?", courseID, userID).
		Updates(map[string]interface{}{
			"progress":   progress,
			"updated_at": time.Now(),
		}).Error
}

func (r *LmsRepository) CompleteEnrollment(courseID, userID uuid.UUID) error {
	return r.db.Model(&models.LmsEnrollment{}).
		Where("course_id = ? AND user_id = ?", courseID, userID).
		Updates(map[string]interface{}{
			"progress":     100,
			"completed_at": time.Now(),
			"updated_at":   time.Now(),
		}).Error
}

func (r *LmsRepository) ListEnrollmentsByUserID(userID uuid.UUID) ([]models.LmsEnrollment, error) {
	var enrollments []models.LmsEnrollment
	err := r.db.Where("user_id = ?", userID).Find(&enrollments).Error
	return enrollments, err
}

func (r *LmsRepository) ListEnrollmentsByCourseID(courseID uuid.UUID) ([]models.LmsEnrollment, error) {
	var enrollments []models.LmsEnrollment
	err := r.db.Where("course_id = ?", courseID).Find(&enrollments).Error
	return enrollments, err
}

// EnrollUserInBundle enrolls a user in all courses of a bundle.
func (r *LmsRepository) EnrollUserInBundle(bundleID, userID uuid.UUID) error {
	var bundle models.LmsBundle
	if err := r.db.Preload("Courses").First(&bundle, "id = ?", bundleID).Error; err != nil {
		return err
	}
	tx := r.db.Begin()
	for _, c := range bundle.Courses {
		e := models.LmsEnrollment{
			CourseID:   c.ID,
			UserID:     userID,
			BundleID:   &bundleID,
			EnrolledAt: time.Now(),
		}
		if err := tx.Where("course_id = ? AND user_id = ?", c.ID, userID).FirstOrCreate(&e).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}
