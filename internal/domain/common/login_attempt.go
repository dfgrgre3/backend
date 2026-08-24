package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LoginAttemptStatus string

const (
	LoginAttemptSuccess LoginAttemptStatus = "SUCCESS"
	LoginAttemptFailed  LoginAttemptStatus = "FAILED"
	LoginAttemptBlocked LoginAttemptStatus = "BLOCKED"
)

type LoginAttempt struct {
	ID            string             `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID        *string            `gorm:"type:uuid;index;column:user_id" json:"userId"`
	IPAddress     string             `gorm:"not null;index;column:ip_address" json:"ipAddress"`
	UserAgent     string             `gorm:"type:text;column:user_agent" json:"userAgent"`
	Status        LoginAttemptStatus `gorm:"not null;index;column:status" json:"status"`
	FailureReason *string            `gorm:"type:text;column:failure_reason" json:"failureReason"`
	Location      *string            `gorm:"column:location" json:"location"`
	RiskScore     int                `gorm:"not null;default:0;column:risk_score" json:"riskScore"`
	CreatedAt     time.Time          `gorm:"not null;index;column:created_at" json:"createdAt"`

	// Relations
	User *User `gorm:"foreignKey:UserID;constraint:OnDelete:SET NULL" json:"-"`
}

func (LoginAttempt) TableName() string {
	return "LoginAttempt"
}

func (l *LoginAttempt) BeforeCreate(tx *gorm.DB) (err error) {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return
}
