package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LmsVideoNote represents user timestamped bookmarks and notes
type LmsVideoNote struct {
	ID           uuid.UUID `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	LessonID     uuid.UUID `gorm:"not null;type:uuid;index;column:lesson_id;constraint:OnDelete:CASCADE" json:"lessonId"`
	UserID       uuid.UUID `gorm:"not null;type:uuid;index;column:user_id;constraint:OnDelete:CASCADE" json:"userId"`
	TimestampSec int       `gorm:"not null;column:timestamp_sec" json:"timestampSec"`
	Note         string    `gorm:"not null;type:text;column:note" json:"note"`
	CreatedAt    time.Time `gorm:"index;column:created_at" json:"createdAt"`
}

func (LmsVideoNote) TableName() string {
	return "LmsVideoNote"
}

func (n *LmsVideoNote) BeforeCreate(tx *gorm.DB) (err error) {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return
}
