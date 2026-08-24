package protected

import (
	"net/http"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/api/middleware"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
)

func BanUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "User ID is required")
		return
	}

	var req struct {
		Reason        *string `json:"reason"`
		DurationHours *int    `json:"durationHours"`
		NotifyUser    bool    `json:"notifyUser"`
		Permanent     bool    `json:"permanent"`
		ExpiresAt     *string `json:"expiresAt"`
		HideContent   bool    `json:"hideContent"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var user models.User
	if err := db.DB.Where(idQuery, userID).First(&user).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	// Set status to BANNED
	user.Status = models.StatusBanned

	// Set expiration if not permanent
	if !req.Permanent && req.ExpiresAt != nil {
		expiresAt, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err == nil {
			user.StatusExpiresAt = &expiresAt
		}
	} else if req.DurationHours != nil {
		expiresAt := time.Now().Add(time.Duration(*req.DurationHours) * time.Hour)
		user.StatusExpiresAt = &expiresAt
	}

	if req.Reason != nil {
		user.StatusReason = req.Reason
	}

	if err := db.DB.Save(&user).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to ban user")
		return
	}

	middleware.InvalidateRolePermsCache(userID)
	getUserRepo().InvalidateCache(userID)

	LogAudit(c, "BAN_USER", "user", userID, gin.H{"reason": req.Reason, "permanent": req.Permanent})
	api_response.Success(c, buildUserDetailsPayload(user))
}

// SuspendUser suspends a user temporarily
func SuspendUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "User ID is required")
		return
	}

	var req struct {
		Reason        *string `json:"reason"`
		DurationHours *int    `json:"durationHours"`
		NotifyUser    bool    `json:"notifyUser"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	var user models.User
	if err := db.DB.Where(idQuery, userID).First(&user).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	// Set status to SUSPENDED
	user.Status = models.StatusSuspended

	// Set expiration
	if req.DurationHours != nil {
		expiresAt := time.Now().Add(time.Duration(*req.DurationHours) * time.Hour)
		user.StatusExpiresAt = &expiresAt
	}

	if req.Reason != nil {
		user.StatusReason = req.Reason
	}

	if err := db.DB.Save(&user).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to suspend user")
		return
	}

	middleware.InvalidateRolePermsCache(userID)
	getUserRepo().InvalidateCache(user.ID)

	LogAudit(c, "SUSPEND_USER", "user", userID, gin.H{"reason": req.Reason, "durationHours": req.DurationHours})
	api_response.Success(c, buildUserDetailsPayload(user))
}

// ActivateUser activates a suspended or banned user
func ActivateUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "User ID is required")
		return
	}

	var user models.User
	if err := db.DB.Where(idQuery, userID).First(&user).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	user.Status = models.StatusActive
	user.StatusReason = nil
	user.StatusExpiresAt = nil

	if err := db.DB.Save(&user).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to activate user")
		return
	}

	middleware.InvalidateRolePermsCache(userID)
	getUserRepo().InvalidateCache(user.ID)

	LogAudit(c, "ACTIVATE_USER", "user", userID, nil)
	api_response.Success(c, buildUserDetailsPayload(user))
}

// RestoreUser restores a soft-deleted user.
func RestoreUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}
	if err := db.DB.Model(&models.User{}).Unscoped().Where("id = ?", userID).Update("deleted_at", nil).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to restore user")
		return
	}
	middleware.InvalidateRolePermsCache(userID)
	getUserRepo().InvalidateCache(userID)
	LogAudit(c, "RESTORE", "user", userID, nil)
	api_response.Success(c, gin.H{"success": true})
}
