package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Admin/system models are split across several files in this package (all
// sharing package models), one model group per file: this file
// (InstructorPayout), role.go, user_group.go, mfa.go, scheduled_task.go,
// queue_job.go, cache_entry.go, email_template.go, feature_flag.go,
// api_key.go, webhook.go, import_export_job.go, system_log.go,
// activity_log.go and login_attempt.go.

type PayoutStatus string

const (
	PayoutPending  PayoutStatus = "PENDING"
	PayoutApproved PayoutStatus = "APPROVED"
	PayoutPaid     PayoutStatus = "PAID"
	PayoutRejected PayoutStatus = "REJECTED"
	PayoutFailed   PayoutStatus = "FAILED"
)

type InstructorPayout struct {
	ID            string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	InstructorID  string         `gorm:"not null;index;type:uuid;column:instructor_id" json:"instructorId"`
	Amount        float64        `gorm:"not null;check:amount >= 0;column:amount" json:"amount"`
	Currency      string         `gorm:"not null;default:'EGP';column:currency" json:"currency"`
	Status        PayoutStatus   `gorm:"not null;default:'PENDING';index;column:status" json:"status"`
	PaymentMethod string         `gorm:"column:payment_method" json:"paymentMethod"`
	TransactionID *string        `gorm:"column:transaction_id" json:"transactionId"`
	Notes         *string        `gorm:"type:text;column:notes" json:"notes"`
	ApprovedBy    *string        `gorm:"type:uuid;column:approved_by" json:"approvedBy"`
	ApprovedAt    *time.Time     `gorm:"column:approved_at" json:"approvedAt"`
	PaidAt        *time.Time     `gorm:"column:paid_at" json:"paidAt"`
	CreatedAt     time.Time      `gorm:"index;column:created_at" json:"requestedAt"`
	UpdatedAt     time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	Instructor User `gorm:"foreignKey:InstructorID;constraint:OnDelete:CASCADE" json:"-"`
}

func (InstructorPayout) TableName() string {
	return "InstructorPayout"
}

func (p *InstructorPayout) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return
}
