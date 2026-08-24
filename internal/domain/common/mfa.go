package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MFAMethod string

const (
	MFAMethodTOTP  MFAMethod = "TOTP"
	MFAMethodSMS   MFAMethod = "SMS"
	MFAMethodEmail MFAMethod = "EMAIL"
)

type MFA struct {
	ID              string     `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID          string     `gorm:"not null;uniqueIndex;type:uuid;column:user_id" json:"userId"`
	IsEnabled       bool       `gorm:"not null;default:false;column:is_enabled" json:"isEnabled"`
	Method          *MFAMethod `gorm:"column:method" json:"method"`
	Secret          *string    `gorm:"column:secret" json:"-"`
	BackupCodes     *[]byte    `gorm:"type:jsonb;column:backup_codes" json:"-"`
	BackupCodesUsed int        `gorm:"not null;default:0;column:backup_codes_used" json:"backupCodesUsed"`
	LastUsedAt      *time.Time `gorm:"column:last_used_at" json:"lastUsedAt"`
	CreatedAt       time.Time  `gorm:"column:created_at" json:"enrolledAt"`
	UpdatedAt       time.Time  `gorm:"column:updated_at" json:"updatedAt"`

	// Relations
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

func (MFA) TableName() string {
	return "MFA"
}

func (m *MFA) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return
}
