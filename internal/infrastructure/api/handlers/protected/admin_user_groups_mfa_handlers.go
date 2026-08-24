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
//  User Groups Management
// ─────────────────────────────────────────────

func AdminListUserGroups(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.UserGroup{})
	if search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var groups []models.UserGroup
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&groups).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch user groups")
		return
	}

	items := make([]gin.H, 0, len(groups))
	var totalMembers int64
	for _, g := range groups {
		var memberCount int64
		db.DB.Model(&models.User{}).Where("group_id = ?", g.ID).Count(&memberCount)
		totalMembers += memberCount
		items = append(items, userGroupToGin(g, memberCount))
	}

	var activeCount int64
	db.DB.Model(&models.UserGroup{}).Where("is_active = ?", true).Count(&activeCount)

	api_response.Success(c, gin.H{
		"groups":     items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalGroups": total, "activeGroups": activeCount, "totalMembers": totalMembers},
	})
}

func AdminCreateUserGroup(c *gin.Context) {
	var req models.UserGroup
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := SafeCreate(db.DB, &req); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create user group")
		return
	}
	api_response.Created(c, userGroupToGin(req, 0))
}

func AdminUpdateUserGroup(c *gin.Context) {
	id := c.Param("id")
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req["updated_at"] = time.Now()
	if err := db.DB.Model(&models.UserGroup{}).Where("id = ?", id).Updates(req).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update user group")
		return
	}
	api_response.Success(c, gin.H{"message": "User group updated"})
}

func AdminDeleteUserGroup(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Delete(&models.UserGroup{}, "id = ?", id).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete user group")
		return
	}
	api_response.Success(c, gin.H{"message": "User group deleted"})
}

func userGroupToGin(g models.UserGroup, memberCount int64) gin.H {
	return gin.H{
		"id":           g.ID,
		"name":         g.Name,
		"description":  g.Description,
		"type":         g.Type,
		"membersCount": memberCount,
		"isActive":     g.IsActive,
		"createdBy":    g.CreatedBy,
		"createdAt":    g.CreatedAt,
	}
}

// ─────────────────────────────────────────────
//  MFA Management
// ─────────────────────────────────────────────

func AdminListMFA(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	search := c.Query("search")
	status := c.Query("status")

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	query := db.DB.Model(&models.MFA{})
	if search != "" {
		query = query.Joins("JOIN \"User\" ON \"MFA\".user_id = \"User\".id").
			Where("\"User\".name ILIKE ? OR \"User\".email ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if status != "" && status != "all" {
		query = query.Where("is_enabled = ?", status == "enabled")
	}

	var total int64
	query.Count(&total)

	var mfas []models.MFA
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&mfas).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch MFA data")
		return
	}

	items := make([]gin.H, 0, len(mfas))
	var enabledCount, disabledCount, totpCount, smsCount int64
	for _, m := range mfas {
		items = append(items, mfaToGin(m))
		if m.IsEnabled {
			enabledCount++
			if m.Method != nil {
				switch string(*m.Method) {
				case "TOTP":
					totpCount++
				case "SMS":
					smsCount++
				}
			}
		} else {
			disabledCount++
		}
	}

	api_response.Success(c, gin.H{
		"users":      items,
		"pagination": gin.H{"page": page, "limit": limit, "total": total, "totalPages": (total + int64(limit) - 1) / int64(limit)},
		"summary":    gin.H{"totalUsers": total, "enabledCount": enabledCount, "disabledCount": disabledCount, "totpCount": totpCount, "smsCount": smsCount},
	})
}

func AdminResetMFA(c *gin.Context) {
	id := c.Param("id")
	if err := db.DB.Model(&models.MFA{}).Where("user_id = ?", id).Updates(map[string]interface{}{
		"is_enabled":   false,
		"method":       nil,
		"secret":       nil,
		"backup_codes": nil,
	}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to reset MFA")
		return
	}
	api_response.Success(c, gin.H{"message": "MFA reset successfully"})
}

func mfaToGin(m models.MFA) gin.H {
	userName := ""
	var user models.User
	if err := db.DB.Select("name", "email").Where("id = ?", m.UserID).First(&user).Error; err == nil {
		if user.Name != nil {
			userName = *user.Name
		}
	}
	return gin.H{
		"id":              m.ID,
		"name":            userName,
		"email":           user.Email,
		"mfaEnabled":      m.IsEnabled,
		"mfaMethod":       m.Method,
		"backupCodesUsed": m.BackupCodesUsed,
		"lastUsedAt":      m.LastUsedAt,
		"enrolledAt":      m.CreatedAt,
	}
}
