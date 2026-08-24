package admin

import (
	"encoding/json"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

var defaultAdminSettings = map[string]interface{}{
	"siteName":        "Thanawy",
	"siteDescription": "منصة تعليمية لإدارة التعلم والمحتوى.",
	"siteKeywords":    []string{"education", "thanawy"},
	"contactEmail":    "admin@thanawy.local",
	"supportPhone":    "",
	"socialLinks": map[string]interface{}{
		"facebook":  "",
		"twitter":   "",
		"instagram": "",
		"youtube":   "",
	},
	"features": map[string]interface{}{
		"registration":      true,
		"emailVerification": true,
		"engagement":        true,
		"forum":             true,
		"blog":              true,
		"events":            true,
		"aiAssistant":       true,
	},
	"engagement": map[string]interface{}{
		"pointsPerTask":         10,
		"pointsPerStudySession": 5,
		"pointsPerExam":         20,
		"streakBonus":           2,
	},
	"limits": map[string]interface{}{
		"maxUploadSize":           10,
		"maxStudySessionDuration": 180,
		"examTimeLimit":           60,
	},
	"maintenance": map[string]interface{}{
		"enabled": false,
		"message": "",
	},
}

func requestBodyOrEmpty(c *gin.Context) gin.H {
	var body gin.H
	if err := c.ShouldBindJSON(&body); err != nil {
		return gin.H{}
	}
	return body
}

func mergeMaps(dest map[string]interface{}, src map[string]interface{}) {
	for k, v := range src {
		dest[k] = v
	}
}

func AdminSettings(c *gin.Context) {
	var dbSetting models.SystemSetting
	settings := make(map[string]interface{})

	// Initialize with defaults
	mergeMaps(settings, defaultAdminSettings)

	// Overlay settings from database
	if db.DB != nil {
		if err := db.DB.Where("key = ?", "admin_settings").Take(&dbSetting).Error; err == nil {
			var dbMap map[string]interface{}
			if err := json.Unmarshal([]byte(dbSetting.Value), &dbMap); err == nil {
				mergeMaps(settings, dbMap)
			}
		}
	}

	// Process updates if applicable
	method := c.Request.Method
	if method == http.MethodPatch || method == http.MethodPut {
		mergeMaps(settings, requestBodyOrEmpty(c))

		jsonData, _ := json.Marshal(settings)
		dbSetting.Key = "admin_settings"
		dbSetting.Value = string(jsonData)

		if err := db.DB.Save(&dbSetting).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to save settings")
			return
		}
		InvalidateSettingsCache()
	}

	api_response.Success(c, gin.H{"settings": settings})
}
