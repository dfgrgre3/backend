package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LiveSession is an instructor-hosted online class scheduled from the admin panel.
type LiveSession struct {
	ID          uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SubjectID   *uuid.UUID     `gorm:"type:uuid;index;column:subject_id" json:"subjectId,omitempty"`
	Title       string         `gorm:"not null;column:title" json:"title"`
	Description *string        `gorm:"type:text;column:description" json:"description,omitempty"`
	Provider    string         `gorm:"not null;default:'ZOOM';column:provider" json:"provider"`
	JoinURL     *string        `gorm:"type:text;column:join_url" json:"joinUrl,omitempty"`
	StartURL    *string        `gorm:"type:text;column:start_url" json:"startUrl,omitempty"`
	HostEmail   string         `gorm:"column:host_email" json:"hostEmail"`
	ScheduledAt time.Time      `gorm:"not null;index;column:scheduled_at" json:"scheduledAt"`
	DurationMin int            `gorm:"not null;default:60;column:duration_min" json:"durationMin"`
	Status      string         `gorm:"not null;default:'SCHEDULED';index;column:status" json:"status"`
	CreatedAt   time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time      `gorm:"column:updated_at" json:"-"`
	DeletedAt   gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (LiveSession) TableName() string { return "LiveSession" }

func (s *LiveSession) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return
}
