package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Webhook struct {
	ID              string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name            string         `gorm:"not null;column:name" json:"name"`
	URL             string         `gorm:"not null;column:url" json:"url"`
	Events          []byte         `gorm:"type:jsonb;not null;column:events" json:"events"` // JSON array of event names
	IsActive        bool           `gorm:"not null;default:true;index;column:is_active" json:"isActive"`
	Secret          *string        `gorm:"column:secret" json:"-"`
	LastTriggeredAt *time.Time     `gorm:"column:last_triggered_at" json:"lastTriggeredAt"`
	SuccessCount    int            `gorm:"not null;default:0;column:success_count" json:"successCount"`
	FailureCount    int            `gorm:"not null;default:0;column:failure_count" json:"failureCount"`
	CreatedAt       time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt       time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (Webhook) TableName() string {
	return "Webhook"
}

func (w *Webhook) BeforeCreate(tx *gorm.DB) (err error) {
	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	return
}
