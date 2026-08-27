package protected

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetSettings retrieves user settings/preferences
func GetSettings(c *gin.Context) {
	uid, err := extractUserID(c)
	if err != nil {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	_, aborted := safeDB(c)
	if aborted {
		return
	}

	if trySettingsL1Cache(c, uid) {
		return
	}

	// Try Redis L2 Cache (1 hour TTL)
	cacheKey := fmt.Sprintf("user:settings:%s", uid)
	if cache.Redis != nil {
		cachedData, err := cache.Redis.Get(c.Request.Context(), cacheKey).Result()
		if err == nil {
			var cachedSettings models.UserSettings
			if json.Unmarshal([]byte(cachedData), &cachedSettings) == nil {
				userSettingsL1.Store(uid, &userSettingsL1Entry{settings: cachedSettings, expiresAt: time.Now().Add(userSettingsL1TTL)})
				api_response.Success(c, gin.H{"settings": cachedSettings})
				return
			}
		}
	}

	settings, ok := fetchOrCreateSettingsForGet(c, uid)
	if !ok {
		return
	}

	userSettingsL1.Store(uid, &userSettingsL1Entry{settings: settings, expiresAt: time.Now().Add(userSettingsL1TTL)})
	if cache.Redis != nil {
		if bytes, err := json.Marshal(settings); err == nil {
			cache.Redis.Set(c.Request.Context(), cacheKey, bytes, 1*time.Hour)
		}
	}
	api_response.Success(c, gin.H{"settings": settings})
}

func trySettingsL1Cache(c *gin.Context, uid string) bool {
	if raw, ok := userSettingsL1.Load(uid); ok {
		entry := raw.(*userSettingsL1Entry)
		if time.Now().Before(entry.expiresAt) {
			api_response.Success(c, gin.H{"settings": entry.settings})
			return true
		}
		userSettingsL1.Delete(uid)
	}
	return false
}

func fetchOrCreateSettingsForGet(c *gin.Context, uid string) (models.UserSettings, bool) {
	// Use the write DB directly to avoid double-query in the fallback path.
	// ReadDB may be nil or slow, and fallback to write DB adds latency.
	var settings models.UserSettings
	result := db.DB.Where("user_id = ?", uid).Take(&settings)

	if result.Error != nil {
		if handleSettingsFetchError(c, uid, result) {
			return settings, false
		}
	}
	return settings, true
}

// handleSettingsFetchError processes the error from fetching settings.
// Returns true if the caller should return (error already written to response).
func handleSettingsFetchError(c *gin.Context, uid string, result *gorm.DB) bool {
	if result.Error != gorm.ErrRecordNotFound {
		log.Printf("ERROR: Failed to fetch settings for user %v: %v", uid, result.Error)
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch settings")
		return true
	}

	settings, err := createDefaultUserSettings(c, uid)
	if err != nil {
		return true
	}

	userSettingsL1.Store(uid, &userSettingsL1Entry{settings: settings, expiresAt: time.Now().Add(userSettingsL1TTL)})
	api_response.Success(c, gin.H{"settings": settings})
	return true
}

func createDefaultUserSettings(c *gin.Context, uid string) (models.UserSettings, error) {
	log.Printf("INFO: Creating default settings for user %v", uid)
	// Create default settings for user
	settings := models.UserSettings{
		UserID:               uid,
		Theme:                "light",
		FontSize:             "medium",
		ReducedMotion:        false,
		HighContrast:         false,
		CompactMode:          false,
		EfficiencyMode:       false,
		Language:             "ar",
		NumberFormat:         "english",
		NotificationsEnabled: true,
		StudyReminders:       true,
		EmailNotifications:   true,
		PushNotifications:    true,
		TaskReminders:        true,
		TaskReminderTime:     "30",
		DailyGoalReminders:   true,
		ExamReminders:        true,
		ExamReminderDays:     3,
		DeadlineReminders:    true,
		ProgressReports:      true,
		WeeklyReport:         true,
		AchievementAlerts:    true,
		CommentNotifications: true,
		MentionNotifications: true,
		PushEnabled:          true,
		EmailEnabled:         true,
		SmsEnabled:           false,
		QuietHoursEnabled:    false,
		QuietHoursStart:      "22:00",
		QuietHoursEnd:        "07:00",
		SoundEnabled:         true,
		VibrationEnabled:     true,
		ProfileVisibility:    "public",
		ShowOnlineStatus:     true,
		ShowProgress:         true,
		ShowLastSeen:         true,
		ShowAchievements:     true,
		AllowMessages:        "everyone",
		AllowFriendRequests:  true,
		DataCollection:       true,
		Personalization:      true,
		Analytics:            true,
	}

	// Use RawWriteDB to bypass RLS when creating default settings
	// This is necessary because the app role connection has RLS enabled,
	// but we need to insert system-level default settings for users
	if err := db.RawWriteDB(c.Request.Context()).Clauses(clause.OnConflict{DoNothing: true}).Create(&settings).Error; err != nil {
		log.Printf("ERROR: Failed to create settings for user %v: %v", uid, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to create settings")
		return settings, err
	}

	// Re-fetch to ensure we have the settings if DoNothing was triggered
	if settings.ID == "" {
		db.DB.Where(&models.UserSettings{UserID: uid}).First(&settings)
	}

	return settings, nil
}
