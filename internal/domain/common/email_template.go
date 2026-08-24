package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type EmailTemplate struct {
	ID         string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name       string         `gorm:"not null;column:name" json:"name"`
	Subject    string         `gorm:"not null;column:subject" json:"subject"`
	Body       string         `gorm:"type:text;not null;column:body" json:"body"`
	Category   string         `gorm:"not null;index;column:category" json:"category"`
	Variables  []byte         `gorm:"type:jsonb;column:variables" json:"variables"` // JSON array of variable names
	IsActive   bool           `gorm:"not null;default:true;index;column:is_active" json:"isActive"`
	LastUsedAt *time.Time     `gorm:"column:last_used_at" json:"lastUsedAt"`
	CreatedAt  time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt  time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt  gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (EmailTemplate) TableName() string {
	return "EmailTemplate"
}

func (e *EmailTemplate) BeforeCreate(tx *gorm.DB) (err error) {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	return
}
