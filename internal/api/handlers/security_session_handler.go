package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	api_response "thanawy-backend/internal/api/response"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/middleware"
	"thanawy-backend/internal/models"
)

// GetActiveSessions returns all active sessions
// @Summary Get active sessions
// @Description Get all active sessions for the current user or all users (admin)
// @Tags admin,security
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/security/sessions [get]
func GetActiveSessions(c *gin.Context) {
	userID, _ := c.Get("userId")
	isAdmin := c.GetBool("is_admin")

	query := db.DB.Model(&models.UserSession{}).Where("is_active = ?", true)

	// Regular users can only see their own sessions
	if !isAdmin {
		query = query.Where("user_id = ?", userID)
	}

	var sessions []models.UserSession
	if err := query.Order("last_accessed DESC").Find(&sessions).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch sessions")
		return
	}

	api_response.Success(c, gin.H{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// RevokeSession revokes a specific session
// @Summary Revoke session
// @Description Revoke/end a specific session
// @Tags admin,security
// @Accept json
// @Produce json
// @Param id path string true "Session ID"
// @Success 200 {object} map[string]string
// @Router /api/admin/security/sessions/{id}/revoke [post]
func RevokeSession(c *gin.Context) {
	sessionID := c.Param("id")

	var session models.UserSession
	if err := db.DB.First(&session, "id = ?", sessionID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Session not found")
		return
	}

	session.IsActive = false
	session.Status = "revoked"
	now := time.Now()
	session.RevokedAt = &now

	if err := db.DB.Save(&session).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to revoke session")
		return
	}

	middleware.LogCriticalOperation(c, "session_revoked", map[string]interface{}{
		"session_id": sessionID,
		"user_id":    session.UserID,
	})

	api_response.Success(c, gin.H{"message": "Session revoked successfully"})
}

// RevokeOtherSessions revokes all sessions except current
// @Summary Revoke other sessions
// @Description Revoke all other sessions except the current one
// @Tags admin,security
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/security/sessions/revoke-others [post]
func RevokeOtherSessions(c *gin.Context) {
	userID, _ := c.Get("userId")
	currentSessionID := c.GetString("jti")

	result := db.DB.Model(&models.UserSession{}).
		Where("user_id = ? AND id != ? AND is_active = ?", userID, currentSessionID, true).
		Updates(map[string]interface{}{
			"is_active":  false,
			"status":     "revoked",
			"revoked_at": time.Now(),
		})

	middleware.LogCriticalOperation(c, "sessions_revoked_others", map[string]interface{}{
		"revoked_count": result.RowsAffected,
	})

	api_response.Success(c, gin.H{
		"message": "Other sessions revoked",
		"revokedCount": result.RowsAffected,
	})
}

// RevokeUserSessions revokes all sessions for a specific user
// @Summary Revoke user sessions
// @Description Revoke all sessions for a specific user (admin only)
// @Tags admin,security
// @Accept json
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/security/sessions/user/{userId}/revoke-all [post]
func RevokeUserSessions(c *gin.Context) {
	userID := c.Param("userId")

	result := db.DB.Model(&models.UserSession{}).
		Where("user_id = ? AND is_active = ?", userID, true).
		Updates(map[string]interface{}{
			"is_active":  false,
			"status":     "revoked",
			"revoked_at": time.Now(),
		})

	middleware.LogCriticalOperation(c, "user_sessions_revoked", map[string]interface{}{
		"target_user":   userID,
		"revoked_count": result.RowsAffected,
	})

	api_response.Success(c, gin.H{
		"message": "All user sessions revoked",
		"revokedCount": result.RowsAffected,
	})
}

// GetSessionStats returns session statistics
// @Summary Get session statistics
// @Description Get statistics about user sessions
// @Tags admin,security
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/security/sessions/stats [get]
func GetSessionStats(c *gin.Context) {
	var stats struct {
		TotalActive   int64 `json:"totalActive"`
		TotalExpired  int64 `json:"totalExpired"`
		UniqueDevices int64 `json:"uniqueDevices"`
	}

	db.DB.Model(&models.UserSession{}).Where("is_active = ?", true).Count(&stats.TotalActive)
	db.DB.Model(&models.UserSession{}).Where("is_active = ?", false).Count(&stats.TotalExpired)
	db.DB.Model(&models.UserSession{}).Where("is_active = ?", true).Select("COUNT(DISTINCT user_agent)").Scan(&stats.UniqueDevices)

	api_response.Success(c, stats)
}
