package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LmsReview represents course reviews
type LmsReview struct {
	ID        uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	CourseID  uuid.UUID      `gorm:"not null;type:uuid;index:idx_lms_review_user_course,unique;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	UserID    uuid.UUID      `gorm:"not null;type:uuid;index:idx_lms_review_user_course,unique;column:user_id;constraint:OnDelete:CASCADE" json:"userId"`
	Rating    int            `gorm:"not null;default:5;column:rating" json:"rating"`
	Comment   string         `gorm:"type:text;column:comment" json:"comment"`
	Status    string         `gorm:"default:'APPROVED';index;column:status" json:"status"` // APPROVED, PENDING, REJECTED
	Reply     *string        `gorm:"type:text;column:reply" json:"reply,omitempty"`
	CreatedAt time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (LmsReview) TableName() string {
	return "LmsReview"
}

func (r *LmsReview) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return
}

// LmsReviewComment holds reviewer comments during the workflow review process.
type LmsReviewComment struct {
	ID         uuid.UUID `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	CourseID   uuid.UUID `gorm:"not null;type:uuid;index;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	ReviewerID uuid.UUID `gorm:"not null;type:uuid;index;column:reviewer_id;constraint:OnDelete:CASCADE" json:"reviewerId"`
	Comment    string    `gorm:"not null;type:text;column:comment" json:"comment"`
	Status     string    `gorm:"default:'pending';column:status" json:"status"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (LmsReviewComment) TableName() string {
	return "LmsReviewComment"
}

func (rc *LmsReviewComment) BeforeCreate(tx *gorm.DB) (err error) {
	if rc.ID == uuid.Nil {
		rc.ID = uuid.New()
	}
	return
}
