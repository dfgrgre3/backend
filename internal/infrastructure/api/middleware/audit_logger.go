package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	analyticsservice "thanawy-backend/internal/domain/analytics/service"
	"time"

	"github.com/gin-gonic/gin"
)

// AuditLoggerConfig configures audit logging behavior
type AuditLoggerConfig struct {
	// LogRequestBody determines if request body should be logged
	LogRequestBody bool
	// LogResponseBody determines if response body should be logged
	LogResponseBody bool
	// MaxBodySize limits the size of logged body (in bytes)
	MaxBodySize int
	// SensitiveFields are fields that should be redacted from logs
	SensitiveFields []string
}

// DefaultAuditLoggerConfig returns default audit logger configuration
func DefaultAuditLoggerConfig() AuditLoggerConfig {
	return AuditLoggerConfig{
		LogRequestBody:  true,
		LogResponseBody: false, // Don't log responses by default (can be large)
		MaxBodySize:     10000, // 10KB max
		SensitiveFields: []string{
			"password", "token", "secret", "api_key", "credit_card",
			"cvv", "pin", "authorization", "cookie", "session",
		},
	}
}

// AdminAuditLogger logs all admin operations
type AdminAuditLogger struct {
	config AuditLoggerConfig
	audit  *analyticsservice.AuditService
}

// NewAdminAuditLogger creates a new admin audit logger
func NewAdminAuditLogger(config AuditLoggerConfig) *AdminAuditLogger {
	return &AdminAuditLogger{
		config: config,
		audit:  analyticsservice.GetAuditService(),
	}
}

// LogAdminOperations middleware that logs all admin API operations
func (al *AdminAuditLogger) LogAdminOperations() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip logging for certain paths
		if shouldSkipLogging(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Capture request start time
		startTime := time.Now()

		// Capture request body if enabled
		var requestBody map[string]interface{}
		if al.config.LogRequestBody && c.Request.Body != nil {
			bodyBytes, _ := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			// Try to parse as JSON
			if len(bodyBytes) > 0 && len(bodyBytes) < al.config.MaxBodySize {
				var body map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &body); err == nil {
					requestBody = al.sanitizeBody(body)
				}
			}
		}

		// Capture response using custom writer
		responseWriter := &responseCaptureWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = responseWriter

		// Process request
		c.Next()

		// Build audit log entry
		duration := time.Since(startTime)
		al.logOperation(c, requestBody, responseWriter, duration)
	}
}

// shouldSkipLogging checks if this path should be skipped
func shouldSkipLogging(path string) bool {
	path = normalizeAPIPath(path)
	skippedPaths := []string{
		"/healthz",
		"/readyz",
		"/metrics",
		// Reading the audit feed must not create an audit event for every poll;
		// otherwise the feed becomes dominated by its own read operations.
		"/api/admin/audit-logs",
	}

	for _, p := range skippedPaths {
		if strings.Contains(path, p) {
			return true
		}
	}

	return false
}

// sanitizeBody removes sensitive fields from logged body
func (al *AdminAuditLogger) sanitizeBody(body map[string]interface{}) map[string]interface{} {
	return al.sanitizeMap(body)
}

func (al *AdminAuditLogger) sanitizeMap(m map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})

	for key, value := range m {
		lowerKey := strings.ToLower(key)
		isSensitive := false

		for _, field := range al.config.SensitiveFields {
			if strings.Contains(lowerKey, strings.ToLower(field)) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			sanitized[key] = "[REDACTED]"
		} else {
			if subMap, ok := value.(map[string]interface{}); ok {
				sanitized[key] = al.sanitizeMap(subMap)
			} else if subSlice, ok := value.([]interface{}); ok {
				sanitized[key] = al.sanitizeSlice(subSlice)
			} else {
				sanitized[key] = value
			}
		}
	}

	return sanitized
}

func (al *AdminAuditLogger) sanitizeSlice(s []interface{}) []interface{} {
	sanitized := make([]interface{}, len(s))
	for i, val := range s {
		if subMap, ok := val.(map[string]interface{}); ok {
			sanitized[i] = al.sanitizeMap(subMap)
		} else if subSlice, ok := val.([]interface{}); ok {
			sanitized[i] = al.sanitizeSlice(subSlice)
		} else {
			sanitized[i] = val
		}
	}
	return sanitized
}

// logOperation creates and saves the audit log
func (al *AdminAuditLogger) logOperation(
	c *gin.Context,
	requestBody map[string]interface{},
	writer *responseCaptureWriter,
	duration time.Duration,
) {
	// Align with middleware.Auth: userId + role (fallback to legacy context keys if present).
	userID, uidOK := c.Get("userId")
	if !uidOK {
		userID, _ = c.Get("user_id")
	}
	userRole, roleOK := c.Get("role")
	if !roleOK {
		userRole, _ = c.Get("user_role")
	}

	// Build resource info from path
	resource, resourceID := parseResourceInfo(c.Request.URL.Path)

	// Build metadata
	metadata := map[string]interface{}{
		"method":      c.Request.Method,
		"path":        c.Request.URL.Path,
		"query":       c.Request.URL.RawQuery,
		"status_code": writer.statusCode,
		"duration_ms": duration.Milliseconds(),
		"user_agent":  c.Request.UserAgent(),
		"ip":          c.ClientIP(),
		"user_role":   userRole,
	}

	// Add request body if present
	if len(requestBody) > 0 {
		metadata["request_body"] = requestBody
	}

	// Add error info if present
	if len(c.Errors) > 0 {
		errors := make([]string, len(c.Errors))
		for i, err := range c.Errors {
			errors[i] = err.Error()
		}
		metadata["errors"] = errors
	}

	// Determine event type
	eventType := determineEventType(c.Request.Method, resource)

	// Get IP and user agent
	ip := c.ClientIP()
	userAgent := c.Request.UserAgent()

	// Convert userID to string
	userIDStr := ""
	if userID != nil {
		if value, ok := userID.(string); ok {
			userIDStr = value
		}
	}

	// Log asynchronously to not block response
	go al.audit.LogEvent(
		userIDStr,
		eventType,
		resource,
		resourceID,
		metadata,
		ip,
		userAgent,
	)
}

// parseResourceInfo extracts resource type and ID from path
func parseResourceInfo(path string) (resource, resourceID string) {
	// Path format: /api/admin/{resource}/{id}
	parts := strings.Split(strings.Trim(normalizeAPIPath(path), "/"), "/")

	if len(parts) >= 3 && parts[0] == "api" && parts[1] == "admin" {
		resource = parts[2]
		if len(parts) >= 4 {
			resourceID = parts[3]
		}
	}

	return resource, resourceID
}

// determineEventType determines the audit event type
func determineEventType(method, resource string) string {
	var action string

	switch method {
	case "GET":
		action = "view"
	case "POST":
		action = "create"
	case "PUT", "PATCH":
		action = "update"
	case "DELETE":
		action = "delete"
	default:
		action = "access"
	}

	return "admin." + resource + "." + action
}

// responseCaptureWriter captures response for logging
type responseCaptureWriter struct {
	gin.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (w *responseCaptureWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *responseCaptureWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// LogCriticalOperation specifically logs critical operations with extra metadata
func LogCriticalOperation(c *gin.Context, operation string, details map[string]interface{}) {
	audit := analyticsservice.GetAuditService()

	userID, uidOK := c.Get("userId")
	if !uidOK {
		userID, _ = c.Get("user_id")
	}
	userIDStr := ""
	if userID != nil {
		if s, ok := userID.(string); ok {
			userIDStr = s
		}
	}

	metadata := map[string]interface{}{
		"operation":  operation,
		"details":    details,
		"timestamp":  time.Now().Unix(),
		"critical":   true,
		"ip":         c.ClientIP(),
		"user_agent": c.Request.UserAgent(),
	}

	// Synchronous logging for critical operations
	audit.LogEvent(
		userIDStr,
		analyticsservice.AuditEventAdminAction,
		"critical_operation",
		"",
		metadata,
		c.ClientIP(),
		c.Request.UserAgent(),
	)
}
