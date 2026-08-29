package protected

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// ─────────────────────────────────────────────
//  Security Logs
// ─────────────────────────────────────────────

func AdminListSecurityLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	search := c.Query("search")
	status := c.Query("status")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.SecurityLog{})
	if search != "" {
		query = query.Joins("JOIN \"User\" ON \"SecurityLog\".user_id = \"User\".id").
			Where("\"User\".name ILIKE ? OR \"User\".email ILIKE ? OR \"SecurityLog\".ip_address ILIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var logs []models.SecurityLog
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch security logs")
		return
	}

	items := make([]gin.H, 0, len(logs))
	var successCount, failedCount, blockedCount, highRiskCount int64
	for _, l := range logs {
		items = append(items, securityLogToGin(l))
		switch l.EventType {
		case models.SecurityEventLoginSuccess:
			successCount++
		case models.SecurityEventLoginFailed:
			failedCount++
		case models.SecurityEvent2FAFailed:
			blockedCount++
		}
		if l.Metadata != nil && len(*l.Metadata) > 0 {
			highRiskCount++
		}
	}

	api_response.Success(c, gin.H{
		"logs":       items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalLogs": total, "successCount": successCount, "failedCount": failedCount, "blockedCount": blockedCount, "highRiskCount": highRiskCount},
	})
}

func securityLogToGin(l models.SecurityLog) gin.H {
	userName := ""
	userEmail := ""
	var user models.User
	if err := db.DB.Select("name", "email").Where("id = ?", l.UserID).First(&user).Error; err == nil {
		if user.Name != nil {
			userName = *user.Name
		}
		userEmail = user.Email
	}
	return gin.H{
		"id":        l.ID,
		"userId":    l.UserID,
		"userName":  userName,
		"userEmail": userEmail,
		"action":    l.EventType,
		"resource":  "security_log",
		"ipAddress": l.IP,
		"userAgent": l.UserAgent,
		"status":    string(l.EventType),
		"riskScore": 0,
		"location":  l.Location,
		"createdAt": l.CreatedAt,
	}
}

// ─────────────────────────────────────────────
//  Activity Log
// ─────────────────────────────────────────────

func AdminListActivityLog(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	search := c.Query("search")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.ActivityLog{})
	if search != "" {
		query = query.Joins("JOIN \"User\" ON \"ActivityLog\".user_id = \"User\".id").
			Where("\"User\".name ILIKE ? OR \"User\".email ILIKE ? OR \"ActivityLog\".action ILIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var logs []models.ActivityLog
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch activity log")
		return
	}

	items := make([]gin.H, 0, len(logs))
	for _, l := range logs {
		items = append(items, activityLogToGin(l))
	}

	today := time.Now().Truncate(24 * time.Hour)
	weekAgo := today.AddDate(0, 0, -7)
	var todayCount, weekCount int64
	db.DB.Model(&models.ActivityLog{}).Where("created_at >= ?", today).Count(&todayCount)
	db.DB.Model(&models.ActivityLog{}).Where("created_at >= ?", weekAgo).Count(&weekCount)

	api_response.Success(c, gin.H{
		"logs":       items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalLogs": total, "todayCount": todayCount, "weekCount": weekCount},
	})
}

func activityLogToGin(l models.ActivityLog) gin.H {
	userName := ""
	userEmail := ""
	var user models.User
	if err := db.DB.Select("name", "email").Where("id = ?", l.UserID).First(&user).Error; err == nil {
		if user.Name != nil {
			userName = *user.Name
		}
		userEmail = user.Email
	}
	return gin.H{
		"id":         l.ID,
		"userId":     l.UserID,
		"userName":   userName,
		"userEmail":  userEmail,
		"action":     l.Action,
		"resource":   l.Resource,
		"resourceId": l.ResourceID,
		"ipAddress":  l.IPAddress,
		"userAgent":  l.UserAgent,
		"metadata":   l.Metadata,
		"createdAt":  l.CreatedAt,
	}
}

// ─────────────────────────────────────────────
//  User Sessions
// ─────────────────────────────────────────────

func AdminListUserSessions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	search := c.Query("search")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.UserSession{})
	if search != "" {
		query = query.Joins("JOIN \"User\" ON \"UserSession\".user_id = \"User\".id").
			Where("\"User\".name ILIKE ? OR \"User\".email ILIKE ? OR \"UserSession\".ip_address ILIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var sessions []models.UserSession
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&sessions).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch user sessions")
		return
	}

	items := make([]gin.H, 0, len(sessions))
	var activeCount int64
	uniqueUsers := make(map[string]bool)
	for _, s := range sessions {
		items = append(items, userSessionToGin(s))
		if s.IsActive {
			activeCount++
		}
		uniqueUsers[s.UserID] = true
	}

	api_response.Success(c, gin.H{
		"sessions":   items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalSessions": total, "activeSessions": activeCount, "uniqueUsers": len(uniqueUsers)},
	})
}

func AdminRevokeUserSession(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Model(&models.UserSession{}).Where("id = ?", id).Updates(map[string]interface{}{
		"is_active": false,
	}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to revoke session")
		return
	}
	api_response.Success(c, gin.H{"message": "Session revoked"})
}

func userSessionToGin(s models.UserSession) gin.H {
	userName := ""
	userEmail := ""
	var user models.User
	if err := db.DB.Select("name", "email").Where("id = ?", s.UserID).First(&user).Error; err == nil {
		if user.Name != nil {
			userName = *user.Name
		}
		userEmail = user.Email
	}

	ipAddress := s.IPAddress
	if ipAddress == "" {
		ipAddress = s.IP
	}
	lastActivity := s.LastActivity
	if lastActivity.IsZero() {
		lastActivity = s.LastAccessed
	}
	device := s.Device
	if device == nil && s.DeviceType != "" {
		deviceValue := s.DeviceType
		device = &deviceValue
	}

	return gin.H{
		"id":           s.ID,
		"userId":       s.UserID,
		"userName":     userName,
		"userEmail":    userEmail,
		"ipAddress":    ipAddress,
		"userAgent":    s.UserAgent,
		"device":       device,
		"location":     s.Location,
		"isActive":     s.IsActive,
		"lastActivity": lastActivity,
		"createdAt":    s.CreatedAt,
		"expiresAt":    s.ExpiresAt,
	}
}

// ─────────────────────────────────────────────
//  Login Attempts
// ─────────────────────────────────────────────

func AdminListLoginAttempts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	search := c.Query("search")
	status := c.Query("status")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.LoginAttempt{})
	if search != "" {
		query = query.Where("ip_address ILIKE ?", "%"+search+"%")
	}
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var attempts []models.LoginAttempt
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&attempts).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch login attempts")
		return
	}

	items := make([]gin.H, 0, len(attempts))
	var successCount, failedCount, blockedCount, suspiciousCount int64
	for _, a := range attempts {
		items = append(items, loginAttemptToGin(a))
		switch a.Status {
		case "SUCCESS":
			successCount++
		case "FAILED":
			failedCount++
		case "BLOCKED":
			blockedCount++
		}
		if a.RiskScore > 50 {
			suspiciousCount++
		}
	}

	api_response.Success(c, gin.H{
		"attempts":   items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalAttempts": total, "successCount": successCount, "failedCount": failedCount, "blockedCount": blockedCount, "suspiciousCount": suspiciousCount},
	})
}

func loginAttemptToGin(a models.LoginAttempt) gin.H {
	userName := ""
	userEmail := ""
	if a.UserID != nil {
		var user models.User
		if err := db.DB.Select("name", "email").Where("id = ?", *a.UserID).First(&user).Error; err == nil {
			if user.Name != nil {
				userName = *user.Name
			}
			userEmail = user.Email
		}
	}
	return gin.H{
		"id":            a.ID,
		"userId":        a.UserID,
		"userName":      userName,
		"userEmail":     userEmail,
		"ipAddress":     a.IPAddress,
		"userAgent":     a.UserAgent,
		"status":        a.Status,
		"failureReason": a.FailureReason,
		"location":      a.Location,
		"riskScore":     a.RiskScore,
		"createdAt":     a.CreatedAt,
	}
}
