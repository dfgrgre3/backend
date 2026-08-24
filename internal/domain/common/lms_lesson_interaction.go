package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LmsLessonInteraction tracks video watch progress & quiz answers
type LmsLessonInteraction struct {
	ID                 uuid.UUID       `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	LessonID           uuid.UUID       `gorm:"not null;type:uuid;index:idx_lms_interaction_user_lesson,unique;column:lesson_id;constraint:OnDelete:CASCADE" json:"lessonId"`
	UserID             uuid.UUID       `gorm:"not null;type:uuid;index:idx_lms_interaction_user_lesson,unique;column:user_id;constraint:OnDelete:CASCADE" json:"userId"`
	WatchedDurationSec int             `gorm:"default:0;column:watched_duration_sec" json:"watchedDurationSec"`
	LastPositionSec    int             `gorm:"default:0;column:last_position_sec" json:"lastPositionSec"`
	PlayCount          int             `gorm:"default:0;column:play_count" json:"playCount"`
	IsCompleted        bool            `gorm:"default:false;index;column:is_completed" json:"isCompleted"`
	QuizAnswers        json.RawMessage `gorm:"type:jsonb;column:quiz_answers" json:"quizAnswers,omitempty"`
	CreatedAt          time.Time       `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt          time.Time       `gorm:"column:updated_at" json:"updatedAt"`
}

func (LmsLessonInteraction) TableName() string {
	return "LmsLessonInteraction"
}

func (i *LmsLessonInteraction) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return
}
