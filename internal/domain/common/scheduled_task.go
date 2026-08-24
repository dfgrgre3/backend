package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TaskType string

const (
	TaskTypeEmail        TaskType = "EMAIL"
	TaskTypeNotification TaskType = "NOTIFICATION"
	TaskTypeReport       TaskType = "REPORT"
	TaskTypeBackup       TaskType = "BACKUP"
	TaskTypeCustom       TaskType = "CUSTOM"
)

type ScheduledTask struct {
	ID           string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name         string         `gorm:"not null;column:name" json:"name"`
	Description  *string        `gorm:"type:text;column:description" json:"description"`
	Type         TaskType       `gorm:"not null;column:type" json:"type"`
	Status       TaskStatus     `gorm:"not null;default:'ACTIVE';index;column:status" json:"status"`
	Schedule     string         `gorm:"not null;column:schedule" json:"schedule"` // Cron expression
	LastRunAt    *time.Time     `gorm:"column:last_run_at" json:"lastRunAt"`
	NextRunAt    *time.Time     `gorm:"column:next_run_at" json:"nextRunAt"`
	RunCount     int            `gorm:"not null;default:0;column:run_count" json:"runCount"`
	SuccessCount int            `gorm:"not null;default:0;column:success_count" json:"successCount"`
	FailureCount int            `gorm:"not null;default:0;column:failure_count" json:"failureCount"`
	CreatedAt    time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt    time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (ScheduledTask) TableName() string {
	return "ScheduledTask"
}

func (t *ScheduledTask) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return
}
