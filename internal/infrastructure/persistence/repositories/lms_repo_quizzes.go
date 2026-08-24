package repositories

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
)

// ----------------------------
// Interactive Quizzes
// ----------------------------

func (r *LmsRepository) CreateQuiz(q *models.LmsInteractiveQuiz) error {
	return r.db.Create(q).Error
}

func (r *LmsRepository) ListQuizzes(lessonID uuid.UUID) ([]models.LmsInteractiveQuiz, error) {
	var quizzes []models.LmsInteractiveQuiz
	err := r.db.Where("lesson_id = ?", lessonID).Order("timestamp_sec ASC").Find(&quizzes).Error
	return quizzes, err
}

func (r *LmsRepository) DeleteQuiz(id uuid.UUID) error {
	return r.db.Delete(&models.LmsInteractiveQuiz{}, "id = ?", id).Error
}
