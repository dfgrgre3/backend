package repositories

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ----------------------------
// Lesson CRUD
// ----------------------------

func (r *LmsRepository) CreateLesson(lesson *models.LmsLesson) error {
	return r.db.Create(lesson).Error
}

func (r *LmsRepository) GetLessonByID(id uuid.UUID) (*models.LmsLesson, error) {
	var l models.LmsLesson
	err := r.db.Preload("Attachments").Preload("Subtitles").Preload("Quizzes").Preload("Exam").First(&l, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *LmsRepository) UpdateLesson(lesson *models.LmsLesson) error {
	return r.db.Save(lesson).Error
}

func (r *LmsRepository) DeleteLesson(id uuid.UUID) error {
	return r.db.Delete(&models.LmsLesson{}, "id = ?", id).Error
}

func (r *LmsRepository) ListLessonsBySectionID(sectionID uuid.UUID) ([]models.LmsLesson, error) {
	var lessons []models.LmsLesson
	err := r.db.Preload("Attachments").Preload("Exam").Where("section_id = ?", sectionID).Order("order_index ASC").Find(&lessons).Error
	return lessons, err
}

// LinkExamToLesson sets (or clears, if examID is nil) a lesson's linked exam.
func (r *LmsRepository) LinkExamToLesson(lessonID uuid.UUID, examID *string) error {
	return r.db.Model(&models.LmsLesson{}).Where("id = ?", lessonID).Update("exam_id", examID).Error
}

// ReorderLessons persists a new order_index for each lesson id in lessonIDs,
// in the order given (0-based). All updates run in one transaction so a
// partial failure never leaves the list half-reordered.
func (r *LmsRepository) ReorderLessons(sectionID uuid.UUID, lessonIDs []uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i, lessonID := range lessonIDs {
			if err := tx.Model(&models.LmsLesson{}).
				Where("id = ? AND section_id = ?", lessonID, sectionID).
				Update("order_index", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
