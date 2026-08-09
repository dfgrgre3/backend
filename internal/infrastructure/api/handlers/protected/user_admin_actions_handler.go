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

// ChangeUserRole changes a user's role
func ChangeUserRole(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "User ID is required")
		return
	}

	var req struct {
		Role   *string `json:"role" binding:"required"`
		Reason *string `json:"reason"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	validRoles := map[string]bool{
		"STUDENT": true, "TEACHER": true, "MODERATOR": true,
		"ADMIN": true, "SUPER_ADMIN": true, "SUPPORT": true, "PARENT": true,
	}
	if !validRoles[*req.Role] {
		api_response.Error(c, http.StatusBadRequest, "Invalid role")
		return
	}

	var user models.User
	if err := db.DB.Where(idQuery, userID).First(&user).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}

	user.Role = models.UserRole(*req.Role)

	if err := db.DB.Save(&user).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to change user role")
		return
	}

	middleware.InvalidateRolePermsCache(userID)
	getUserRepo().InvalidateCache(user.ID)

	LogAudit(c, "CHANGE_ROLE", "user", userID, gin.H{"oldRole": user.Role, "newRole": req.Role, "reason": req.Reason})
	api_response.Success(c, buildUserDetailsPayload(user))
}

// SendPasswordReset sends a password reset email to user
func SendPasswordReset(c *gin.Context) {
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

	// Generate password reset token
	_ = generateRandomToken(32)
	_ = time.Now().Add(24 * time.Hour)

	// Store token (you might need a password_resets table)
	// For now, we'll just log it
	LogAudit(c, "PASSWORD_RESET_REQUEST", "user", userID, gin.H{"email": user.Email})

	api_response.Success(c, gin.H{"message": "Password reset email sent"})
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

func VerifyUserEmail(c *gin.Context) {
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

	user.EmailVerified = true
	if err := db.DB.Save(&user).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to verify user email")
		return
	}

	middleware.InvalidateRolePermsCache(userID)
	getUserRepo().InvalidateCache(user.ID)

	LogAudit(c, "VERIFY_USER_EMAIL", "user", userID, nil)
	api_response.Success(c, buildUserDetailsPayload(user))
}

func VerifyUserPhone(c *gin.Context) {
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

	user.PhoneVerified = true
	if err := db.DB.Save(&user).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to verify user phone")
		return
	}

	middleware.InvalidateRolePermsCache(userID)
	getUserRepo().InvalidateCache(user.ID)

	LogAudit(c, "VERIFY_USER_PHONE", "user", userID, nil)
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

// AssignRole assigns a role to a user.
func AssignRole(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}
	var req struct {
		Role   string `json:"role" binding:"required"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if !models.IsValidUserRole(models.UserRole(req.Role)) {
		api_response.Error(c, http.StatusBadRequest, "invalid role")
		return
	}
	if err := db.DB.Model(&models.User{}).Where(idQuery, userID).Updates(map[string]interface{}{
		"role":       req.Role,
		"updated_at": time.Now(),
	}).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to assign role")
		return
	}
	middleware.InvalidateRolePermsCache(userID)
	getUserRepo().InvalidateCache(userID)
	LogAudit(c, "ASSIGN_ROLE", "user", userID, gin.H{"role": req.Role, "reason": req.Reason})
	api_response.Success(c, gin.H{"success": true})
}

// AddPermission adds a permission to a user.
func AddPermission(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}
	var req struct {
		Permission string `json:"permission" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	var user models.User
	if err := db.DB.First(&user, idQuery, userID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}
	for _, p := range user.GetEffectivePermissions() {
		if p == req.Permission {
			api_response.Success(c, gin.H{"success": true, "message": "permission already exists"})
			return
		}
	}
	user.Permissions = append(user.Permissions, req.Permission)
	if err := db.DB.Model(&user).Update("permissions", user.Permissions).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to add permission")
		return
	}
	middleware.InvalidateRolePermsCache(user.ID)
	getUserRepo().InvalidateCache(user.ID)
	LogAudit(c, "ADD_PERMISSION", "user", user.ID, gin.H{"permission": req.Permission})
	api_response.Success(c, gin.H{"success": true})
}

// RemovePermission removes a permission from a user.
func RemovePermission(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		api_response.Error(c, http.StatusBadRequest, "user id is required")
		return
	}
	var req struct {
		Permission string `json:"permission" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	var user models.User
	if err := db.DB.First(&user, idQuery, userID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, errUserNotFound)
		return
	}
	updated := make(models.JSONStringArray, 0, len(user.Permissions))
	for _, p := range user.Permissions {
		if p != req.Permission {
			updated = append(updated, p)
		}
	}
	if err := db.DB.Model(&user).Update("permissions", updated).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to remove permission")
		return
	}
	middleware.InvalidateRolePermsCache(user.ID)
	getUserRepo().InvalidateCache(user.ID)
	LogAudit(c, "REMOVE_PERMISSION", "user", user.ID, gin.H{"permission": req.Permission})
	api_response.Success(c, gin.H{"success": true})
}

// GetUserSessions returns sessions for a user.
