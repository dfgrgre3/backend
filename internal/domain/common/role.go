package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Role struct {
	ID          string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name        string         `gorm:"not null;uniqueIndex;column:name" json:"name"`
	Description *string        `gorm:"type:text;column:description" json:"description"`
	Permissions []byte         `gorm:"type:jsonb;column:permissions" json:"permissions"` // JSON array of permissions
	IsSystem    bool           `gorm:"not null;default:false;index;column:is_system" json:"isSystem"`
	CreatedAt   time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	Users []User `gorm:"foreignKey:RoleID;constraint:OnDelete:SET NULL" json:"-"`
}

func (Role) TableName() string {
	return "Role"
}

func (r *Role) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return
}
