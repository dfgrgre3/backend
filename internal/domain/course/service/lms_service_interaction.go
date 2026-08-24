package courseservice

import (
	"encoding/json"
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
)

// ----------------------------
// Lesson Interaction & Video Notes
// ----------------------------

// UpdateLessonInteraction upserts a lesson interaction record.
func (s *LmsService) UpdateLessonInteraction(lessonID, userID uuid.UUID, watchedDuration, lastPosition int, playCount int, isCompleted bool, quizAnswers json.RawMessage) error {
	interaction := &models.LmsLessonInteraction{
		LessonID:           lessonID,
		UserID:             userID,
		WatchedDurationSec: watchedDuration,
		LastPositionSec:    lastPosition,
		PlayCount:          playCount,
		IsCompleted:        isCompleted,
		QuizAnswers:        quizAnswers,
	}
	return s.repo.UpsertLessonInteraction(interaction)
}

// GetLessonInteraction retrieves a user's interaction with a lesson.
func (s *LmsService) GetLessonInteraction(lessonID, userID uuid.UUID) (*models.LmsLessonInteraction, error) {
	return s.repo.GetLessonInteraction(lessonID, userID)
}

// AddVideoNote adds a timestamped note for a lesson.
func (s *LmsService) AddVideoNote(lessonID, userID uuid.UUID, timestampSec int, note string) (*models.LmsVideoNote, error) {
	n := &models.LmsVideoNote{
		LessonID:     lessonID,
		UserID:       userID,
		TimestampSec: timestampSec,
		Note:         note,
	}
	if err := s.repo.CreateVideoNote(n); err != nil {
		return nil, err
	}
	return n, nil
}

// ListVideoNotes returns all video notes for a user in a lesson.
func (s *LmsService) ListVideoNotes(lessonID, userID uuid.UUID) ([]models.LmsVideoNote, error) {
	return s.repo.ListVideoNotes(lessonID, userID)
}
