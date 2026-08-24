package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type JobStatus string

const (
	JobStatusPending    JobStatus = "PENDING"
	JobStatusProcessing JobStatus = "PROCESSING"
	JobStatusCompleted  JobStatus = "COMPLETED"
	JobStatusFailed     JobStatus = "FAILED"
	JobStatusRetrying   JobStatus = "RETRYING"
)

type QueueJob struct {
	ID          string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name        string         `gorm:"not null;column:name" json:"name"`
	Type        string         `gorm:"not null;column:type" json:"type"`
	Status      JobStatus      `gorm:"not null;default:'PENDING';index;column:status" json:"status"`
	Priority    int            `gorm:"not null;default:0;column:priority" json:"priority"`
	Attempts    int            `gorm:"not null;default:0;column:attempts" json:"attempts"`
	MaxAttempts int            `gorm:"not null;default:3;column:max_attempts" json:"maxAttempts"`
	Error       *string        `gorm:"type:text;column:error" json:"error"`
	Payload     []byte         `gorm:"type:jsonb;column:payload" json:"-"`
	ProcessedAt *time.Time     `gorm:"column:processed_at" json:"processedAt"`
	CreatedAt   time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt   time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (QueueJob) TableName() string {
	return "QueueJob"
}

func (j *QueueJob) BeforeCreate(tx *gorm.DB) (err error) {
	if j.ID == "" {
		j.ID = uuid.New().String()
	}
	return
}
