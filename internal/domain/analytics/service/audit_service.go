package analyticsservice

import (
	"encoding/json"
	"log"
	"strings"
	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"
)

// Audit Event Types
const (
	AuditEventLogin              = "auth.login"
	AuditEventLogout             = "auth.logout"
	AuditEventRegister           = "auth.register"
	AuditEventLoginFailed        = "auth.login_failed"
	AuditEventPasswordChange     = "user.password_change"
	AuditEventProfileUpdate      = "user.profile_update"
	AuditEventPaymentStarted     = "payment.started"
	AuditEventPaymentSuccess     = "payment.success"
	AuditEventPaymentFailed      = "payment.failed"
	AuditEventExamStarted        = "exam.started"
	AuditEventExamFinished       = "exam.finished"
	AuditEventAdminAction        = "admin.action"
	AuditEventDataDeletion       = "data.deletion"
	AuditEventImpersonationStart = "admin.impersonation_start"
)

type AuditService struct{}

var auditServiceInstance *AuditService

func GetAuditService() *AuditService {
	if auditServiceInstance == nil {
		auditServiceInstance = &AuditService{}
	}
	return auditServiceInstance
}

// LogEvent records a new audit log entry
func (s *AuditService) LogEvent(userID, eventType, resource, resourceID string, metadata interface{}, ip, userAgent string) {
	var nullableUserID *string
	if strings.TrimSpace(userID) != "" {
		nullableUserID = &userID
	}

	metadataJSON := ""
	if metadata != nil {
		bytes, err := json.Marshal(metadata)
		if err == nil {
			metadataJSON = string(bytes)
		}
	}

	auditLog := models.AuditLog{
		UserID:     nullableUserID,
		EventType:  eventType,
		Action:     eventType,
		Resource:   resource,
		ResourceID: resourceID,
		Metadata:   metadataJSON,
		IP:         ip,
		UserAgent:  userAgent,
	}

	// Save to DB using the raw write connection (bypasses RLS).
	// AuditLog is an internal system table written by the backend itself,
	// not by end users, so it does not require multi-tenant row isolation.
	// Using RawWriteDB ensures audit logs are always persisted even when
	// RLS is enabled on the table without a matching policy.
	if err := db.RawWriteDB().Create(&auditLog).Error; err != nil {
		// Log at debug level - table might not exist yet during migrations
		log.Printf("[DEBUG] Audit log not saved: %v", err)
	}
}

// LogAsync records an audit log without blocking the main thread
func (s *AuditService) LogAsync(userID, eventType, resource, resourceID string, metadata interface{}, ip, userAgent string) {
	go s.LogEvent(userID, eventType, resource, resourceID, metadata, ip, userAgent)
}
