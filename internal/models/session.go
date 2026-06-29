package models

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserSession struct {
	ID                string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID            string         `gorm:"not null;type:uuid;index;column:user_id" json:"userId"`
	RefreshToken      string         `gorm:"uniqueIndex;not null;column:refresh_token" json:"-"`
	RefreshTokenHash  string         `gorm:"uniqueIndex:idx_user_session_refresh_hash;not null;column:refresh_token_hash" json:"-"`
	UserAgent         string         `gorm:"type:text;column:user_agent" json:"userAgent"`
	IP                string         `gorm:"not null;column:ip" json:"ip"`
	Location          *string        `gorm:"column:location" json:"location"`
	Browser           string         `gorm:"column:browser" json:"browser"`
	OS                string         `gorm:"column:os" json:"os"`
	Country           string         `gorm:"column:country" json:"country"`
	DeviceType        string         `gorm:"column:device_type" json:"deviceType"`
	FingerprintHash   string         `gorm:"column:fingerprint_hash;type:varchar(64)" json:"-"`
	RememberMe        bool           `gorm:"column:remember_me;default:false" json:"rememberMe"`
	Status            string         `gorm:"default:'active';column:status" json:"status"` // active, expired, revoked
	IsActive          bool           `gorm:"default:true;index;column:is_active" json:"isActive"`
	LastAccessed      time.Time      `gorm:"column:last_accessed" json:"lastActive"`
	ExpiresAt         time.Time      `gorm:"index;column:expires_at" json:"expiresAt"`
	AbsoluteExpiresAt time.Time      `gorm:"column:absolute_expires_at" json:"absoluteExpiresAt"`
	RevokedAt         *time.Time     `gorm:"column:revoked_at" json:"revokedAt,omitempty"`
	RevokedBy         *string        `gorm:"type:uuid;column:revoked_by" json:"revokedBy,omitempty"`
	CreatedAt         time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt         time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt         gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (UserSession) TableName() string {
	return "UserSession"
}

func (s *UserSession) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	// Auto-compute hash if refresh_token is set but hash is empty
	if s.RefreshToken != "" && s.RefreshTokenHash == "" {
		s.RefreshTokenHash = ComputeRefreshTokenHash(s.RefreshToken)
	}

	// Limit active sessions to 5 per user to prevent session hijacking and resource bloating
	var activeCount int64
	tx.Model(&UserSession{}).
		Where("user_id = ? AND is_active = ?", s.UserID, true).
		Count(&activeCount)

	if activeCount >= 5 {
		var oldestSessions []UserSession
		toRevokeCount := int(activeCount - 4)
		if err := tx.Where("user_id = ? AND is_active = ?", s.UserID, true).
			Order("last_accessed ASC").
			Limit(toRevokeCount).
			Find(&oldestSessions).Error; err == nil {
			for _, os := range oldestSessions {
				tx.Model(&os).Updates(map[string]interface{}{
					"is_active":  false,
					"status":     "revoked",
					"revoked_at": time.Now(),
				})
			}
		}
	}
	return
}

func (s *UserSession) BeforeUpdate(tx *gorm.DB) (err error) {
	// Auto-update hash if refresh_token changed
	if s.RefreshToken != "" {
		hash := ComputeRefreshTokenHash(s.RefreshToken)
		if hash != s.RefreshTokenHash {
			s.RefreshTokenHash = hash
		}
	}
	return
}

// ComputeRefreshTokenHash computes a SHA-256 hash of the refresh token for fast lookups.
func ComputeRefreshTokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// IsExpired checks if the session has expired (soft or absolute).
func (s *UserSession) IsExpired() bool {
	now := time.Now()
	if now.After(s.AbsoluteExpiresAt) {
		return true
	}
	return now.After(s.ExpiresAt)
}

// RefreshSession updates an existing session with new token data (avoids DELETE+INSERT).
func (s *UserSession) RefreshSession(newToken string, newExpiresAt time.Time) {
	s.RefreshToken = newToken
	s.RefreshTokenHash = ComputeRefreshTokenHash(newToken)
	s.LastAccessed = time.Now()
	s.ExpiresAt = newExpiresAt
	s.UpdatedAt = time.Now()
}

// IsExpiredByInactivity checks if the session has expired due to inactivity (idle timeout).
func (s *UserSession) IsExpiredByInactivity(idleTimeout time.Duration) bool {
	return time.Now().After(s.LastAccessed.Add(idleTimeout))
}

// RemainingTime returns the time until the session expires.
func (s *UserSession) RemainingTime() time.Duration {
	return time.Until(s.AbsoluteExpiresAt)
}

// ShouldSlideExpiration checks if the session should be extended based on sliding window.
func (s *UserSession) ShouldSlideExpiration(slidingWindow time.Duration) bool {
	return time.Since(s.LastAccessed) < slidingWindow && !s.IsExpired()
}

// String returns a debug representation (only hash, never the actual token).
func (s *UserSession) String() string {
	return fmt.Sprintf("UserSession{id=%s, user=%s, hash=%s, active=%v, expires=%s}",
		s.ID[:8], s.UserID[:8], s.RefreshTokenHash[:12], s.IsActive, s.ExpiresAt.Format(time.RFC3339))
}
