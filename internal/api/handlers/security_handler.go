package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	api_response "thanawy-backend/internal/api/response"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"
	"thanawy-backend/internal/repository"

	"github.com/gin-gonic/gin"
)

var (
	securityLogRepo     *repository.SecurityLogRepository
	securityLogRepoOnce sync.Once
)

func getSecurityLogRepo() *repository.SecurityLogRepository {
	securityLogRepoOnce.Do(func() {
		securityLogRepo = repository.NewSecurityLogRepository(db.DB)
	})
	return securityLogRepo
}
func GetSecurityLogs(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	userIdStr, ok := userId.(string)
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, "Invalid user ID in session")
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	logs, err := getSecurityLogRepo().FindByUserID(userIdStr, limit)
	if err != nil {
		fmt.Printf("Error fetching security logs for user %s: %v\n", userIdStr, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch security logs")
		return
	}

	api_response.Success(c, gin.H{
		"logs": logs,
	})
}

func GetSecurityLogsForUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	logs, err := getSecurityLogRepo().FindByUserID(userID, limit)
	if err != nil {
		fmt.Printf("Error fetching security logs for user %s: %v\n", userID, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch security logs")
		return
	}

	api_response.Success(c, gin.H{
		"logs": logs,
	})
}

func GetUserLoginAttempts(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	logs, err := getSecurityLogRepo().FindByUserID(userID, limit)
	if err != nil {
		fmt.Printf("Error fetching login attempts for user %s: %v\n", userID, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch login attempts")
		return
	}

	attempts := make([]gin.H, 0, len(logs))
	failedCount := 0
	for _, logEntry := range logs {
		success := logEntry.EventType == models.SecurityEventLoginSuccess
		if !success {
			failedCount++
		}
		attempts = append(attempts, gin.H{
			"id":         logEntry.ID,
			"eventType":  logEntry.EventType,
			"success":    success,
			"ip":         logEntry.IP,
			"userAgent":  logEntry.UserAgent,
			"location":   logEntry.Location,
			"createdAt":  logEntry.CreatedAt,
		})
	}

	api_response.Success(c, gin.H{
		"userId":      userID,
		"total":       len(attempts),
		"failedCount": failedCount,
		"attempts":    attempts,
	})
}

// LogSecurityEvent is a helper function to log security events
// This can be called from other handlers after successful operations
// Note: Errors are silently ignored if table doesn't exist (migration pending)
func LogSecurityEvent(userID string, eventType models.SecurityEventType, ip, userAgent string, location *string, metadata *string) error {
	var nullableUserID *string
	if strings.TrimSpace(userID) != "" {
		nullableUserID = &userID
	}

	log := &models.SecurityLog{
		UserID:    nullableUserID,
		EventType: eventType,
		IP:        ip,
		UserAgent: userAgent,
		Location:  location,
		Metadata:  metadata,
	}
	go func() {
		_ = getSecurityLogRepo().Create(log)
	}()
	return nil
}
