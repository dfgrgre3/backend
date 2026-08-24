package protected

import (
	"encoding/json"
	"log"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetSystemSettings retrieves public system settings (feature toggles, etc)
func GetSystemSettings(c *gin.Context) {
	// Initialize defaults outside the closure so they are accessible to recover()
	defaultSettings := buildDefaultSystemSettings()

	defer func() {
		if r := recover(); r != nil {
			log.Printf("CRITICAL: Panic in GetSystemSettings: %v", r)
			api_response.Success(c, gin.H{"settings": defaultSettings})
			c.Abort()
		}
	}()

	// Safe DB access
	if db.DB == nil {
		log.Printf("WARN: Database connection is not initialized in GetSystemSettings, returning defaults")
		api_response.Success(c, gin.H{"settings": defaultSettings})
		return
	}

	settings := fetchSystemSettings(db.DB, defaultSettings)

	publicSettings := extractPublicSettings(settings, defaultSettings)
	api_response.Success(c, gin.H{"settings": publicSettings})
}

func buildDefaultSystemSettings() map[string]interface{} {
	return map[string]interface{}{
		"siteName":        "Thanawy",
		"siteDescription": "منصة تعليمية لإدارة التعلم والمحتوى.",
		"features": map[string]interface{}{
			"registration": true,
			"engagement":   true,
			"forum":        true,
			"blog":         true,
			"events":       true,
			"aiAssistant":  true,
		},
		"maintenance": map[string]interface{}{
			"enabled": false,
			"message": "",
		},
	}
}

func InvalidateSettingsCache() {
	cache.InvalidateSettingsCache()
}

func fetchSystemSettings(database *gorm.DB, defaultSettings map[string]interface{}) map[string]interface{} {
	if cached, ok := cache.CachedSettings(); ok {
		return cached
	}

	// Use the read replica when available to avoid loading the write source.
	// The SystemSetting table is rarely updated, so reading from a replica
	// is safe and reduces write-source contention.
	readDB := db.ReadDB()
	if readDB == nil {
		readDB = database
	}

	var dbSetting models.SystemSetting

	err := readDB.Where("key = ?", "admin_settings").Take(&dbSetting).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			log.Printf("ERROR: Failed to fetch admin_settings from DB: %v. Using defaults.", err)
		}
		cache.StoreSettingsCache(defaultSettings)
		return defaultSettings
	}

	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(dbSetting.Value), &settings); err != nil || settings == nil {
		log.Printf("WARN: Failed to unmarshal admin_settings from DB: %v. Using defaults.", err)
		cache.StoreSettingsCache(defaultSettings)
		return defaultSettings
	}

	// Double safety check
	if settings == nil {
		cache.StoreSettingsCache(defaultSettings)
		return defaultSettings
	}

	cache.StoreSettingsCache(settings)

	return settings
}

func extractPublicSettings(settings, defaultSettings map[string]interface{}) gin.H {
	return gin.H{
		"siteName":        extractString(settings, "siteName", extractString(defaultSettings, "siteName", "Thanawy")),
		"siteDescription": extractString(settings, "siteDescription", extractString(defaultSettings, "siteDescription", "")),
		"features":        extractMap(settings, "features", extractMap(defaultSettings, "features", map[string]interface{}{})),
		"maintenance":     extractMap(settings, "maintenance", extractMap(defaultSettings, "maintenance", map[string]interface{}{})),
	}
}

// Helper to safely extract string from map
func extractString(m map[string]interface{}, key string, fallback string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return fallback
}

// Helper to safely extract map from map
func extractMap(m map[string]interface{}, key string, fallback map[string]interface{}) map[string]interface{} {
	if val, ok := m[key]; ok {
		if res, ok := val.(map[string]interface{}); ok {
			return res
		}
	}
	return fallback
}
