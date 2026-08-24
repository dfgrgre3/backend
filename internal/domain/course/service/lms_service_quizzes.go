package courseservice

import (
	"encoding/json"
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
)

// ----------------------------
// Interactive Quizzes
// ----------------------------

func (s *LmsService) AddQuiz(lessonID uuid.UUID, timestampSec int, question string, options json.RawMessage, correctIndex int) (*models.LmsInteractiveQuiz, error) {
	q := &models.LmsInteractiveQuiz{
		LessonID:     lessonID,
		TimestampSec: timestampSec,
		Question:     question,
		Options:      options,
		CorrectIndex: correctIndex,
	}
	if err := s.repo.CreateQuiz(q); err != nil {
		return nil, err
	}
	return q, nil
}

func (s *LmsService) ListQuizzes(lessonID uuid.UUID) ([]models.LmsInteractiveQuiz, error) {
	return s.repo.ListQuizzes(lessonID)
}
