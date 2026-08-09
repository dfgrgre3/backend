package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────────
//  Dashboard Alert Severities & States
// ─────────────────────────────────────────────

const (
	DashboardAlertSeverityInfo     = "info"
	DashboardAlertSeverityWarning  = "warning"
	DashboardAlertSeverityError    = "error"
	DashboardAlertSeverityCritical = "critical"
)

const (
	DashboardAlertStateOpen         = "open"
	DashboardAlertStateAcknowledged = "acknowledged"
	DashboardAlertStateResolved     = "resolved"
	DashboardAlertStateSuppressed   = "suppressed"
)

// DashboardAlertSeverities returns the accepted severity values.
func DashboardAlertSeverities() []string {
	return []string{
		DashboardAlertSeverityInfo,
		DashboardAlertSeverityWarning,
		DashboardAlertSeverityError,
		DashboardAlertSeverityCritical,
	}
}

// DashboardAlertStates returns the accepted lifecycle states.
func DashboardAlertStates() []string {
	return []string{
		DashboardAlertStateOpen,
		DashboardAlertStateAcknowledged,
		DashboardAlertStateResolved,
		DashboardAlertStateSuppressed,
	}
}

// DashboardAlert is a persisted operational alert surfaced on the admin
// dashboard. Repeat occurrences of the same condition are collapsed onto a
// single row via DedupeKey so the widget shows a count instead of duplicates.
type DashboardAlert struct {
	ID                 string     `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Severity           string     `gorm:"not null;default:'info';index:idx_dashboard_alert_severity,priority:1;column:severity" json:"severity"`
	Category           string     `gorm:"not null;index;column:category" json:"category"`
	Title              string     `gorm:"not null;column:title" json:"title"`
	Description        string     `gorm:"type:text;column:description" json:"description"`
	Source             string     `gorm:"not null;default:'system';column:source" json:"source"`
	DedupeKey          *string    `gorm:"column:dedupe_key" json:"-"`
	OccurrenceCount    int64      `gorm:"not null;default:1;column:occurrence_count" json:"occurrenceCount"`
	State              string     `gorm:"not null;default:'open';index:idx_dashboard_alert_state_seen,priority:1;column:state" json:"state"`
	RelatedEntityType  *string    `gorm:"column:related_entity_type" json:"relatedEntityType"`
	RelatedEntityID    *string    `gorm:"column:related_entity_id" json:"relatedEntityId"`
	ActionURL          *string    `gorm:"column:action_url" json:"actionUrl"`
	RequiredPermission *string    `gorm:"column:required_permission" json:"requiredPermission"`
	AcknowledgedBy     *string    `gorm:"type:uuid;column:acknowledged_by" json:"acknowledgedBy"`
	AcknowledgedAt     *time.Time `gorm:"column:acknowledged_at" json:"acknowledgedAt"`
	AcknowledgeNote    *string    `gorm:"type:text;column:acknowledge_note" json:"acknowledgeNote"`
	FirstSeenAt        time.Time  `gorm:"not null;column:first_seen_at" json:"firstSeenAt"`
	LastSeenAt         time.Time  `gorm:"not null;index:idx_dashboard_alert_state_seen,priority:2;column:last_seen_at" json:"lastSeenAt"`
	CreatedAt          time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt          time.Time  `gorm:"column:updated_at" json:"updatedAt"`

	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (DashboardAlert) TableName() string {
	return "DashboardAlert"
}

func (a *DashboardAlert) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	now := time.Now()
	if a.FirstSeenAt.IsZero() {
		a.FirstSeenAt = now
	}
	if a.LastSeenAt.IsZero() {
		a.LastSeenAt = now
	}
	return
}

// ─────────────────────────────────────────────
//  Dashboard Saved Filter
// ─────────────────────────────────────────────

const (
	DashboardFilterVisibilityPrivate = "private"
	DashboardFilterVisibilityShared  = "shared"
)

// DashboardSavedFilter stores a named dashboard filter state for one admin.
type DashboardSavedFilter struct {
	ID          string          `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID      string          `gorm:"not null;type:uuid;index:idx_dashboard_saved_filter_user,priority:1;column:user_id" json:"userId"`
	Name        string          `gorm:"not null;column:name" json:"name"`
	Description *string         `gorm:"type:text;column:description" json:"description"`
	FilterState json.RawMessage `gorm:"type:jsonb;not null;default:'{}';column:filter_state" json:"filterState"`
	IsDefault   bool            `gorm:"not null;default:false;column:is_default" json:"isDefault"`
	Visibility  string          `gorm:"not null;default:'private';column:visibility" json:"visibility"`
	CreatedAt   time.Time       `gorm:"index:idx_dashboard_saved_filter_user,priority:2;column:created_at" json:"createdAt"`
	UpdatedAt   time.Time       `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt  `gorm:"index;column:deleted_at" json:"-"`
}

func (DashboardSavedFilter) TableName() string {
	return "DashboardSavedFilter"
}

func (f *DashboardSavedFilter) BeforeCreate(tx *gorm.DB) (err error) {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	return
}

// ─────────────────────────────────────────────
//  Dashboard Export Job
// ─────────────────────────────────────────────

const (
	DashboardExportStatusPending    = "pending"
	DashboardExportStatusProcessing = "processing"
	DashboardExportStatusCompleted  = "completed"
	DashboardExportStatusFailed     = "failed"
	DashboardExportStatusExpired    = "expired"
)

// DashboardExportJob tracks an asynchronous dashboard export request.
type DashboardExportJob struct {
	ID                    string          `gorm:"primaryKey;type:uuid;column:id" json:"exportJobId"`
	UserID                string          `gorm:"not null;type:uuid;index:idx_dashboard_export_user_created,priority:1;column:user_id" json:"-"`
	ExportScope           string          `gorm:"not null;column:export_scope" json:"exportScope"`
	FileFormat            string          `gorm:"not null;column:file_format" json:"fileFormat"`
	Status                string          `gorm:"not null;default:'pending';index:idx_dashboard_export_status,priority:1;column:status" json:"status"`
	Progress              int             `gorm:"not null;default:0;column:progress" json:"progress"`
	Filters               json.RawMessage `gorm:"type:jsonb;column:filters" json:"filters,omitempty"`
	IncludeChartsMetadata bool            `gorm:"not null;default:false;column:include_charts_metadata" json:"includeChartsMetadata"`
	IncludeSensitive      bool            `gorm:"not null;default:false;column:include_sensitive_fields" json:"includeSensitiveFields"`
	StartDate             *time.Time      `gorm:"column:start_date" json:"startDate,omitempty"`
	EndDate               *time.Time      `gorm:"column:end_date" json:"endDate,omitempty"`
	RowCount              int64           `gorm:"not null;default:0;column:row_count" json:"rowCount"`
	FileURL               *string         `gorm:"column:file_url" json:"fileUrl,omitempty"`
	ErrorCode             *string         `gorm:"column:error_code" json:"errorCode,omitempty"`
	ExpiresAt             *time.Time      `gorm:"column:expires_at" json:"expiresAt,omitempty"`
	CompletedAt           *time.Time      `gorm:"column:completed_at" json:"completedAt,omitempty"`
	CreatedAt             time.Time       `gorm:"index:idx_dashboard_export_user_created,priority:2;column:created_at" json:"createdAt"`
	UpdatedAt             time.Time       `gorm:"column:updated_at" json:"updatedAt"`
}

func (DashboardExportJob) TableName() string {
	return "DashboardExportJob"
}

func (j *DashboardExportJob) BeforeCreate(tx *gorm.DB) (err error) {
	if j.ID == "" {
		j.ID = uuid.New().String()
	}
	return
}

// IsExpired reports whether a completed export's download link is stale.
func (j DashboardExportJob) IsExpired(now time.Time) bool {
	return j.ExpiresAt != nil && now.After(*j.ExpiresAt)
}
