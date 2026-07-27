package models

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// ─────────────────────────────────────────────
//  Permission Model
// ─────────────────────────────────────────────

type Permission struct {
	ID          string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name        string    `gorm:"type:varchar(100);unique;not null" json:"name"`
	Module      string    `gorm:"type:varchar(50);not null;index" json:"module"`
	Action      string    `gorm:"type:varchar(50);not null" json:"action"` // create, read, update, delete, manage, approve, publish, export, import
	Description string    `gorm:"type:text" json:"description"`
	CreatedAt   time.Time `gorm:"column:created_at;default:now()" json:"createdAt"`
}

func (Permission) TableName() string {
	return "permissions"
}

// ─────────────────────────────────────────────
//  RolePermission (Many-to-Many)
// ─────────────────────────────────────────────

type RolePermission struct {
	RoleID       string    `gorm:"column:role_id;type:uuid;primaryKey" json:"roleId"`
	PermissionID string    `gorm:"column:permission_id;type:uuid;primaryKey" json:"permissionId"`
	CreatedAt    time.Time `gorm:"column:created_at;default:now()" json:"createdAt"`
}

func (RolePermission) TableName() string {
	return "role_permissions"
}

// ─────────────────────────────────────────────
//  UserRoleMapping (Many-to-Many)
// ─────────────────────────────────────────────

type UserRoleMapping struct {
	UserID     string    `gorm:"column:user_id;type:uuid;primaryKey" json:"userId"`
	RoleID     string    `gorm:"column:role_id;type:uuid;primaryKey" json:"roleId"`
	AssignedAt time.Time `gorm:"column:assigned_at;default:now()" json:"assignedAt"`
	AssignedBy *string   `gorm:"column:assigned_by;type:uuid" json:"assignedBy"`
}

func (UserRoleMapping) TableName() string {
	return "user_roles"
}

// ─────────────────────────────────────────────
//  UserPermission (Direct Override)
// ─────────────────────────────────────────────

type UserPermission struct {
	ID           string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID       string    `gorm:"column:user_id;type:uuid;not null;uniqueIndex:idx_user_permission" json:"userId"`
	PermissionID string    `gorm:"column:permission_id;type:uuid;not null;uniqueIndex:idx_user_permission" json:"permissionId"`
	GrantType    string    `gorm:"type:varchar(10);default:'ALLOW'" json:"grantType"` // ALLOW or DENY
	CreatedAt    time.Time `gorm:"column:created_at;default:now()" json:"createdAt"`
}

func (UserPermission) TableName() string {
	return "user_permissions"
}

// ─────────────────────────────────────────────
//  OAuthAccount
// ─────────────────────────────────────────────

type OAuthAccount struct {
	ID             string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID         string     `gorm:"column:user_id;type:uuid;not null" json:"userId"`
	Provider       string     `gorm:"type:varchar(50);not null;uniqueIndex:idx_provider_user" json:"provider"`
	ProviderUserID string     `gorm:"column:provider_user_id;type:varchar(255);not null;uniqueIndex:idx_provider_user" json:"providerUserId"`
	Email          *string    `gorm:"type:varchar(255)" json:"email"`
	AvatarURL      *string    `gorm:"column:avatar_url;type:text" json:"avatarUrl"`
	Name           *string    `gorm:"type:varchar(150)" json:"name"`
	AccessToken    *string    `gorm:"column:access_token;type:text" json:"-"`
	RefreshToken   *string    `gorm:"column:refresh_token;type:text" json:"-"`
	ExpiresAt      *time.Time `gorm:"column:expires_at" json:"expiresAt"`
	RawAttributes  *string    `gorm:"column:raw_attributes;type:jsonb" json:"-"`
	CreatedAt      time.Time  `gorm:"column:created_at;default:now()" json:"createdAt"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;default:now()" json:"updatedAt"`
}

func (OAuthAccount) TableName() string {
	return "oauth_accounts"
}

// ─────────────────────────────────────────────
//  VerificationCode
// ─────────────────────────────────────────────

type VerificationCode struct {
	ID              string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID          string     `gorm:"column:user_id;type:uuid;not null;index" json:"userId"`
	Code            string     `gorm:"type:varchar(100);not null" json:"code"`
	Type            string     `gorm:"type:varchar(50);not null;index" json:"type"` // email_verification, phone_verification, mfa, password_reset
	ExpiresAt       time.Time  `gorm:"column:expires_at;not null" json:"expiresAt"`
	IsUsed          bool       `gorm:"column:is_used;default:false;index" json:"isUsed"`
	MaxAttempts     int        `gorm:"column:max_attempts;default:5" json:"maxAttempts"`
	AttemptCount    int        `gorm:"column:attempt_count;default:0" json:"attemptCount"`
	CreatedAt       time.Time  `gorm:"column:created_at;default:now()" json:"createdAt"`
	LastAttemptedAt *time.Time `gorm:"column:last_attempted_at" json:"-"`
}

func (VerificationCode) TableName() string {
	return "verification_codes"
}

// ─────────────────────────────────────────────
//  LoginHistory
// ─────────────────────────────────────────────

type LoginHistory struct {
	ID                string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID            string    `gorm:"column:user_id;type:uuid;not null;index" json:"userId"`
	IP                *string   `gorm:"type:varchar(45)" json:"ip"`
	UserAgent         *string   `gorm:"column:user_agent;type:text" json:"userAgent"`
	Status            string    `gorm:"type:varchar(20);not null" json:"status"` // SUCCESS, FAILED, MFA_REQUIRED, BLOCKED
	Reason            *string   `gorm:"type:varchar(255)" json:"reason"`
	Country           *string   `gorm:"type:varchar(100)" json:"country"`
	City              *string   `gorm:"type:varchar(100)" json:"city"`
	Region            *string   `gorm:"type:varchar(100)" json:"region"`
	ISP               *string   `gorm:"type:varchar(100)" json:"isp"`
	DeviceFingerprint *string   `gorm:"column:device_fingerprint;type:varchar(64)" json:"deviceFingerprint"`
	Suspicious        bool      `gorm:"default:false" json:"suspicious"`
	MFAUsed           bool      `gorm:"column:mfa_used;default:false" json:"mfaUsed"`
	CreatedAt         time.Time `gorm:"column:created_at;default:now();index" json:"createdAt"`
}

func (LoginHistory) TableName() string {
	return "login_history"
}

// ─────────────────────────────────────────────
//  Profile
// ─────────────────────────────────────────────

type Profile struct {
	ID              string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID          string     `gorm:"column:user_id;type:uuid;unique;not null" json:"userId"`
	Name            string     `gorm:"type:varchar(150);not null" json:"name"`
	AvatarURL       *string    `gorm:"column:avatar_url;type:varchar(512)" json:"avatarUrl"`
	DateOfBirth     *time.Time `gorm:"column:date_of_birth;type:date" json:"dateOfBirth"`
	Gender          *string    `gorm:"type:varchar(20)" json:"gender"`
	Bio             *string    `gorm:"type:text" json:"bio"`
	Language        string     `gorm:"type:varchar(10);default:'ar'" json:"language"`
	Timezone        string     `gorm:"type:varchar(50);default:'Africa/Cairo'" json:"timezone"`
	TwoFactorMethod string     `gorm:"column:two_factor_method;type:varchar(20);default:'none'" json:"twoFactorMethod"` // app, sms, email, none
	School          *string    `gorm:"type:varchar(150)" json:"school"`
	Section         *string    `gorm:"type:varchar(50)" json:"section"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;default:now()" json:"updatedAt"`
}

func (Profile) TableName() string {
	return "profiles"
}

// ─────────────────────────────────────────────
//  PasswordResetToken
// ─────────────────────────────────────────────

type PasswordResetToken struct {
	ID        string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID    string    `gorm:"column:user_id;type:uuid;not null;index" json:"userId"`
	TokenHash string    `gorm:"column:token_hash;type:varchar(64);unique;not null;index" json:"tokenHash"`
	IsUsed    bool      `gorm:"column:is_used;default:false;index" json:"isUsed"`
	ExpiresAt time.Time `gorm:"column:expires_at;not null" json:"expiresAt"`
	CreatedAt time.Time `gorm:"column:created_at;default:now()" json:"createdAt"`
}

func (PasswordResetToken) TableName() string {
	return "password_reset_tokens"
}

// ─────────────────────────────────────────────
//  RefreshTokenArchive
// ─────────────────────────────────────────────

type RefreshTokenArchive struct {
	ID        string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SessionID string    `gorm:"column:session_id;type:uuid;not null;index" json:"sessionId"`
	TokenHash string    `gorm:"column:token_hash;type:varchar(64);unique;not null" json:"tokenHash"`
	IsUsed    bool      `gorm:"column:is_used;default:false" json:"isUsed"`
	ExpiresAt time.Time `gorm:"column:expires_at;not null" json:"expiresAt"`
	CreatedAt time.Time `gorm:"column:created_at;default:now()" json:"createdAt"`
}

func (RefreshTokenArchive) TableName() string {
	return "refresh_tokens"
}

// ─────────────────────────────────────────────
//  StudentParent (Relationship)
// ─────────────────────────────────────────────

type StudentParent struct {
	StudentID string    `gorm:"column:student_id;type:uuid;primaryKey" json:"studentId"`
	ParentID  string    `gorm:"column:parent_id;type:uuid;primaryKey" json:"parentId"`
	CreatedAt time.Time `gorm:"column:created_at;default:now()" json:"createdAt"`
}

func (StudentParent) TableName() string {
	return "student_parents"
}

// ─────────────────────────────────────────────
//  TrustedDevice
// ─────────────────────────────────────────────

type TrustedDevice struct {
	ID              string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID          string    `gorm:"column:user_id;type:uuid;not null;uniqueIndex:idx_trusted_device" json:"userId"`
	DeviceName      *string   `gorm:"column:device_name;type:varchar(255)" json:"deviceName"`
	DeviceType      *string   `gorm:"column:device_type;type:varchar(50)" json:"deviceType"` // mobile, tablet, desktop
	FingerprintHash string    `gorm:"column:fingerprint_hash;type:varchar(64);not null;uniqueIndex:idx_trusted_device" json:"-"`
	UserAgent       *string   `gorm:"column:user_agent;type:text" json:"-"`
	IPAddress       *string   `gorm:"column:ip_address;type:varchar(45)" json:"-"`
	TrustedAt       time.Time `gorm:"column:trusted_at;default:now()" json:"trustedAt"`
	ExpiresAt       time.Time `gorm:"column:expires_at;not null" json:"expiresAt"`
	LastUsedAt      time.Time `gorm:"column:last_used_at;default:now()" json:"lastUsedAt"`
	IsActive        bool      `gorm:"column:is_active;default:true" json:"isActive"`
	CreatedAt       time.Time `gorm:"column:created_at;default:now()" json:"createdAt"`
	UpdatedAt       time.Time `gorm:"column:updated_at;default:now()" json:"updatedAt"`
}

func (TrustedDevice) TableName() string {
	return "trusted_devices"
}

// ─────────────────────────────────────────────
//  SecurityEvent
// ─────────────────────────────────────────────

type SecurityEvent struct {
	ID              string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID          *string    `gorm:"column:user_id;type:uuid;index" json:"userId"`
	EventType       string     `gorm:"column:event_type;type:varchar(50);not null;index" json:"eventType"`
	Severity        string     `gorm:"type:varchar(20);default:'info';index" json:"severity"` // info, warning, critical
	IPAddress       *string    `gorm:"column:ip_address;type:varchar(45)" json:"ipAddress"`
	UserAgent       *string    `gorm:"column:user_agent;type:text" json:"userAgent"`
	FingerprintHash *string    `gorm:"column:fingerprint_hash;type:varchar(64)" json:"-"`
	Location        *string    `gorm:"type:jsonb" json:"-"`
	Metadata        string     `gorm:"type:jsonb;default:'{}'" json:"-"`
	Resolved        bool       `gorm:"default:false;index" json:"resolved"`
	ResolvedBy      *string    `gorm:"column:resolved_by;type:uuid" json:"resolvedBy"`
	ResolvedAt      *time.Time `gorm:"column:resolved_at" json:"resolvedAt"`
	CreatedAt       time.Time  `gorm:"column:created_at;default:now();index" json:"createdAt"`
}

func (SecurityEvent) TableName() string {
	return "security_events"
}

// ─────────────────────────────────────────────
//  BlockedToken (Persistent JTI Blacklist)
// ─────────────────────────────────────────────

type BlockedToken struct {
	ID        string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	JTI       string    `gorm:"type:varchar(64);unique;not null;index" json:"jti"`
	UserID    *string   `gorm:"column:user_id;type:uuid" json:"userId"`
	Reason    string    `gorm:"type:varchar(50);not null" json:"reason"` // logout, revoke, rotation, security
	ExpiresAt time.Time `gorm:"column:expires_at;not null;index" json:"expiresAt"`
	CreatedAt time.Time `gorm:"column:created_at;default:now()" json:"createdAt"`
}

func (BlockedToken) TableName() string {
	return "blocked_tokens"
}

// ─────────────────────────────────────────────
//  BlacklistedIP
// ─────────────────────────────────────────────

type BlacklistedIP struct {
	ID          string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	IPAddress   string     `gorm:"column:ip_address;type:varchar(45);not null;index" json:"ipAddress"`
	Reason      *string    `gorm:"type:varchar(255)" json:"reason"`
	BlockedBy   *string    `gorm:"column:blocked_by;type:uuid" json:"blockedBy"`
	BlockedAt   time.Time  `gorm:"column:blocked_at;default:now()" json:"blockedAt"`
	ExpiresAt   *time.Time `gorm:"column:expires_at;index" json:"expiresAt"`
	IsPermanent bool       `gorm:"column:is_permanent;default:false" json:"isPermanent"`
}

func (BlacklistedIP) TableName() string {
	return "blacklisted_ips"
}

// ─────────────────────────────────────────────
//  BackupCode (MFA Recovery)
// ─────────────────────────────────────────────

type BackupCode struct {
	ID        string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID    string     `gorm:"column:user_id;type:uuid;not null;uniqueIndex:idx_backup_code" json:"userId"`
	CodeHash  string     `gorm:"column:code_hash;type:varchar(64);not null;uniqueIndex:idx_backup_code" json:"-"`
	IsUsed    bool       `gorm:"column:is_used;default:false" json:"isUsed"`
	UsedAt    *time.Time `gorm:"column:used_at" json:"usedAt"`
	CreatedAt time.Time  `gorm:"column:created_at;default:now()" json:"createdAt"`
}

func (BackupCode) TableName() string {
	return "backup_codes"
}

// ─────────────────────────────────────────────
//  TwoFactorSecret
// ─────────────────────────────────────────────

type TwoFactorSecret struct {
	ID           string     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID       string     `gorm:"column:user_id;type:uuid;unique;not null" json:"userId"`
	Secret       string     `gorm:"type:text;not null" json:"-"`
	Method       string     `gorm:"type:varchar(20);default:'app'" json:"method"` // app, sms, email
	PhoneNumber  *string    `gorm:"column:phone_number;type:varchar(20)" json:"-"`
	IsEnabled    bool       `gorm:"column:is_enabled;default:false" json:"isEnabled"`
	EnabledAt    *time.Time `gorm:"column:enabled_at" json:"enabledAt"`
	LastVerified *time.Time `gorm:"column:last_verified" json:"lastVerified"`
	CreatedAt    time.Time  `gorm:"column:created_at;default:now()" json:"createdAt"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;default:now()" json:"updatedAt"`
}

func (TwoFactorSecret) TableName() string {
	return "two_factor_secrets"
}

// ─────────────────────────────────────────────
//  PasswordHistory
// ─────────────────────────────────────────────

type PasswordHistory struct {
	ID           string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID       string    `gorm:"column:user_id;type:uuid;not null;index" json:"userId"`
	PasswordHash string    `gorm:"column:password_hash;type:text;not null" json:"-"`
	CreatedAt    time.Time `gorm:"column:created_at;default:now()" json:"createdAt"`
}

func (PasswordHistory) TableName() string {
	return "password_history"
}

// ─────────────────────────────────────────────
//  Helpers
// ─────────────────────────────────────────────

// HashToken computes SHA-256 hash for secure token storage
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
