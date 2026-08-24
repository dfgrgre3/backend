package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LmsAssignment represents a course-scoped assignment/homework, optionally
// linked to exactly one lesson.
type LmsAssignment struct {
	ID          uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	CourseID    uuid.UUID      `gorm:"not null;type:uuid;index;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	LessonID    *uuid.UUID     `gorm:"type:uuid;index;column:lesson_id" json:"lessonId,omitempty"`
	Title       string         `gorm:"not null;column:title" json:"title"`
	Description *string        `gorm:"type:text;column:description" json:"description,omitempty"`
	DueDate     *time.Time     `gorm:"column:due_date" json:"dueDate,omitempty"`
	MaxScore    float64        `gorm:"default:100;column:max_score" json:"maxScore"`
	CreatedAt   time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (LmsAssignment) TableName() string {
	return "LmsAssignment"
}

func (a *LmsAssignment) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return
}
