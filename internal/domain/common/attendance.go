package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AttendanceStatus string

const (
	AttendancePresent AttendanceStatus = "present"
	AttendanceAbsent  AttendanceStatus = "absent"
	AttendanceLate    AttendanceStatus = "late"
	AttendanceExcused AttendanceStatus = "excused"
)

// Attendance records whether a user attended a session for a subject on a
// given date, optionally recorded by an admin/teacher.
type Attendance struct {
	ID          string           `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID      string           `gorm:"not null;index;type:uuid;column:user_id" json:"userId" binding:"required,uuid"`
	SubjectID   *string          `gorm:"index;type:uuid;column:subject_id" json:"subjectId"`
	SessionDate time.Time        `gorm:"not null;index;column:session_date" json:"sessionDate" binding:"required"`
	Status      AttendanceStatus `gorm:"not null;default:'present';column:status" json:"status" binding:"omitempty,oneof=present absent late excused"`
	Notes       string           `gorm:"column:notes" json:"notes"`
	RecordedBy  *string          `gorm:"type:uuid;column:recorded_by" json:"recordedBy"`
	CreatedAt   time.Time        `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time        `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt   `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	User    User     `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	Subject *Subject `gorm:"foreignKey:SubjectID;constraint:OnDelete:SET NULL" json:"subject,omitempty"`
}

func (Attendance) TableName() string {
	return "Attendance"
}

func (a *Attendance) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.Status == "" {
		a.Status = AttendancePresent
	}
	return
}

// StudentNote is a free-text note an admin/teacher attaches to a student's
// profile. Not visible to the student themself.
type StudentNote struct {
	ID        string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID    string         `gorm:"not null;index;type:uuid;column:user_id" json:"userId" binding:"required,uuid"`
	AuthorID  *string        `gorm:"type:uuid;column:author_id" json:"authorId"`
	Category  string         `gorm:"not null;default:'general';column:category" json:"category"`
	Note      string         `gorm:"not null;type:text;column:note" json:"note" binding:"required,min=1"`
	CreatedAt time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	User   User  `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	Author *User `gorm:"foreignKey:AuthorID;constraint:OnDelete:SET NULL" json:"author,omitempty"`
}

func (StudentNote) TableName() string {
	return "StudentNote"
}

func (n *StudentNote) BeforeCreate(tx *gorm.DB) (err error) {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	if n.Category == "" {
		n.Category = "general"
	}
	return
}
