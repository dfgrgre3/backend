package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ImportExportJob struct {
	ID               string     `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Type             string     `gorm:"not null;column:type" json:"type"` // IMPORT or EXPORT
	Entity           string     `gorm:"not null;column:entity" json:"entity"`
	Status           string     `gorm:"not null;default:'PENDING';index;column:status" json:"status"`
	Progress         int        `gorm:"not null;default:0;column:progress" json:"progress"`
	TotalRecords     int        `gorm:"not null;default:0;column:total_records" json:"totalRecords"`
	ProcessedRecords int        `gorm:"not null;default:0;column:processed_records" json:"processedRecords"`
	ErrorMessage     *string    `gorm:"type:text;column:error_message" json:"errorMessage"`
	FilePath         *string    `gorm:"column:file_path" json:"filePath"`
	CreatedAt        time.Time  `gorm:"index;column:created_at" json:"createdAt"`
	CompletedAt      *time.Time `gorm:"column:completed_at" json:"completedAt"`
}

func (ImportExportJob) TableName() string {
	return "ImportExportJob"
}

func (i *ImportExportJob) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == "" {
		i.ID = uuid.New().String()
	}
	return
}
