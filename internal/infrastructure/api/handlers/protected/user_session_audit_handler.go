package protected

import (
	"net/http"
	"strconv"
	"strings"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
)

func GetUserSessions(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}
	var sessions []models.UserSession
	if err := db.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&sessions).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch sessions")
		return
	}
	items := make([]gin.H, 0, len(sessions))
	for _, s := range sessions {
		items = append(items, gin.H{
			"id":           s.ID,
			"device":       s.Device,
			"browser":      s.Browser,
			"os":           s.OS,
			"ip":           s.IP,
			"country":      s.Country,
			"lastActivity": s.LastActivity,
			"loginTime":    s.CreatedAt,
			"logoutTime":   s.RevokedAt,
			"isCurrent":    s.IsActive,
		})
	}
	api_response.Success(c, items)
}

// ListUserSessions returns a paginated, server-filtered view of sessions
// across ALL users for the admin workspace ("user-sessions" page). It accepts
// the same page/limit query parameters as the other admin list endpoints and
// supports optional narrowing by userId, activity state and status. Ordering
// and pagination are executed by PostgreSQL so the endpoint stays safe for
// large session tables.
func ListUserSessions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	query := db.DB.Model(&models.UserSession{})
	if userID := strings.TrimSpace(c.Query("userId")); userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	switch active := c.Query("active"); active {
	case "true":
		query = query.Where("is_active = ?", true)
	case "false":
		query = query.Where("is_active = ?", false)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to count sessions")
		return
	}

	var sessions []models.UserSession
	if err := query.
		Order("last_accessed DESC, created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&sessions).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch sessions")
		return
	}

	items := make([]gin.H, 0, len(sessions))
	for _, s := range sessions {
		items = append(items, gin.H{
			"id":           s.ID,
			"userId":       s.UserID,
			"device":       s.Device,
			"deviceType":   s.DeviceType,
			"browser":      s.Browser,
			"os":           s.OS,
			"ip":           s.IP,
			"country":      s.Country,
			"location":     s.Location,
			"status":       s.Status,
			"isActive":     s.IsActive,
			"lastActivity": s.LastAccessed,
			"loginTime":    s.CreatedAt,
			"logoutTime":   s.RevokedAt,
			"expiresAt":    s.ExpiresAt,
		})
	}
	api_response.List(c, items, api_response.Pagination{
		Page: page, Limit: limit, Total: total, TotalPages: calculateTotalPages(total, limit),
	}, gin.H{"items": items})
}

// TerminateSession terminates a specific session.
func TerminateSession(c *gin.Context) {
	userID := c.Param("id")
	sessionID := c.Param("sessionId")
	if userID == "" || sessionID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id and session id are required")
		return
	}
	if err := db.DB.Model(&models.UserSession{}).Where("id = ? AND user_id = ?", sessionID, userID).Updates(map[string]interface{}{
		"is_active":  false,
		"status":     "revoked",
		"revoked_at": time.Now(),
		"updated_at": time.Now(),
	}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to terminate session")
		return
	}
	LogAudit(c, "TERMINATE_SESSION", "user", userID, gin.H{"sessionId": sessionID})
	api_response.Success(c, gin.H{"success": true})
}

// TerminateAllSessions terminates all sessions for a user.
func TerminateAllSessions(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}
	if err := db.DB.Model(&models.UserSession{}).Where("user_id = ?", userID).Updates(map[string]interface{}{
		"is_active":  false,
		"status":     "revoked",
		"revoked_at": time.Now(),
		"updated_at": time.Now(),
	}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to terminate sessions")
		return
	}
	LogAudit(c, "TERMINATE_ALL_SESSIONS", "user", userID, nil)
	api_response.Success(c, gin.H{"success": true})
}

// GetUserAuditLogs returns audit logs for a user.
func GetUserAuditLogs(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 50
	}
	pageStr := c.DefaultQuery("page", "1")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	var total int64
	if err := db.DB.Model(&models.AuditLog{}).Where("resource_id = ?", userID).Count(&total).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to count audit logs")
		return
	}
	var logs []models.AuditLog
	if err := db.DB.Where("resource_id = ?", userID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch audit logs")
		return
	}
	items := make([]gin.H, 0, len(logs))
	for _, l := range logs {
		items = append(items, gin.H{
			"id":          l.ID,
			"action":      l.Action,
			"eventType":   l.EventType,
			"performedBy": l.UserID,
			"resource":    l.Resource,
			"changes":     l.Changes,
			"metadata":    l.Metadata,
			"ip":          l.IP,
			"createdAt":   l.CreatedAt,
		})
	}
	api_response.List(c, items, api_response.Pagination{
		Page: page, Limit: limit, Total: total, TotalPages: calculateTotalPages(total, limit),
	}, gin.H{"items": items})
}

// GetUserPermissions returns permissions for a user.
func GetUserPermissions(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}
	var user models.User
	if err := db.DB.First(&user, idQuery, userID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}
	api_response.Success(c, gin.H{
		"role":        user.Role,
		"roles":       []string{string(user.Role)},
		"permissions": user.GetEffectivePermissions(),
	})
}
