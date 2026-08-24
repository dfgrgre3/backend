package repositories

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
	"gorm.io/gorm/clause"
)

// ----------------------------
// Lesson Interaction & Video Notes
// ----------------------------

func (r *LmsRepository) UpsertLessonInteraction(i *models.LmsLessonInteraction) error {
	// Upsert by unique (lesson_id, user_id)
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "lesson_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"watched_duration_sec", "last_position_sec", "play_count", "is_completed", "quiz_answers", "updated_at",
		}),
	}).Create(i).Error
}

func (r *LmsRepository) GetLessonInteraction(lessonID, userID uuid.UUID) (*models.LmsLessonInteraction, error) {
	var i models.LmsLessonInteraction
	err := r.db.Where("lesson_id = ? AND user_id = ?", lessonID, userID).First(&i).Error
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (r *LmsRepository) CreateVideoNote(n *models.LmsVideoNote) error {
	return r.db.Create(n).Error
}

func (r *LmsRepository) ListVideoNotes(lessonID, userID uuid.UUID) ([]models.LmsVideoNote, error) {
	var notes []models.LmsVideoNote
	err := r.db.Where("lesson_id = ? AND user_id = ?", lessonID, userID).Order("timestamp_sec ASC").Find(&notes).Error
	return notes, err
}

func (r *LmsRepository) DeleteVideoNote(id uuid.UUID) error {
	return r.db.Delete(&models.LmsVideoNote{}, "id = ?", id).Error
}
