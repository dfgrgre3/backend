package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// UserCredential stores authentication credentials separately from user profile
// This improves security by separating sensitive auth data from profile data
type UserCredential struct {
	ID             string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID         string         `gorm:"not null;uniqueIndex;type:uuid;column:user_id;constraint:OnDelete:CASCADE" json:"userId" binding:"required,uuid"`
	PasswordHash   string         `gorm:"not null;column:password_hash" json:"-" binding:"required"`
	LastChangedAt  time.Time      `gorm:"column:last_changed_at" json:"lastChangedAt"`
	ExpiresAt      *time.Time     `gorm:"column:expires_at" json:"expiresAt"`
	ResetToken     *string        `gorm:"index;column:reset_token" json:"-"`
	ResetExpiresAt *time.Time     `gorm:"column:reset_expires_at" json:"-"`
	CreatedAt      time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

func (UserCredential) TableName() string {
	return "UserCredential"
}

func (uc *UserCredential) BeforeCreate(tx *gorm.DB) (err error) {
	if uc.ID == "" {
		uc.ID = uuid.New().String()
	}
	if uc.LastChangedAt.IsZero() {
		uc.LastChangedAt = time.Now()
	}
	return
}

func (uc *UserCredential) BeforeUpdate(tx *gorm.DB) (err error) {
	// Update last changed timestamp when password changes
	if tx.Statement.Changed("PasswordHash") {
		uc.LastChangedAt = time.Now()
	}
	return
}

// SetPassword hashes and sets the password
func (uc *UserCredential) SetPassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	
	uc.PasswordHash = string(hashedPassword)
	return nil
}

// CheckPassword verifies if the provided password matches the hash
func (uc *UserCredential) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(uc.PasswordHash), []byte(password))
	return err == nil
}

// GenerateResetToken generates a password reset token
func (uc *UserCredential) GenerateResetToken() string {
	token := uuid.New().String()
	uc.ResetToken = &token
	expiresAt := time.Now().Add(1 * time.Hour)
	uc.ResetExpiresAt = &expiresAt
	return token
}

// ClearResetToken clears the reset token after successful password reset
func (uc *UserCredential) ClearResetToken() {
	uc.ResetToken = nil
	uc.ResetExpiresAt = nil
}

// IsResetTokenValid checks if the reset token is valid and not expired
func (uc *UserCredential) IsResetTokenValid(token string) bool {
	if uc.ResetToken == nil || uc.ResetExpiresAt == nil {
		return false
	}
	
	if *uc.ResetToken != token {
		return false
	}
	
	if time.Now().After(*uc.ResetExpiresAt) {
		return false
	}
	
	return true
}

// IsPasswordExpired checks if the password has expired
func (uc *UserCredential) IsPasswordExpired() bool {
	if uc.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*uc.ExpiresAt)
}

// TwoFactorCredential stores 2FA secrets separately
type TwoFactorCredential struct {
	ID              string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID          string         `gorm:"not null;uniqueIndex;type:uuid;column:user_id;constraint:OnDelete:CASCADE" json:"userId" binding:"required,uuid"`
	Secret          string         `gorm:"not null;column:secret" json:"-"`
	Enabled         bool           `gorm:"default:false;column:enabled" json:"enabled"`
	BackupCodes     string         `gorm:"type:jsonb;column:backup_codes" json:"-"`
	LastUsedAt      *time.Time     `gorm:"column:last_used_at" json:"lastUsedAt"`
	CreatedAt       time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt       time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

func (TwoFactorCredential) TableName() string {
	return "TwoFactorCredential"
}

func (tfc *TwoFactorCredential) BeforeCreate(tx *gorm.DB) (err error) {
	if tfc.ID == "" {
		tfc.ID = uuid.New().String()
	}
	return
}

// SessionCredential stores session tokens separately
type SessionCredential struct {
	ID           string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID       string         `gorm:"not null;index;type:uuid;column:user_id;constraint:OnDelete:CASCADE" json:"userId" binding:"required,uuid"`
	RefreshToken string         `gorm:"not null;uniqueIndex;column:refresh_token" json:"-"`
	DeviceID     *string        `gorm:"index;column:device_id" json:"deviceId"`
	UserAgent    *string        `gorm:"column:user_agent" json:"userAgent"`
	IPAddress    *string        `gorm:"column:ip_address" json:"ipAddress"`
	ExpiresAt    time.Time      `gorm:"index;column:expires_at" json:"expiresAt"`
	CreatedAt    time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

func (SessionCredential) TableName() string {
	return "SessionCredential"
}

func (sc *SessionCredential) BeforeCreate(tx *gorm.DB) (err error) {
	if sc.ID == "" {
		sc.ID = uuid.New().String()
	}
	// Trim user agent
	if sc.UserAgent != nil {
		ua := strings.TrimSpace(*sc.UserAgent)
		sc.UserAgent = &ua
	}
	return
}

// IsExpired checks if the session has expired
func (sc *SessionCredential) IsExpired() bool {
	return time.Now().After(sc.ExpiresAt)
}
