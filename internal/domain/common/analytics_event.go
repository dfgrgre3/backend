package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AnalyticsEvent stores raw analytics events ingested from the frontend.
// Used for Event-Driven Analytics — written by the batch worker (Redis Stream consumer),
// queried by analytics dashboards and aggregated into materialized views.
type AnalyticsEvent struct {
	ID          string     `gorm:"primaryKey;type:uuid" json:"id"`
	EventID     string     `gorm:"uniqueIndex;not null;column:event_id" json:"eventId"`
	EventType   string     `gorm:"not null;index;column:event_type" json:"eventType"`
	UserID      *string    `gorm:"index;column:user_id" json:"userId"`
	Payload     JSONMap    `gorm:"type:jsonb;not null;default:'{}'" json:"payload"`
	Source      string     `gorm:"default:'frontend';column:source" json:"source"`
	IPAddress   *string    `gorm:"column:ip_address" json:"ipAddress"`
	UserAgent   *string    `gorm:"column:user_agent" json:"userAgent"`
	ReceivedAt  time.Time  `gorm:"not null;index;column:received_at" json:"receivedAt"`
	ProcessedAt *time.Time `gorm:"index;column:processed_at" json:"processedAt"`
	CreatedAt   time.Time  `gorm:"column:created_at;<-:create" json:"createdAt"`
}

// BeforeCreate generates a UUID for the primary key if one has not been set.
// GORM's `default:gen_random_uuid()` tag only reads the value back after insert;
// it does NOT prevent GORM from sending the zero-value empty string in the INSERT
// statement, which violates the NOT NULL constraint on the `id` column.
func (a *AnalyticsEvent) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

func (AnalyticsEvent) TableName() string {
	return "AnalyticsEvent"
}
