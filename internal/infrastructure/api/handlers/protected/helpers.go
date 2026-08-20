package protected

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// currentUserID extracts the authenticated user's ID from the Gin context.
func currentUserID(c *gin.Context) (string, bool) {
	userID := c.GetString("userId")
	if userID == "" {
		return "", false
	}
	return userID, true
}

// mapErrorToHTTPStatus intelligently maps service-layer errors to appropriate HTTP status codes.
func mapErrorToHTTPStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return http.StatusNotFound
	}

	errMsg := err.Error()
	if strings.Contains(errMsg, "cannot be empty") ||
		strings.Contains(errMsg, "must be between") ||
		strings.Contains(errMsg, "cannot be negative") ||
		strings.Contains(errMsg, "invalid") {
		return http.StatusBadRequest
	}

	if strings.Contains(errMsg, "database") && strings.Contains(errMsg, "not initialized") {
		return http.StatusServiceUnavailable
	}

	return http.StatusInternalServerError
}
