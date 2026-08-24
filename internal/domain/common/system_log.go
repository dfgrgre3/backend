package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LogLevel string

const (
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
	LogLevelDebug LogLevel = "DEBUG"
)

type SystemLog struct {
	ID        string    `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Level     LogLevel  `gorm:"not null;index;column:level" json:"level"`
	Service   string    `gorm:"not null;index;column:service" json:"service"`
	Message   string    `gorm:"not null;type:text;column:message" json:"message"`
	UserID    *string   `gorm:"type:uuid;index;column:user_id" json:"userId"`
	IPAddress *string   `gorm:"column:ip_address" json:"ipAddress"`
	Metadata  []byte    `gorm:"type:jsonb;column:metadata" json:"metadata"`
	CreatedAt time.Time `gorm:"not null;index:idx_system_logs_created,priority:1;column:created_at" json:"createdAt"`
}

func (SystemLog) TableName() string {
	return "SystemLog"
}

func (s *SystemLog) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return
}
