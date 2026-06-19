package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

// Lesson represents a scheduled lesson/booking with a teacher
type Lesson struct {
	ID        string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID    string         `gorm:"not null;type:uuid;index:idx_lessons_user;column:user_id" json:"userId"`
	TeacherID string         `gorm:"not null;type:uuid;index:idx_lessons_teacher;column:teacher_id" json:"teacherId"`
	Title     string         `gorm:"not null;column:title" json:"title"`
	Location  string         `gorm:"not null;column:location" json:"location"`
	StartTime time.Time      `gorm:"not null;index:idx_lessons_start;column:start_time" json:"startTime"`
	EndTime   time.Time      `gorm:"not null;index:idx_lessons_end;column:end_time" json:"endTime"`
	CreatedAt time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	Teacher *User `gorm:"foreignKey:TeacherID" json:"teacher,omitempty"`
}

func (Lesson) TableName() string {
	return "Lesson"
}

func (l *Lesson) BeforeCreate(tx *gorm.DB) (err error) {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return
}