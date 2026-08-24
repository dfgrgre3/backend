package protected

import (
	"fmt"
	"sync"
	models "thanawy-backend/internal/domain/common"
	"time"

	"github.com/gin-gonic/gin"
)

// User and system settings endpoints.
//
// Handlers are split across several files in this package (all sharing
// package protected), grouped by area: this file (shared L1 cache state +
// auth helper), settings_handler_get.go (GetSettings),
// settings_handler_update.go (UpdateSettings), settings_handler_validate.go
// (patch validation), settings_handler_apply.go (patch application) and
// settings_handler_system.go (GetSystemSettings / public site settings).

type userSettingsL1Entry struct {
	settings  models.UserSettings
	expiresAt time.Time
}

var (
	userSettingsL1    sync.Map
	userSettingsL1TTL = 1 * time.Hour
)

func extractUserID(c *gin.Context) (string, error) {
	userID, exists := c.Get("userId")
	if !exists || userID == nil {
		return "", fmt.Errorf("unauthorized")
	}
	return userID.(string), nil
}
