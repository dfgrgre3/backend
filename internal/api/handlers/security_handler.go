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

// GetAdminSecurityLogs returns overall security logs for admin monitoring
func GetAdminSecurityLogs(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	pageStr := c.DefaultQuery("page", "1")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	logs, count, err := getSecurityLogRepo().FindAll(limit, offset)
	if err != nil {
		fmt.Printf("Error fetching admin security logs: %v\n", err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch security logs")
		return
	}

	userIDs := make([]string, 0)
	for _, l := range logs {
		if l.UserID != nil && *l.UserID != "" {
			userIDs = append(userIDs, *l.UserID)
		}
	}

	userMap := make(map[string]gin.H)
	if len(userIDs) > 0 {
		var users []models.User
		db.DB.Where("id IN ?", userIDs).Find(&users)
		for _, u := range users {
			name := "مستخدم"
			if u.Name != nil && *u.Name != "" {
				name = *u.Name
			}
			userMap[u.ID] = gin.H{
				"name":  name,
				"email": u.Email,
			}
		}
	}

	formattedLogs := make([]gin.H, 0, len(logs))
	for _, l := range logs {
		var userInfo interface{}
		if l.UserID != nil {
			if u, ok := userMap[*l.UserID]; ok {
				userInfo = u
			}
		}
		formattedLogs = append(formattedLogs, gin.H{
			"id":        l.ID,
			"eventType": l.EventType,
			"userId":    l.UserID,
			"user":      userInfo,
			"ip":        l.IP,
			"userAgent": l.UserAgent,
			"location":  l.Location,
			"metadata":  l.Metadata,
			"createdAt": l.CreatedAt,
		})
	}

	api_response.Success(c, gin.H{
		"logs":  formattedLogs,
		"count": count,
	})
}

// GetDeviceFingerprints returns list of active/tracked device fingerprints
func GetDeviceFingerprints(c *gin.Context) {
	var sessions []models.UserSession
	err := db.DB.Order("last_accessed DESC").Limit(200).Find(&sessions).Error
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch device fingerprints")
		return
	}

	userIDs := make([]string, 0)
	for _, s := range sessions {
		if s.UserID != "" {
			userIDs = append(userIDs, s.UserID)
		}
	}

	userMap := make(map[string]string)
	if len(userIDs) > 0 {
		var users []models.User
		db.DB.Where("id IN ?", userIDs).Find(&users)
		for _, u := range users {
			if u.Name != nil && *u.Name != "" {
				userMap[u.ID] = *u.Name
			}
		}
	}

	devices := make([]gin.H, 0)
	seen := make(map[string]bool)

	for _, s := range sessions {
		fpKey := s.FingerprintHash
		if fpKey == "" {
			fpKey = fmt.Sprintf("%s-%s", s.UserID, s.IP)
		}
		if seen[fpKey] {
			continue
		}
		seen[fpKey] = true

		userName := userMap[s.UserID]
		if userName == "" {
			userName = "مستخدم"
		}

		devices = append(devices, gin.H{
			"id":          s.ID,
			"userId":      s.UserID,
			"userName":    userName,
			"fingerprint": fpKey,
			"ip":          s.IP,
			"userAgent":   s.UserAgent,
			"deviceType":  s.DeviceType,
			"lastSeen":    s.LastAccessed,
			"isBlocked":   s.Status == "revoked" || !s.IsActive,
			"blockReason": nil,
			"loginCount":  1,
		})
	}

	api_response.Success(c, gin.H{
		"devices": devices,
	})
}

// BlockDeviceFingerprint blocks a device fingerprint or session
func BlockDeviceFingerprint(c *gin.Context) {
	var req struct {
		FingerprintID string `json:"fingerprintId"`
		Reason        string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	db.DB.Model(&models.UserSession{}).Where("id = ? OR fingerprint_hash = ?", req.FingerprintID, req.FingerprintID).Updates(map[string]interface{}{
		"is_active": false,
		"status":    "revoked",
	})

	api_response.Success(c, gin.H{"message": "Device blocked successfully"})
}

// UnblockDeviceFingerprint unblocks a device fingerprint
func UnblockDeviceFingerprint(c *gin.Context) {
	fingerprintID := c.Param("id")
	if fingerprintID == "" {
		api_response.Error(c, http.StatusBadRequest, "Fingerprint ID is required")
		return
	}

	db.DB.Model(&models.UserSession{}).Where("id = ? OR fingerprint_hash = ?", fingerprintID, fingerprintID).Updates(map[string]interface{}{
		"is_active": true,
		"status":    "active",
	})

	api_response.Success(c, gin.H{"message": "Device unblocked successfully"})
}

// GetRolePermissions returns role permission mappings and user counts
func GetRolePermissions(c *gin.Context) {
	type RoleCount struct {
		Role  string
		Count int
	}
	var roleCounts []RoleCount
	db.DB.Model(&models.User{}).Select("role, count(*) as count").Group("role").Scan(&roleCounts)

	countMap := make(map[string]int)
	for _, rc := range roleCounts {
		countMap[strings.ToUpper(rc.Role)] = rc.Count
	}

	roles := []gin.H{
		{
			"id":          "super_admin",
			"name":        "SUPER_ADMIN",
			"description": "مدير النظام الأعلى - جميع الصلاحيات والإعدادات الحساسة",
			"permissions": []string{"all", "manage_users", "manage_roles", "manage_backups", "manage_system"},
			"userCount":   countMap["SUPER_ADMIN"],
		},
		{
			"id":          "admin",
			"name":        "ADMIN",
			"description": "مدير - إدارة المستخدمين، المحتوى والتقارير",
			"permissions": []string{"manage_users", "manage_courses", "manage_analytics", "manage_tickets"},
			"userCount":   countMap["ADMIN"],
		},
		{
			"id":          "moderator",
			"name":        "MODERATOR",
			"description": "مشرف - إدارة المحتوى والمنتديات ودعم المشتركين",
			"permissions": []string{"manage_courses", "manage_tickets", "moderate_forum"},
			"userCount":   countMap["MODERATOR"],
		},
		{
			"id":          "teacher",
			"name":        "TEACHER",
			"description": "معلم - إنشاء المواد، الاختبارات والأنشطة",
			"permissions": []string{"create_courses", "manage_exams", "grade_students"},
			"userCount":   countMap["TEACHER"],
		},
		{
			"id":          "student",
			"name":        "STUDENT",
			"description": "طالب - الوصول إلى الدورات، الامتحانات والمنتدى",
			"permissions": []string{"view_courses", "take_exams", "use_forum"},
			"userCount":   countMap["STUDENT"],
		},
	}

	api_response.Success(c, gin.H{
		"roles": roles,
	})
}

