package repositories

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
)

// ----------------------------
// Assignments
// ----------------------------

func (r *LmsRepository) CreateAssignment(a *models.LmsAssignment) error {
	return r.db.Create(a).Error
}

func (r *LmsRepository) GetAssignmentByID(id uuid.UUID) (*models.LmsAssignment, error) {
	var a models.LmsAssignment
	if err := r.db.First(&a, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *LmsRepository) ListAssignmentsByCourseID(courseID uuid.UUID) ([]models.LmsAssignment, error) {
	var assignments []models.LmsAssignment
	err := r.db.Where("course_id = ?", courseID).Order("created_at DESC").Find(&assignments).Error
	return assignments, err
}

func (r *LmsRepository) UpdateAssignment(a *models.LmsAssignment) error {
	return r.db.Save(a).Error
}

func (r *LmsRepository) DeleteAssignment(id uuid.UUID) error {
	return r.db.Delete(&models.LmsAssignment{}, "id = ?", id).Error
}

// LinkAssignmentToLesson sets (or clears, if lessonID is nil) which lesson an
// assignment is linked to.
func (r *LmsRepository) LinkAssignmentToLesson(assignmentID uuid.UUID, lessonID *uuid.UUID) error {
	return r.db.Model(&models.LmsAssignment{}).Where("id = ?", assignmentID).Update("lesson_id", lessonID).Error
}
