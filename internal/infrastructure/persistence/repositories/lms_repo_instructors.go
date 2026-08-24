package repositories

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
)

// ----------------------------
// Instructors
// ----------------------------

func (r *LmsRepository) AddInstructor(i *models.LmsInstructor) error {
	return r.db.Create(i).Error
}

func (r *LmsRepository) RemoveInstructor(courseID, instructorID uuid.UUID) error {
	return r.db.Where("course_id = ? AND instructor_id = ?", courseID, instructorID).Delete(&models.LmsInstructor{}).Error
}

func (r *LmsRepository) ListInstructors(courseID uuid.UUID) ([]models.LmsInstructor, error) {
	var instructors []models.LmsInstructor
	err := r.db.Where("course_id = ?", courseID).Find(&instructors).Error
	return instructors, err
}
