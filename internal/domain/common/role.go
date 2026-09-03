package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Role struct {
	ID          string    `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name        string    `gorm:"not null;uniqueIndex;column:name" json:"name"`
	Description *string   `gorm:"type:text;column:description" json:"description"`
	IsSystem    bool      `gorm:"not null;default:false;index;column:is_system" json:"isSystem"`
	Level       int       `gorm:"not null;default:0;index;column:level" json:"level"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (Role) TableName() string {
	return "roles"
}

func (r *Role) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return
}
