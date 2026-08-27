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

// UpdateSettings updates user settings/preferences
func UpdateSettings(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("CRITICAL: Panic in UpdateSettings: %v", r)
			api_response.Error(c, http.StatusInternalServerError, "Internal server error during settings update")
			c.Abort()
		}
	}()

	userID, exists := c.Get("userId")
	if !exists || userID == nil {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var patch map[string]interface{}
	if err := c.ShouldBindJSON(&patch); err != nil {
		log.Printf("ERROR: UpdateSettings - ShouldBindJSON failed for user %v: %v", userID, err)
		api_response.Error(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validate the incoming patch before doing any work so invalid values are
	// rejected with a clear 400 response instead of being silently ignored or
	// persisted with corrupt data.
	if err := validateSettingsPatch(patch); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("DEBUG: UpdateSettings - User %v patch received: %+v", userID, patch)

	settings, err := fetchOrCreateUserSettings(userID.(string))
	if err != nil {
		log.Printf("ERROR: UpdateSettings - fetchOrCreateUserSettings failed for user %v: %v", userID, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch or create settings: "+err.Error())
		return
	}

	log.Printf("DEBUG: UpdateSettings - Current settings before patch: theme=%v", settings.Theme)

	applySettingsPatch(&settings, patch)

	log.Printf("DEBUG: UpdateSettings - Settings after patch: theme=%v", settings.Theme)

	if err := db.DB.Save(&settings).Error; err != nil {
		log.Printf("ERROR: UpdateSettings - DB.Save failed for user %v: %v", userID, err)
		api_response.Error(c, http.StatusInternalServerError, "Failed to update settings")
		return
	}

	log.Printf("INFO: UpdateSettings - Successfully updated settings for user %v", userID)
	uidStr := userID.(string)
	userSettingsL1.Store(uidStr, &userSettingsL1Entry{settings: settings, expiresAt: time.Now().Add(userSettingsL1TTL)})
	if cache.Redis != nil {
		cacheKey := fmt.Sprintf("user:settings:%s", uidStr)
		if bytes, err := json.Marshal(settings); err == nil {
			cache.Redis.Set(c.Request.Context(), cacheKey, bytes, 1*time.Hour)
		}
	}
	api_response.Success(c, gin.H{"settings": settings})
}

func fetchOrCreateUserSettings(userID string) (models.UserSettings, error) {
	var settings models.UserSettings

	if db.DB == nil {
		return settings, fmt.Errorf("database connection is nil")
	}

	result := db.DB.Where(&models.UserSettings{UserID: userID}).First(&settings)

	if result.Error == nil {
		return settings, nil
	}

	if result.Error != gorm.ErrRecordNotFound {
		log.Printf("ERROR: Failed to fetch settings for user %v: %v", userID, result.Error)
		return settings, result.Error
	}

	log.Printf("INFO: Creating default settings for user %v", userID)
	settings = models.UserSettings{
		UserID:               userID,
		Theme:                "light",
		FontSize:             "medium",
		Language:             "ar",
		NumberFormat:         "english",
		NotificationsEnabled: true,
		StudyReminders:       true,
		EmailNotifications:   true,
		PushNotifications:    true,
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

	// Use RawWriteDB to bypass RLS when creating default settings, matching the
	// read path (createDefaultUserSettings). The app role connection has RLS
	// enabled but lacks INSERT privileges for the UserSettings table, which
	// otherwise fails with SQLSTATE 42501 when a user updates settings before
	// they have been created (e.g. via PATCH /settings/preferences).
	if err := db.RawWriteDB().Clauses(clause.OnConflict{DoNothing: true}).Create(&settings).Error; err != nil {
		log.Printf("ERROR: Failed to create settings for user %v: %v", userID, err)
		return settings, err
	}

	if settings.ID == "" {
		db.DB.Where(&models.UserSettings{UserID: userID}).First(&settings)
	}

	return settings, nil
}
