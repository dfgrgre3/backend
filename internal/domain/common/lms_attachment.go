package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LmsAttachment represents downloadable files
type LmsAttachment struct {
	ID        uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	LessonID  uuid.UUID      `gorm:"not null;type:uuid;index;column:lesson_id;constraint:OnDelete:CASCADE" json:"lessonId"`
	Title     string         `gorm:"not null;column:title" json:"title"`
	FileURL   string         `gorm:"not null;column:file_url" json:"fileUrl"`
	FileType  string         `gorm:"column:file_type" json:"fileType"`
	FileSize  *int64         `gorm:"column:file_size" json:"fileSize,omitempty"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"createdAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (LmsAttachment) TableName() string {
	return "LmsAttachment"
}

func (a *LmsAttachment) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return
}
