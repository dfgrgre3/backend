package models

import (
	"database/sql/driver"
	"encoding/json"
	"strings"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Broadcast represents a mass notification broadcast
type Broadcast struct {
	ID           string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Title        string         `gorm:"size:200;not null" json:"title"`
	Message      string         `gorm:"type:text;not null" json:"message"`
	Type         string         `gorm:"size:20;default:'info'" json:"type"` // info, success, warning, error
	Channels     StringArray    `gorm:"type:jsonb" json:"channels"`         // in-app, email, sms, push
	TargetCount  int            `json:"targetCount"`
	SuccessCount int            `json:"successCount"`
	FailureCount int            `json:"failureCount"`
	Status       string         `gorm:"size:20;default:'draft'" json:"status"` // draft, scheduled, sending, completed, cancelled
	ScheduledFor *time.Time     `json:"scheduledFor,omitempty"`
	SentAt       *time.Time     `json:"sentAt,omitempty"`
	CreatedBy    string         `gorm:"type:uuid;not null" json:"createdBy"`
	CancelledBy  *string        `gorm:"type:uuid" json:"cancelledBy,omitempty"`
	CancelledAt  *time.Time     `json:"cancelledAt,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// PushToken represents a user's push notification token (FCM, APNs)
type PushToken struct {
	ID        string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID    string         `gorm:"type:uuid;not null;index" json:"userId"`
	Token     string         `gorm:"size:500;not null;uniqueIndex" json:"token"`
	Platform  string         `gorm:"size:20;not null" json:"platform"` // ios, android, web
	Provider  string         `gorm:"size:20;not null" json:"provider"` // fcm, apns
	IsActive  bool           `gorm:"default:true" json:"isActive"`
	LastUsed  *time.Time     `json:"lastUsed,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	User User `gorm:"foreignKey:UserID" json:"-"`
}

// StringArray is a custom type for storing string arrays in JSONB
type StringArray []string

// UnmarshalJSON implements the json.Unmarshaler interface
func (a *StringArray) UnmarshalJSON(data []byte) error {
	// First, try to unmarshal as standard array of strings
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*a = arr
		return nil
	}

	// If that fails, it might be a JSON string
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	// The string itself might be a JSON-encoded array
	var nestedArr []string
	if err := json.Unmarshal([]byte(str), &nestedArr); err == nil {
		*a = nestedArr
		return nil
	}

	// Otherwise, treat it as a multiline or comma-separated string
	if str == "" {
		*a = []string{}
		return nil
	}

	lines := strings.Split(str, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	*a = result
	return nil
}

// Value implements the driver.Valuer interface
func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	return json.Marshal(a)
}

// Scan implements the sql.Scanner interface
func (a *StringArray) Scan(value interface{}) error {
	if value == nil {
		*a = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}

	return json.Unmarshal(bytes, a)
}

// PGStringArray is a custom type for storing string arrays in PostgreSQL text[]
type PGStringArray []string

// UnmarshalJSON implements the json.Unmarshaler interface
func (a *PGStringArray) UnmarshalJSON(data []byte) error {
	// First, try to unmarshal as standard array of strings
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*a = arr
		return nil
	}

	// If that fails, it might be a JSON string
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	// The string itself might be a JSON-encoded array
	var nestedArr []string
	if err := json.Unmarshal([]byte(str), &nestedArr); err == nil {
		*a = nestedArr
		return nil
	}

	// Otherwise, treat it as a multiline or comma-separated string
	if str == "" {
		*a = []string{}
		return nil
	}

	lines := strings.Split(str, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	*a = result
	return nil
}

// Value implements the driver.Valuer interface using pq.StringArray
func (a PGStringArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	return pq.StringArray(a).Value()
}

// Scan implements the sql.Scanner interface using pq.StringArray
func (a *PGStringArray) Scan(value interface{}) error {
	var pqArr pq.StringArray
	if err := pqArr.Scan(value); err != nil {
		return err
	}
	*a = PGStringArray(pqArr)
	return nil
}

// JSONMap is a custom type for storing JSON objects
type JSONMap map[string]interface{}

// Value implements the driver.Valuer interface
func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

// Scan implements the sql.Scanner interface
func (m *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*m = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}

	return json.Unmarshal(bytes, m)
}

// TableName returns the table name for Broadcast
func (Broadcast) TableName() string {
	return "broadcasts"
}

// TableName returns the table name for PushToken
func (PushToken) TableName() string {
	return "push_tokens"
}
