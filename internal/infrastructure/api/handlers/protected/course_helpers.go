package protected

import (
	"encoding/json"

	models "thanawy-backend/internal/domain/common"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// =============================================================
// Helper Functions
// =============================================================

func optionalStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func boolPtr(s string) *bool {
	if s == "" {
		return nil
	}
	b := s == "true"
	return &b
}

func decimalPtrFromFloatPtr(f *float64) *decimal.Decimal {
	if f == nil {
		return nil
	}
	d := decimal.NewFromFloat(*f)
	return &d
}

// logAudit logs audit events to the database
func (h *CourseRESTHandler) logAudit(c *gin.Context, action, resourceType, resourceID string, details gin.H) {
	if h.db == nil {
		return
	}

	// Get user ID from context
	userID, _ := c.Get("userId")
	var userIDStr *string
	if uid, ok := userID.(string); ok && uid != "" {
		userIDStr = &uid
	}

	// Get IP address
	ip := c.ClientIP()

	// Get user agent
	userAgent := c.GetHeader("User-Agent")

	// Convert details to JSON (jsonb columns reject empty strings)
	changesJSON := "{}"
	if len(details) > 0 {
		if bytes, err := json.Marshal(details); err == nil {
			changesJSON = string(bytes)
		}
	}

	auditLog := models.AuditLog{
		UserID:     userIDStr,
		EventType:  action,
		Action:     action,
		Resource:   resourceType,
		ResourceID: resourceID,
		Changes:    models.JSONText(changesJSON),
		Metadata:   models.JSONText(changesJSON),
		IP:         models.InetText(ip),
		UserAgent:  userAgent,
	}

	if err := h.db.Create(&auditLog).Error; err != nil {
		// Log error but don't fail the request
		// In production, you might want to use a proper logger
	}
}
