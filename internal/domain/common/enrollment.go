package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ProgressStatus string

const (
	ProgressStatusNotStarted ProgressStatus = "NOT_STARTED"
	ProgressStatusInProgress ProgressStatus = "IN_PROGRESS"
	ProgressStatusCompleted  ProgressStatus = "COMPLETED"
)

type Enrollment struct {
	ID         string          `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID     string          `gorm:"not null;type:uuid;column:user_id;index:idx_user_subject,unique;constraint:OnDelete:CASCADE" json:"userId"`
	SubjectID  string          `gorm:"not null;type:uuid;column:subject_id;index:idx_user_subject,unique;constraint:OnDelete:CASCADE" json:"subjectId"`
	Progress   decimal.Decimal `gorm:"default:0;type:numeric(5,2);check:progress >= 0 AND progress <= 100;index;column:progress" json:"progress"`
	EnrolledAt time.Time       `gorm:"index;column:enrolled_at" json:"enrolledAt"`
	CreatedAt  time.Time       `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt  time.Time       `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt  gorm.DeletedAt  `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	Subject Subject `gorm:"foreignKey:SubjectID" json:"subject,omitempty"`
	User    User    `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (Enrollment) TableName() string {
	return "SubjectEnrollment"
}

type LessonProgress struct {
	ID                  string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID              string         `gorm:"not null;type:uuid;column:user_id;index:idx_user_lesson,unique" json:"userId"`
	LessonID            string         `gorm:"column:sub_topic_id;not null;type:uuid;index:idx_user_lesson,unique" json:"lessonId"`
	Status              ProgressStatus `gorm:"default:'NOT_STARTED';index;column:status" json:"status"`
	Completed           bool           `gorm:"default:false;index;column:completed" json:"completed"`
	TimeSpentSeconds    int            `gorm:"default:0;column:time_spent_seconds" json:"timeSpentSeconds"`
	LastWatchedPosition int            `gorm:"default:0;column:last_watched_position" json:"lastWatchedPosition"`
	CreatedAt           time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt           time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt           gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (LessonProgress) TableName() string {
	return "TopicProgress"
}

// LessonNoteContent stores a user's notes for one lesson as a single
// serialized blob (the video player composes free-form text plus a
// timestamped-lines block client-side and overwrites the whole thing on
// every save — see src/components/video/player/hooks/useTimelineNotes.ts).
type LessonNoteContent struct {
	ID        string    `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID    string    `gorm:"not null;type:uuid;column:user_id;index:idx_note_user_lesson,unique" json:"userId"`
	LessonID  string    `gorm:"not null;type:uuid;column:lesson_id;index:idx_note_user_lesson,unique" json:"lessonId"`
	Content   string    `gorm:"not null;type:text;default:'';column:content" json:"content"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (LessonNoteContent) TableName() string {
	return "LessonNoteContent"
}

func (n *LessonNoteContent) BeforeCreate(tx *gorm.DB) (err error) {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	return
}

// LessonTranscript stores an admin/instructor-uploaded SRT or VTT transcript
// for a lesson, used by the video player's searchable-transcript panel.
type LessonTranscript struct {
	ID        string    `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	LessonID  string    `gorm:"not null;type:uuid;column:lesson_id;uniqueIndex" json:"lessonId"`
	Format    string    `gorm:"not null;default:'srt';column:format" json:"format"`
	Content   string    `gorm:"not null;type:text;column:content" json:"content"`
	Language  string    `gorm:"not null;default:'ar';column:language" json:"language"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (LessonTranscript) TableName() string {
	return "LessonTranscript"
}

func (t *LessonTranscript) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return
}

func (e *Enrollment) BeforeCreate(tx *gorm.DB) (err error) {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	if e.EnrolledAt.IsZero() {
		e.EnrolledAt = time.Now()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	if e.UpdatedAt.IsZero() {
		e.UpdatedAt = time.Now()
	}
	return
}

func (lp *LessonProgress) BeforeCreate(tx *gorm.DB) (err error) {
	if lp.ID == "" {
		lp.ID = uuid.New().String()
	}
	return
}
