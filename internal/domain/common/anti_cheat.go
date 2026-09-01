package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AntiCheatEventType enumerates the proctoring signals the exam client can
// report (tab switch, blur, copy/paste, ...).
type AntiCheatEventType string

const (
	AntiCheatTabSwitch      AntiCheatEventType = "TAB_SWITCH"
	AntiCheatWindowBlur     AntiCheatEventType = "WINDOW_BLUR"
	AntiCheatCopyPaste      AntiCheatEventType = "COPY_PASTE"
	AntiCheatScreenshot     AntiCheatEventType = "SCREENSHOT"
	AntiCheatRightClick     AntiCheatEventType = "RIGHT_CLICK"
	AntiCheatFullscreenExit AntiCheatEventType = "FULLSCREEN_EXIT"
	AntiCheatCameraOff      AntiCheatEventType = "CAMERA_OFF"
	AntiCheatMultiDevice    AntiCheatEventType = "MULTI_DEVICE"
	AntiCheatMouseLeave     AntiCheatEventType = "MOUSE_LEAVE"
	AntiCheatIdleTimeout    AntiCheatEventType = "IDLE_TIMEOUT"
	AntiCheatVoiceDetected  AntiCheatEventType = "VOICE_DETECTED"
)

// AntiCheatSeverity rates a single event.
type AntiCheatSeverity string

const (
	AntiCheatSeverityLow      AntiCheatSeverity = "LOW"
	AntiCheatSeverityMedium   AntiCheatSeverity = "MEDIUM"
	AntiCheatSeverityHigh     AntiCheatSeverity = "HIGH"
	AntiCheatSeverityCritical AntiCheatSeverity = "CRITICAL"
)

// AntiCheatStatus is the review state of an aggregated flag.
type AntiCheatStatus string

const (
	AntiCheatStatusOpen        AntiCheatStatus = "OPEN"
	AntiCheatStatusUnderReview AntiCheatStatus = "UNDER_REVIEW"
	AntiCheatStatusCleared     AntiCheatStatus = "CLEARED"
	AntiCheatStatusDismissed   AntiCheatStatus = "DISMISSED"
	AntiCheatStatusBlocked     AntiCheatStatus = "BLOCKED"
)

// AntiCheatEvent is a single raw proctoring event recorded during an exam
// attempt.
type AntiCheatEvent struct {
	ID        string            `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID    string            `gorm:"not null;type:uuid;column:user_id" json:"userId"`
	ExamID    *string           `gorm:"type:uuid;column:exam_id" json:"examId"`
	AttemptID *string           `gorm:"type:uuid;column:attempt_id" json:"attemptId"`
	EventType AntiCheatEventType `gorm:"not null;index;column:event_type" json:"eventType"`
	Severity  AntiCheatSeverity  `gorm:"not null;default:'LOW';index;column:severity" json:"severity"`
	Detail    *string           `gorm:"type:text;column:detail" json:"detail"`
	Metadata  []byte            `gorm:"type:jsonb;column:metadata" json:"metadata"`
	IPAddress string            `gorm:"column:ip_address" json:"ipAddress"`
	UserAgent string            `gorm:"type:text;column:user_agent" json:"userAgent"`
	CreatedAt time.Time         `gorm:"not null;index;column:created_at" json:"createdAt"`
}

func (AntiCheatEvent) TableName() string {
	return "AntiCheatEvent"
}

func (e *AntiCheatEvent) BeforeCreate(tx *gorm.DB) (err error) {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	return
}

// AntiCheatFlag is an aggregated review case for a (user, exam, attempt)
// combination: risk score, review status, evidence summary and outcome.
type AntiCheatFlag struct {
	ID         string          `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID     string          `gorm:"not null;type:uuid;column:user_id" json:"userId"`
	ExamID     *string         `gorm:"type:uuid;column:exam_id" json:"examId"`
	AttemptID  *string         `gorm:"type:uuid;column:attempt_id" json:"attemptId"`
	RiskScore  int             `gorm:"not null;default:0;index;column:risk_score" json:"riskScore"`
	Status     AntiCheatStatus `gorm:"not null;default:'OPEN';index;column:status" json:"status"`
	Reason     string          `gorm:"type:text;column:reason" json:"reason"`
	Evidence   []byte          `gorm:"type:jsonb;column:evidence" json:"evidence"`
	EventCount int             `gorm:"not null;default:0;column:event_count" json:"eventCount"`
	IPAddress  string          `gorm:"column:ip_address" json:"ipAddress"`
	ReviewerID *string         `gorm:"type:uuid;column:reviewer_id" json:"reviewerId"`
	ReviewedAt *time.Time      `gorm:"column:reviewed_at" json:"reviewedAt"`
	ReviewNote *string         `gorm:"type:text;column:review_note" json:"reviewNote"`
	CreatedAt  time.Time       `gorm:"not null;index;column:created_at" json:"createdAt"`
	UpdatedAt  time.Time       `gorm:"column:updated_at" json:"updatedAt"`
}

func (AntiCheatFlag) TableName() string {
	return "AntiCheatFlag"
}

func (f *AntiCheatFlag) BeforeCreate(tx *gorm.DB) (err error) {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	return
}
