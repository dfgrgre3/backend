package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// PrivacyActionRequest is the body for POST /api/settings/privacy/actions.
type PrivacyActionRequest struct {
	Action string `json:"action" binding:"required,oneof=export-data clear-history"`
}

// PrivacyActions performs a self-service privacy action for the
// authenticated user only - never accepts a client-supplied user id.
func PrivacyActions(c *gin.Context) {
	uid, err := extractUserID(c)
	if err != nil {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req PrivacyActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	switch req.Action {
	case "export-data":
		exportUserData(c, uid)
	case "clear-history":
		clearUserHistory(c, uid)
	}
}

func exportUserData(c *gin.Context, uid string) {
	var user models.User
	if err := db.DB.Where("id = ?", uid).Take(&user).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to export data")
		return
	}

	var settings models.UserSettings
	db.DB.Where("user_id = ?", uid).Take(&settings)

	logs, err := getSecurityLogRepo().FindByUserID(uid, 0)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to export data")
		return
	}

	api_response.Success(c, gin.H{
		"exportData": gin.H{
			"profile":      user,
			"preferences":  settings,
			"securityLogs": logs,
		},
	})
}

func clearUserHistory(c *gin.Context, uid string) {
	deletedCount, err := getSecurityLogRepo().DeleteByUserID(uid)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to clear history")
		return
	}

	api_response.Success(c, gin.H{"deletedCount": deletedCount})
}
