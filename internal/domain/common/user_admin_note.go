package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserAdminNote is an internal, admin-only note attached to a user profile.
// Never surfaced to the user themselves — visible to staff only.
type UserAdminNote struct {
	ID        string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID    string         `gorm:"index;type:uuid;column:user_id" json:"userId"`
	Content   string         `gorm:"type:text;column:content" json:"content"`
	CreatedBy string         `gorm:"type:uuid;column:created_by" json:"createdById"`
	Creator   *User          `gorm:"foreignKey:CreatedBy" json:"-"`
	UpdatedBy *string        `gorm:"type:uuid;column:updated_by" json:"updatedById"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (UserAdminNote) TableName() string { return "UserAdminNote" }

func (n *UserAdminNote) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	return nil
}
