package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FeatureFlag struct {
	ID                string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name              string         `gorm:"not null;column:name" json:"name"`
	Key               string         `gorm:"not null;uniqueIndex;column:key" json:"key"`
	Description       *string        `gorm:"type:text;column:description" json:"description"`
	IsEnabled         bool           `gorm:"not null;default:false;index;column:is_enabled" json:"isEnabled"`
	Environment       string         `gorm:"not null;default:'production';column:environment" json:"environment"`
	RolloutPercentage int            `gorm:"not null;default:0;column:rollout_percentage" json:"rolloutPercentage"`
	AllowedUsers      []byte         `gorm:"type:jsonb;column:allowed_users" json:"allowedUsers"` // JSON array of user IDs
	CreatedAt         time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt         time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt         gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (FeatureFlag) TableName() string {
	return "FeatureFlag"
}

func (f *FeatureFlag) BeforeCreate(tx *gorm.DB) (err error) {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	return
}
