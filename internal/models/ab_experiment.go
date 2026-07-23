package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ABExperiment represents an A/B testing experiment
type ABExperiment struct {
	ID          string     `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name        string     `gorm:"not null;column:name" json:"name"`
	Description string     `gorm:"column:description" json:"description"`
	Status      string     `gorm:"default:'DRAFT';column:status" json:"status"`
	Variants    string     `gorm:"type:text;column:variants" json:"variants"`
	TrafficPct  int        `gorm:"default:100;column:traffic_pct" json:"trafficPct"`
	Winner      *string    `gorm:"column:winner" json:"winner"`
	StartDate   *time.Time `gorm:"column:start_date" json:"startDate"`
	EndDate     *time.Time `gorm:"column:end_date" json:"endDate"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

func (ABExperiment) TableName() string { return "ABExperiment" }
func (a *ABExperiment) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

// Variant represents a single variant within an experiment's JSON variants field
type Variant struct {
	Name           string  `json:"name"`
	Views          int     `json:"views"`
	CompletionRate float64 `json:"completionRate"`
	AvgScore       float64 `json:"avgScore,omitempty"`
}