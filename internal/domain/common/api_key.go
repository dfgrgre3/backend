package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type APIKey struct {
	ID          string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name        string         `gorm:"not null;column:name" json:"name"`
	Key         string         `gorm:"not null;uniqueIndex;column:key" json:"key"`
	Permissions []byte         `gorm:"type:jsonb;column:permissions" json:"permissions"` // JSON array
	LastUsedAt  *time.Time     `gorm:"column:last_used_at" json:"lastUsedAt"`
	ExpiresAt   *time.Time     `gorm:"index;column:expires_at" json:"expiresAt"`
	IsActive    bool           `gorm:"not null;default:true;index;column:is_active" json:"isActive"`
	CreatedAt   time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt   time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (APIKey) TableName() string {
	return "APIKey"
}

func (a *APIKey) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return
}
