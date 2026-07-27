package handlers

import (
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	api_response "thanawy-backend/internal/api/response"
	"thanawy-backend/internal/config"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/middleware"
	"thanawy-backend/internal/models"
)

// GetIPWhitelist returns all whitelist entries
// @Summary Get IP whitelist
// @Description Get all IP whitelist entries
// @Tags admin,security
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/security/ip-whitelist [get]
func GetIPWhitelist(c *gin.Context) {
	var entries []models.IPWhitelistEntry
	if err := db.DB.Order("created_at DESC").Find(&entries).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch whitelist")
		return
	}

	api_response.Success(c, gin.H{
		"entries": entries,
	})
}

// AddIPToWhitelist adds an IP to the whitelist
// @Summary Add IP to whitelist
// @Description Add an IP address to the whitelist
// @Tags admin,security
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "IP details"
// @Success 201 {object} map[string]interface{}
// @Router /api/admin/security/ip-whitelist [post]
func AddIPToWhitelist(c *gin.Context) {
	var req struct {
		IPAddress   string    `json:"ipAddress" binding:"required,ip"`
		CIDR        string    `json:"cidr,omitempty"`
		Description string    `json:"description,omitempty"`
		Type        string    `json:"type" binding:"required,oneof=admin api webhook"`
		ExpiresAt   time.Time `json:"expiresAt,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	adminID, _ := c.Get("userId")

	entry := models.IPWhitelistEntry{
		IPAddress:   req.IPAddress,
		CIDR:        req.CIDR,
		Description: req.Description,
		Type:        req.Type,
		Status:      "active",
		IsTemporary: !req.ExpiresAt.IsZero(),
		ExpiresAt:   &req.ExpiresAt,
		CreatedBy:   adminID.(string),
		CreatedAt:   time.Now(),
	}

	if err := SafeCreate(db.DB, &entry); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to add IP")
		return
	}

	middleware.LogCriticalOperation(c, "ip_whitelist_added", map[string]interface{}{
		"ip":   req.IPAddress,
		"type": req.Type,
	})

	api_response.Success(c, gin.H{
		"message": "IP added to whitelist",
		"data":    entry,
	})
}

// RemoveIPFromWhitelist removes an IP from the whitelist
// @Summary Remove IP from whitelist
// @Description Remove an IP address from the whitelist
// @Tags admin,security
// @Accept json
// @Produce json
// @Param id path string true "Entry ID"
// @Success 200 {object} map[string]string
// @Router /api/admin/security/ip-whitelist/{id} [delete]
func RemoveIPFromWhitelist(c *gin.Context) {
	id := c.Param("id")

	var entry models.IPWhitelistEntry
	if err := db.DB.First(&entry, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Entry not found")
		return
	}

	if err := db.DB.Delete(&entry).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to remove IP")
		return
	}

	middleware.LogCriticalOperation(c, "ip_whitelist_removed", map[string]interface{}{
		"ip": entry.IPAddress,
	})

	api_response.Success(c, gin.H{"message": "IP removed from whitelist"})
}

// GetIPWhitelistSettings returns whitelist settings
// @Summary Get whitelist settings
// @Description Get IP whitelist configuration settings
// @Tags admin,security
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/security/ip-whitelist/settings [get]
func GetIPWhitelistSettings(c *gin.Context) {
	var settings models.IPWhitelistSettings
	if err := db.DB.First(&settings).Error; err != nil {
		// Return defaults
		api_response.Success(c, gin.H{
			"isEnabled":          false,
			"enforceForAdmins":   false,
			"enforceForAPI":      false,
			"defaultAction":      "allow",
			"allowInternalIPs":   true,
			"internalIPRanges":   config.GlobalConfig.InternalIPRanges,
			"logBlockedAttempts": true,
			"notifyOnViolation":  true,
		})
		return
	}

	api_response.Success(c, settings)
}

// UpdateIPWhitelistSettings updates whitelist settings
// @Summary Update whitelist settings
// @Description Update IP whitelist configuration
// @Tags admin,security
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "Settings"
// @Success 200 {object} map[string]string
// @Router /api/admin/security/ip-whitelist/settings [patch]
func UpdateIPWhitelistSettings(c *gin.Context) {
	var req struct {
		IsEnabled          bool     `json:"isEnabled"`
		EnforceForAdmins   bool     `json:"enforceForAdmins"`
		EnforceForAPI      bool     `json:"enforceForAPI"`
		DefaultAction      string   `json:"defaultAction" binding:"required,oneof=allow deny"`
		AllowInternalIPs   bool     `json:"allowInternalIPs"`
		InternalIPRanges   []string `json:"internalIPRanges"`
		LogBlockedAttempts bool     `json:"logBlockedAttempts"`
		NotifyOnViolation  bool     `json:"notifyOnViolation"`
		NotifyEmail        string   `json:"notifyEmail" binding:"omitempty,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	for _, ipRange := range req.InternalIPRanges {
		if _, _, err := net.ParseCIDR(ipRange); err != nil {
			api_response.Error(c, http.StatusBadRequest, "Invalid internal IP range: "+ipRange)
			return
		}
	}

	var existing models.IPWhitelistSettings
	result := db.DB.First(&existing)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		settings := models.IPWhitelistSettings{
			IsEnabled:          req.IsEnabled,
			EnforceForAdmins:   req.EnforceForAdmins,
			EnforceForAPI:      req.EnforceForAPI,
			DefaultAction:      req.DefaultAction,
			AllowInternalIPs:   req.AllowInternalIPs,
			InternalIPRanges:   req.InternalIPRanges,
			LogBlockedAttempts: req.LogBlockedAttempts,
			NotifyOnViolation:  req.NotifyOnViolation,
			NotifyEmail:        req.NotifyEmail,
		}
		if err := SafeCreate(db.DB, &settings); err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to create settings")
			return
		}
	} else if result.Error != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to load settings")
		return
	} else {
		type whitelistSettingsUpdates struct {
			IsEnabled          *bool     `gorm:"column:is_enabled"`
			EnforceForAdmins   *bool     `gorm:"column:enforce_for_admins"`
			EnforceForAPI      *bool     `gorm:"column:enforce_for_api"`
			DefaultAction      *string   `gorm:"column:default_action"`
			AllowInternalIPs   *bool     `gorm:"column:allow_internal_ips"`
			InternalIPRanges   *[]string `gorm:"column:internal_ip_ranges"`
			LogBlockedAttempts *bool     `gorm:"column:log_blocked_attempts"`
			NotifyOnViolation  *bool     `gorm:"column:notify_on_violation"`
			NotifyEmail        *string   `gorm:"column:notify_email"`
		}
		updates := whitelistSettingsUpdates{
			IsEnabled:          &req.IsEnabled,
			EnforceForAdmins:   &req.EnforceForAdmins,
			EnforceForAPI:      &req.EnforceForAPI,
			DefaultAction:      &req.DefaultAction,
			AllowInternalIPs:   &req.AllowInternalIPs,
			InternalIPRanges:   &req.InternalIPRanges,
			LogBlockedAttempts: &req.LogBlockedAttempts,
			NotifyOnViolation:  &req.NotifyOnViolation,
			NotifyEmail:        &req.NotifyEmail,
		}
		if err := db.DB.Model(&models.IPWhitelistSettings{}).Where(idQuery, existing.ID).
			Updates(&updates).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to update settings")
			return
		}
	}

	middleware.LogCriticalOperation(c, "ip_whitelist_settings_updated", nil)

	api_response.Success(c, gin.H{"message": "Settings updated successfully"})
}

// UpdateIPWhitelistEntry updates fields on an existing whitelist entry (PATCH :id).
func UpdateIPWhitelistEntry(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Description *string `json:"description"`
		Status      *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.Description == nil && req.Status == nil {
		api_response.Error(c, http.StatusBadRequest, "No fields to update")
		return
	}

	var entry models.IPWhitelistEntry
	if err := db.DB.First(&entry, idQuery, id).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Entry not found")
		return
	}

	type entryUpdates struct {
		Description *string `gorm:"column:description"`
		Status      *string `gorm:"column:status"`
	}

	updates := entryUpdates{
		Description: req.Description,
		Status:      req.Status,
	}

	if err := db.DB.Model(&models.IPWhitelistEntry{}).Where(idQuery, entry.ID).
		Updates(&updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to update entry")
		return
	}
	_ = db.DB.First(&entry, idQuery, id)
	middleware.LogCriticalOperation(c, "ip_whitelist_updated", map[string]interface{}{"id": id})
	api_response.Success(c, gin.H{"message": "Entry updated", "data": entry})
}

// BulkAddIPToWhitelist creates multiple whitelist entries in one request.
func BulkAddIPToWhitelist(c *gin.Context) {
	var req struct {
		IPAddresses []string `json:"ipAddresses" binding:"required"`
		Description string   `json:"description,omitempty"`
		Type        string   `json:"type" binding:"required,oneof=admin api webhook"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	adminIDVal, ok := c.Get("userId")
	if !ok {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	adminID, ok := adminIDVal.(string)
	if !ok || adminID == "" {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	tx := db.DB.Begin()
	added := 0
	for _, raw := range req.IPAddresses {
		ip := raw
		if net.ParseIP(ip) == nil {
			tx.Rollback()
			api_response.Error(c, http.StatusBadRequest, "Invalid IP address: "+raw)
			return
		}
		entry := models.IPWhitelistEntry{
			IPAddress:   ip,
			Description: req.Description,
			Type:        req.Type,
			Status:      "active",
			CreatedBy:   adminID,
			CreatedAt:   time.Now(),
		}
		if err := tx.Create(&entry).Error; err != nil {
			tx.Rollback()
			api_response.Error(c, http.StatusInternalServerError, "Failed to add IP: "+ip)
			return
		}
		added++
	}
	if err := tx.Commit().Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to commit bulk add")
		return
	}

	middleware.LogCriticalOperation(c, "ip_whitelist_bulk_added", map[string]interface{}{"count": added})
	api_response.Success(c, gin.H{"message": "IPs added", "added": added})
}

// CheckIPWhitelist reports whether an exact IP exists as an active whitelist entry.
func CheckIPWhitelist(c *gin.Context) {
	ip := c.Query("ip")
	if ip == "" {
		api_response.Error(c, http.StatusBadRequest, "IP required")
		return
	}
	if net.ParseIP(ip) == nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid IP")
		return
	}
	var count int64
	if err := db.DB.Model(&models.IPWhitelistEntry{}).
		Where("ip_address = ? AND status = ?", ip, "active").
		Count(&count).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to check whitelist")
		return
	}
	api_response.Success(c, gin.H{"isWhitelisted": count > 0})
}

// GetBlockedAttempts returns recent blocked IP attempts (if table is populated).
func GetBlockedAttempts(c *gin.Context) {
	var attempts []models.BlockedIPAttempt
	if err := db.DB.Order("attempted_at DESC").Limit(200).Find(&attempts).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch blocked attempts")
		return
	}
	api_response.Success(c, gin.H{"attempts": attempts})
}
