package protected

const errBackupNotFound = "Backup not found"

// CreateBackupRequest represents a request to create a backup
type CreateBackupRequest struct {
	Name             string   `json:"name" binding:"required,max=100"`
	Type             string   `json:"type" binding:"required,oneof=full database files incremental"`
	Tables           []string `json:"tables,omitempty"`
	IncludesFiles    bool     `json:"includesFiles"`
	IncludesDatabase bool     `json:"includesDatabase"`
	RetentionDays    int      `json:"retentionDays" binding:"omitempty,min=1,max=365"`
}

// RestoreBackupRequest represents a restore request
type RestoreBackupRequest struct {
	TargetTables []string `json:"targetTables,omitempty"`
	SkipExisting bool     `json:"skipExisting"`
	DryRun       bool     `json:"dryRun"`
}

// ScheduleBackupRequest represents a scheduled backup configuration
type ScheduleBackupRequest struct {
	Frequency     string `json:"frequency" binding:"required,oneof=daily weekly monthly"`
	Type          string `json:"type" binding:"required,oneof=full database files incremental"`
	Time          string `json:"time" binding:"required,datetime=15:04"`
	DayOfWeek     int    `json:"dayOfWeek,omitempty" binding:"omitempty,min=0,max=6"`
	DayOfMonth    int    `json:"dayOfMonth,omitempty" binding:"omitempty,min=1,max=31"`
	RetentionDays int    `json:"retentionDays" binding:"required,min=1,max=365"`
}
