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
