package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ActivityLog struct {
	ID         string    `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID     string    `gorm:"not null;type:uuid;index:idx_activity_log_user_created,priority:1;column:user_id" json:"userId"`
	Action     string    `gorm:"not null;column:action" json:"action"`
	Resource   string    `gorm:"not null;column:resource" json:"resource"`
	ResourceID *string   `gorm:"type:uuid;column:resource_id" json:"resourceId"`
	IPAddress  string    `gorm:"not null;column:ip_address" json:"ipAddress"`
	UserAgent  string    `gorm:"type:text;column:user_agent" json:"userAgent"`
	Metadata   []byte    `gorm:"type:jsonb;column:metadata" json:"metadata"`
	CreatedAt  time.Time `gorm:"not null;index:idx_activity_log_user_created,priority:2;column:created_at" json:"createdAt"`

	// Relations
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

func (ActivityLog) TableName() string {
	return "ActivityLog"
}

func (a *ActivityLog) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return
}
