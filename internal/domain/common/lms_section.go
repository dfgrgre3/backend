package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LmsSection represents a section/module in a course
type LmsSection struct {
	ID            uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	CourseID      uuid.UUID      `gorm:"not null;type:uuid;index;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	Title         string         `gorm:"not null;column:title" json:"title"`
	OrderIndex    int            `gorm:"default:0;index;column:order_index" json:"orderIndex"`
	AvailableFrom *time.Time     `gorm:"column:available_from" json:"availableFrom,omitempty"`
	DripDelayDays *int           `gorm:"column:drip_delay_days" json:"dripDelayDays,omitempty"`
	CreatedAt     time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Associations
	Lessons []LmsLesson `gorm:"foreignKey:SectionID;constraint:OnDelete:CASCADE" json:"lessons,omitempty"`
}

func (LmsSection) TableName() string {
	return "LmsSection"
}

func (s *LmsSection) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return
}
