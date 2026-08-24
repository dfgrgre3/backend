package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserGroupType string

const (
	UserGroupGrade   UserGroupType = "GRADE"
	UserGroupClass   UserGroupType = "CLASS"
	UserGroupSpecial UserGroupType = "SPECIAL"
	UserGroupCustom  UserGroupType = "CUSTOM"
)

type UserGroup struct {
	ID          string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name        string         `gorm:"not null;column:name" json:"name"`
	Description *string        `gorm:"type:text;column:description" json:"description"`
	Type        UserGroupType  `gorm:"not null;default:'CUSTOM';column:type" json:"type"`
	IsActive    bool           `gorm:"not null;default:true;index;column:is_active" json:"isActive"`
	CreatedBy   string         `gorm:"not null;type:uuid;column:created_by" json:"createdBy"`
	CreatedAt   time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt   time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	Creator User   `gorm:"foreignKey:CreatedBy;constraint:OnDelete:CASCADE" json:"-"`
	Members []User `gorm:"many2many:user_group_members;constraint:OnDelete:CASCADE" json:"-"`
}

func (UserGroup) TableName() string {
	return "UserGroup"
}

func (g *UserGroup) BeforeCreate(tx *gorm.DB) (err error) {
	if g.ID == "" {
		g.ID = uuid.New().String()
	}
	return
}
