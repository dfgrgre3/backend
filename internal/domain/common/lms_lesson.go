package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LmsLesson represents individual course lessons with drip content rules
type LmsLesson struct {
	ID               uuid.UUID        `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SectionID        uuid.UUID        `gorm:"not null;type:uuid;index;column:section_id;constraint:OnDelete:CASCADE" json:"sectionId"`
	Title            string           `gorm:"not null;column:title" json:"title"`
	Type             LessonType       `gorm:"default:'VIDEO';index;column:type" json:"type"`
	Content          *string          `gorm:"type:text;column:content" json:"content,omitempty"`
	MediaURL         *string          `gorm:"column:media_url" json:"mediaUrl,omitempty"`
	DurationSeconds  int              `gorm:"default:0;column:duration_seconds" json:"durationSeconds"`
	IsFreePreview    bool             `gorm:"default:false;index;column:is_free_preview" json:"isFreePreview"`
	OrderIndex       int              `gorm:"default:0;index;column:order_index" json:"orderIndex"`
	AvailabilityType AvailabilityType `gorm:"default:'CALENDAR_DATE';column:availability_type" json:"availabilityType"`
	AvailableFrom    *time.Time       `gorm:"column:available_from" json:"availableFrom,omitempty"`
	DripDelayDays    *int             `gorm:"column:drip_delay_days" json:"dripDelayDays,omitempty"`
	// ExamID links this lesson to a single exam from the legacy Subject-scoped
	// Exam table. Loose reference by design (no FK constraint) — Exam rows
	// live outside the Course/Section/Lesson hierarchy. Mirrors SubTopic.ExamID.
	ExamID    *string        `gorm:"index;type:uuid;column:exam_id" json:"examId,omitempty"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Associations
	Attachments []LmsAttachment      `gorm:"foreignKey:LessonID;constraint:OnDelete:CASCADE" json:"attachments,omitempty"`
	Subtitles   []LmsSubtitle        `gorm:"foreignKey:LessonID;constraint:OnDelete:CASCADE" json:"subtitles,omitempty"`
	Quizzes     []LmsInteractiveQuiz `gorm:"foreignKey:LessonID;constraint:OnDelete:CASCADE" json:"quizzes,omitempty"`
	Exam        *Exam                `gorm:"foreignKey:ExamID" json:"exam,omitempty"`
}

func (LmsLesson) TableName() string {
	return "LmsLesson"
}

func (l *LmsLesson) BeforeCreate(tx *gorm.DB) (err error) {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return
}
