package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LmsSubtitle represents VTT/SRT subtitles
type LmsSubtitle struct {
	ID        uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	LessonID  uuid.UUID      `gorm:"not null;type:uuid;index;column:lesson_id;constraint:OnDelete:CASCADE" json:"lessonId"`
	Language  string         `gorm:"not null;index;column:language" json:"language"`
	VTTURL    string         `gorm:"not null;column:vtt_url" json:"vttUrl"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"createdAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (LmsSubtitle) TableName() string {
	return "LmsSubtitle"
}

func (s *LmsSubtitle) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return
}
