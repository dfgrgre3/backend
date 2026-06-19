package handlers

import (
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch whitelist"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"entries": entries,
		},
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add IP"})
		return
	}

	middleware.LogCriticalOperation(c, "ip_whitelist_added", map[string]interface{}{
		"ip":   req.IPAddress,
		"type": req.Type,
	})

	c.JSON(http.StatusCreated, gin.H{
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
		c.JSON(http.StatusNotFound, gin.H{"error": "Entry not found"})
		return
	}

	if err := db.DB.Delete(&entry).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove IP"})
		return
	}

	middleware.LogCriticalOperation(c, "ip_whitelist_removed", map[string]interface{}{
		"ip": entry.IPAddress,
	})

	c.JSON(http.StatusOK, gin.H{"message": "IP removed from whitelist"})
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
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"isEnabled":          false,
				"enforceForAdmins":   false,
				"enforceForAPI":      false,
				"defaultAction":      "allow",
				"allowInternalIPs":   true,
				"internalIPRanges":   config.GlobalConfig.InternalIPRanges,
				"logBlockedAttempts": true,
				"notifyOnViolation":  true,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": settings})
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
	var req models.IPWhitelistSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var existing models.IPWhitelistSettings
	if err := db.DB.First(&existing).Error; err != nil {
		// Create new
		req.ID = "default"
		SafeCreate(db.DB, &req)
	} else {
		type whitelistSettingsUpdates struct {
			IsEnabled          *bool   `gorm:"column:is_enabled"`
			EnforceForAdmins   *bool   `gorm:"column:enforce_for_admins"`
			EnforceForAPI      *bool   `gorm:"column:enforce_for_api"`
			DefaultAction      *string `gorm:"column:default_action"`
			AllowInternalIPs   *bool   `gorm:"column:allow_internal_ips"`
			LogBlockedAttempts *bool   `gorm:"column:log_blocked_attempts"`
			NotifyOnViolation  *bool   `gorm:"column:notify_on_violation"`
		}
		updates := whitelistSettingsUpdates{
			IsEnabled:          &req.IsEnabled,
			EnforceForAdmins:   &req.EnforceForAdmins,
			EnforceForAPI:      &req.EnforceForAPI,
			DefaultAction:      &req.DefaultAction,
			AllowInternalIPs:   &req.AllowInternalIPs,
			LogBlockedAttempts: &req.LogBlockedAttempts,
			NotifyOnViolation:  &req.NotifyOnViolation,
		}
		db.DB.Model(&models.IPWhitelistSettings{}).Where(idQuery, existing.ID).
			Updates(&updates)
	}

	middleware.LogCriticalOperation(c, "ip_whitelist_settings_updated", nil)

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated successfully"})
}

// UpdateIPWhitelistEntry updates fields on an existing whitelist entry (PATCH :id).
func UpdateIPWhitelistEntry(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Description *string `json:"description"`
		Status      *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Description == nil && req.Status == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	var entry models.IPWhitelistEntry
	if err := db.DB.First(&entry, idQuery, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Entry not found"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update entry"})
		return
	}
	_ = db.DB.First(&entry, idQuery, id)
	middleware.LogCriticalOperation(c, "ip_whitelist_updated", map[string]interface{}{"id": id})
	c.JSON(http.StatusOK, gin.H{"message": "Entry updated", "data": entry})
}

// BulkAddIPToWhitelist creates multiple whitelist entries in one request.
func BulkAddIPToWhitelist(c *gin.Context) {
	var req struct {
		IPAddresses []string `json:"ipAddresses" binding:"required"`
		Description string   `json:"description,omitempty"`
		Type        string   `json:"type" binding:"required,oneof=admin api webhook"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminIDVal, ok := c.Get("userId")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	adminID, ok := adminIDVal.(string)
	if !ok || adminID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	tx := db.DB.Begin()
	added := 0
	for _, raw := range req.IPAddresses {
		ip := raw
		if net.ParseIP(ip) == nil {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid IP address: " + raw})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add IP: " + ip})
			return
		}
		added++
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit bulk add"})
		return
	}

	middleware.LogCriticalOperation(c, "ip_whitelist_bulk_added", map[string]interface{}{"count": added})
	c.JSON(http.StatusCreated, gin.H{"message": "IPs added", "added": added})
}

// CheckIPWhitelist reports whether an exact IP exists as an active whitelist entry.
func CheckIPWhitelist(c *gin.Context) {
	ip := c.Query("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IP required"})
		return
	}
	if net.ParseIP(ip) == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid IP"})
		return
	}
	var count int64
	if err := db.DB.Model(&models.IPWhitelistEntry{}).
		Where("ip_address = ? AND status = ?", ip, "active").
		Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check whitelist"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"isWhitelisted": count > 0})
}

// GetBlockedAttempts returns recent blocked IP attempts (if table is populated).
func GetBlockedAttempts(c *gin.Context) {
	var attempts []models.BlockedIPAttempt
	if err := db.DB.Order("attempted_at DESC").Limit(200).Find(&attempts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch blocked attempts"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"attempts": attempts}})
}
