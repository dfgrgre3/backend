package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

// ─────────────────────────────────────────────
//  Instructor Payouts
// ─────────────────────────────────────────────

type PayoutStatus string

const (
	PayoutPending  PayoutStatus = "PENDING"
	PayoutApproved PayoutStatus = "APPROVED"
	PayoutPaid     PayoutStatus = "PAID"
	PayoutRejected PayoutStatus = "REJECTED"
	PayoutFailed   PayoutStatus = "FAILED"
)

type InstructorPayout struct {
	ID            string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	InstructorID  string         `gorm:"not null;index;type:uuid;column:instructor_id" json:"instructorId"`
	Amount        float64        `gorm:"not null;check:amount >= 0;column:amount" json:"amount"`
	Currency      string         `gorm:"not null;default:'EGP';column:currency" json:"currency"`
	Status        PayoutStatus   `gorm:"not null;default:'PENDING';index;column:status" json:"status"`
	PaymentMethod string         `gorm:"column:payment_method" json:"paymentMethod"`
	TransactionID *string        `gorm:"column:transaction_id" json:"transactionId"`
	Notes         *string        `gorm:"type:text;column:notes" json:"notes"`
	ApprovedBy    *string        `gorm:"type:uuid;column:approved_by" json:"approvedBy"`
	ApprovedAt    *time.Time     `gorm:"column:approved_at" json:"approvedAt"`
	PaidAt        *time.Time     `gorm:"column:paid_at" json:"paidAt"`
	CreatedAt     time.Time      `gorm:"index;column:created_at" json:"requestedAt"`
	UpdatedAt     time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	Instructor User `gorm:"foreignKey:InstructorID;constraint:OnDelete:CASCADE" json:"-"`
}

func (InstructorPayout) TableName() string {
	return "InstructorPayout"
}

func (p *InstructorPayout) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return
}

// ─────────────────────────────────────────────
//  Roles & Permissions
// ─────────────────────────────────────────────

type Role struct {
	ID          string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name        string         `gorm:"not null;uniqueIndex;column:name" json:"name"`
	Description *string        `gorm:"type:text;column:description" json:"description"`
	Permissions []byte         `gorm:"type:jsonb;column:permissions" json:"permissions"` // JSON array of permissions
	IsSystem    bool           `gorm:"not null;default:false;index;column:is_system" json:"isSystem"`
	CreatedAt   time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	Users []User `gorm:"foreignKey:RoleID;constraint:OnDelete:SET NULL" json:"-"`
}

func (Role) TableName() string {
	return "Role"
}

func (r *Role) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return
}

// ─────────────────────────────────────────────
//  User Groups
// ─────────────────────────────────────────────

type UserGroupType string

const (
	UserGroupGrade   UserGroupType = "GRADE"
	UserGroupClass   UserGroupType = "CLASS"
	UserGroupSpecial UserGroupType = "SPECIAL"
	UserGroupCustom  UserGroupType = "CUSTOM"
)

type UserGroup struct {
	ID          string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name        string         `gorm:"not null;column:name" json:"name"`
	Description *string        `gorm:"type:text;column:description" json:"description"`
	Type        UserGroupType  `gorm:"not null;default:'CUSTOM';column:type" json:"type"`
	IsActive    bool           `gorm:"not null;default:true;index;column:is_active" json:"isActive"`
	CreatedBy   string         `gorm:"not null;type:uuid;column:created_by" json:"createdBy"`
	CreatedAt   time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt   time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	Creator User   `gorm:"foreignKey:CreatedBy;constraint:OnDelete:CASCADE" json:"-"`
	Members []User `gorm:"many2many:user_group_members;constraint:OnDelete:CASCADE" json:"-"`
}

func (UserGroup) TableName() string {
	return "UserGroup"
}

func (g *UserGroup) BeforeCreate(tx *gorm.DB) (err error) {
	if g.ID == "" {
		g.ID = uuid.New().String()
	}
	return
}

// ─────────────────────────────────────────────
//  MFA (Multi-Factor Authentication)
// ─────────────────────────────────────────────

type MFAMethod string

const (
	MFAMethodTOTP  MFAMethod = "TOTP"
	MFAMethodSMS   MFAMethod = "SMS"
	MFAMethodEmail MFAMethod = "EMAIL"
)

type MFA struct {
	ID              string     `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID          string     `gorm:"not null;uniqueIndex;type:uuid;column:user_id" json:"userId"`
	IsEnabled       bool       `gorm:"not null;default:false;column:is_enabled" json:"isEnabled"`
	Method          *MFAMethod `gorm:"column:method" json:"method"`
	Secret          *string    `gorm:"column:secret" json:"-"`
	BackupCodes     *[]byte    `gorm:"type:jsonb;column:backup_codes" json:"-"`
	BackupCodesUsed int        `gorm:"not null;default:0;column:backup_codes_used" json:"backupCodesUsed"`
	LastUsedAt      *time.Time `gorm:"column:last_used_at" json:"lastUsedAt"`
	CreatedAt       time.Time  `gorm:"column:created_at" json:"enrolledAt"`
	UpdatedAt       time.Time  `gorm:"column:updated_at" json:"updatedAt"`

	// Relations
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

func (MFA) TableName() string {
	return "MFA"
}

func (m *MFA) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return
}

// ─────────────────────────────────────────────
//  Scheduled Tasks
// ─────────────────────────────────────────────

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

// ─────────────────────────────────────────────
//  Queue Jobs
// ─────────────────────────────────────────────

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

// ─────────────────────────────────────────────
//  Cache Entries
// ─────────────────────────────────────────────

type CacheEntry struct {
	Key          string     `gorm:"primaryKey;column:key" json:"key"`
	Type         string     `gorm:"not null;column:type" json:"type"`
	Value        []byte     `gorm:"type:bytea;column:value" json:"-"`
	Size         int64      `gorm:"not null;column:size" json:"size"`
	Hits         int64      `gorm:"not null;default:0;column:hits" json:"hits"`
	Misses       int64      `gorm:"not null;default:0;column:misses" json:"misses"`
	LastAccessed time.Time  `gorm:"not null;index;column:last_accessed" json:"lastAccessed"`
	ExpiresAt    *time.Time `gorm:"index;column:expires_at" json:"expiresAt"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

func (CacheEntry) TableName() string {
	return "CacheEntry"
}

// ─────────────────────────────────────────────
//  Email Templates
// ─────────────────────────────────────────────

type EmailTemplate struct {
	ID         string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name       string         `gorm:"not null;column:name" json:"name"`
	Subject    string         `gorm:"not null;column:subject" json:"subject"`
	Body       string         `gorm:"type:text;not null;column:body" json:"body"`
	Category   string         `gorm:"not null;index;column:category" json:"category"`
	Variables  []byte         `gorm:"type:jsonb;column:variables" json:"variables"` // JSON array of variable names
	IsActive   bool           `gorm:"not null;default:true;index;column:is_active" json:"isActive"`
	LastUsedAt *time.Time     `gorm:"column:last_used_at" json:"lastUsedAt"`
	CreatedAt  time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt  time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt  gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (EmailTemplate) TableName() string {
	return "EmailTemplate"
}

func (e *EmailTemplate) BeforeCreate(tx *gorm.DB) (err error) {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	return
}

// ─────────────────────────────────────────────
//  Feature Flags
// ─────────────────────────────────────────────

type FeatureFlag struct {
	ID                string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name              string         `gorm:"not null;column:name" json:"name"`
	Key               string         `gorm:"not null;uniqueIndex;column:key" json:"key"`
	Description       *string        `gorm:"type:text;column:description" json:"description"`
	IsEnabled         bool           `gorm:"not null;default:false;index;column:is_enabled" json:"isEnabled"`
	Environment       string         `gorm:"not null;default:'production';column:environment" json:"environment"`
	RolloutPercentage int            `gorm:"not null;default:0;column:rollout_percentage" json:"rolloutPercentage"`
	AllowedUsers      []byte         `gorm:"type:jsonb;column:allowed_users" json:"allowedUsers"` // JSON array of user IDs
	CreatedAt         time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt         time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt         gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (FeatureFlag) TableName() string {
	return "FeatureFlag"
}

func (f *FeatureFlag) BeforeCreate(tx *gorm.DB) (err error) {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	return
}

// ─────────────────────────────────────────────
//  API Keys
// ─────────────────────────────────────────────

type APIKey struct {
	ID          string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name        string         `gorm:"not null;column:name" json:"name"`
	Key         string         `gorm:"not null;uniqueIndex;column:key" json:"key"`
	Permissions []byte         `gorm:"type:jsonb;column:permissions" json:"permissions"` // JSON array
	LastUsedAt  *time.Time     `gorm:"column:last_used_at" json:"lastUsedAt"`
	ExpiresAt   *time.Time     `gorm:"index;column:expires_at" json:"expiresAt"`
	IsActive    bool           `gorm:"not null;default:true;index;column:is_active" json:"isActive"`
	CreatedAt   time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt   time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (APIKey) TableName() string {
	return "APIKey"
}

func (a *APIKey) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return
}

// ─────────────────────────────────────────────
//  Webhooks
// ─────────────────────────────────────────────

type Webhook struct {
	ID              string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name            string         `gorm:"not null;column:name" json:"name"`
	URL             string         `gorm:"not null;column:url" json:"url"`
	Events          []byte         `gorm:"type:jsonb;not null;column:events" json:"events"` // JSON array of event names
	IsActive        bool           `gorm:"not null;default:true;index;column:is_active" json:"isActive"`
	Secret          *string        `gorm:"column:secret" json:"-"`
	LastTriggeredAt *time.Time     `gorm:"column:last_triggered_at" json:"lastTriggeredAt"`
	SuccessCount    int            `gorm:"not null;default:0;column:success_count" json:"successCount"`
	FailureCount    int            `gorm:"not null;default:0;column:failure_count" json:"failureCount"`
	CreatedAt       time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt       time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (Webhook) TableName() string {
	return "Webhook"
}

func (w *Webhook) BeforeCreate(tx *gorm.DB) (err error) {
	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	return
}

// ─────────────────────────────────────────────
//  Import/Export Jobs
// ─────────────────────────────────────────────

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

// ─────────────────────────────────────────────
//  System Logs
// ─────────────────────────────────────────────

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

// ─────────────────────────────────────────────
//  Activity Log
// ─────────────────────────────────────────────

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

// ─────────────────────────────────────────────
//  Login Attempts
// ─────────────────────────────────────────────

type LoginAttemptStatus string

const (
	LoginAttemptSuccess LoginAttemptStatus = "SUCCESS"
	LoginAttemptFailed  LoginAttemptStatus = "FAILED"
	LoginAttemptBlocked LoginAttemptStatus = "BLOCKED"
)

type LoginAttempt struct {
	ID            string             `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID        *string            `gorm:"type:uuid;index;column:user_id" json:"userId"`
	IPAddress     string             `gorm:"not null;index;column:ip_address" json:"ipAddress"`
	UserAgent     string             `gorm:"type:text;column:user_agent" json:"userAgent"`
	Status        LoginAttemptStatus `gorm:"not null;index;column:status" json:"status"`
	FailureReason *string            `gorm:"type:text;column:failure_reason" json:"failureReason"`
	Location      *string            `gorm:"column:location" json:"location"`
	RiskScore     int                `gorm:"not null;default:0;column:risk_score" json:"riskScore"`
	CreatedAt     time.Time          `gorm:"not null;index;column:created_at" json:"createdAt"`

	// Relations
	User *User `gorm:"foreignKey:UserID;constraint:OnDelete:SET NULL" json:"-"`
}

func (LoginAttempt) TableName() string {
	return "LoginAttempt"
}

func (l *LoginAttempt) BeforeCreate(tx *gorm.DB) (err error) {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return
}
