package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestShouldSkipLogging_SkipsSensitivePaths(t *testing.T) {
	assert.True(t, shouldSkipLogging("/healthz"))
	assert.True(t, shouldSkipLogging("/api/admin/audit-logs"))
	assert.False(t, shouldSkipLogging("/api/admin/users"))
}

func TestAuditLoggerSanitizesSensitiveFields(t *testing.T) {
	logger := NewAdminAuditLogger(DefaultAuditLoggerConfig())
	body := map[string]interface{}{
		"password": "secret",
		"email":    "user@example.com",
		"nested": map[string]interface{}{
			"token": "abc",
		},
	}

	sanitized := logger.sanitizeBody(body)
	assert.Equal(t, "[REDACTED]", sanitized["password"])
	assert.Equal(t, "[REDACTED]", sanitized["nested"].(map[string]interface{})["token"])
	assert.Equal(t, "user@example.com", sanitized["email"])
}

func TestAdminAuditLogger_SkipsLoggingForIgnoredPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	logger := NewAdminAuditLogger(DefaultAuditLoggerConfig())
	r.Use(logger.LogAdminOperations())
	r.GET("/healthz", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
