package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// LmsEnrollment represents student enrollment in a course
type LmsEnrollment struct {
	ID          uuid.UUID       `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	CourseID    uuid.UUID       `gorm:"not null;type:uuid;index:idx_lms_enroll_user_course,unique;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	UserID      uuid.UUID       `gorm:"not null;type:uuid;index:idx_lms_enroll_user_course,unique;column:user_id;constraint:OnDelete:CASCADE" json:"userId"`
	Progress    decimal.Decimal `gorm:"default:0;type:numeric(5,2);check:progress >= 0 AND progress <= 100;column:progress" json:"progress"`
	EnrolledAt  time.Time       `gorm:"index;column:enrolled_at" json:"enrolledAt"`
	CompletedAt *time.Time      `gorm:"column:completed_at" json:"completedAt,omitempty"`
	BundleID    *uuid.UUID      `gorm:"type:uuid;index;column:bundle_id" json:"bundleId,omitempty"`
	CreatedAt   time.Time       `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time       `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt  `gorm:"index;column:deleted_at" json:"-"`
}

func (LmsEnrollment) TableName() string {
	return "LmsEnrollment"
}

func (e *LmsEnrollment) BeforeCreate(tx *gorm.DB) (err error) {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.EnrolledAt.IsZero() {
		e.EnrolledAt = time.Now()
	}
	return
}
