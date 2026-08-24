package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LmsInteractiveQuiz represents inline interactive video questions
type LmsInteractiveQuiz struct {
	ID           uuid.UUID       `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	LessonID     uuid.UUID       `gorm:"not null;type:uuid;index;column:lesson_id;constraint:OnDelete:CASCADE" json:"lessonId"`
	TimestampSec int             `gorm:"not null;column:timestamp_sec" json:"timestampSec"`
	Question     string          `gorm:"not null;type:text;column:question" json:"question"`
	Options      json.RawMessage `gorm:"type:jsonb;column:options" json:"options"`
	CorrectIndex int             `gorm:"not null;column:correct_index" json:"correctIndex"`
	CreatedAt    time.Time       `gorm:"column:created_at" json:"createdAt"`
	DeletedAt    gorm.DeletedAt  `gorm:"index;column:deleted_at" json:"-"`
}

func (LmsInteractiveQuiz) TableName() string {
	return "LmsInteractiveQuiz"
}

func (q *LmsInteractiveQuiz) BeforeCreate(tx *gorm.DB) (err error) {
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	return
}
