package protected

import (
	"fmt"
	"net/http"
	analyticsservice "thanawy-backend/internal/domain/analytics/service"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/api/middleware"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/config"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func ImpersonateUser(c *gin.Context) {
	var req struct {
		TargetUserID string `json:"targetUserId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid request")
		return
	}

	// Verify target user exists
	var user models.User
	if err := db.DB.First(&user, "id = ?", req.TargetUserID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "User not found")
		return
	}

	// Enforce role hierarchy: Admin/Super-Admin cannot impersonate equal or higher rank users
	adminID, existsAdmin := c.Get("userId")
	if !existsAdmin || adminID == nil {
		api_response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	adminRole, existsRole := c.Get("role")
	adminRoleStr, _ := adminRole.(string)
	if !existsRole || adminRoleStr == "" {
		adminRoleStr = "ADMIN" // Default fallback
	}

	roleHierarchy := map[string]int{
		"STUDENT":     1,
		"TEACHER":     2,
		"MODERATOR":   3,
		"ADMIN":       4,
		"SUPER_ADMIN": 5,
	}

	targetRoleStr := string(user.Role)
	if roleHierarchy[adminRoleStr] <= roleHierarchy[targetRoleStr] {
		api_response.Error(c, http.StatusForbidden, "Cannot impersonate a user of equal or higher administrative rank")
		return
	}

	// Generate securely signed token for cookie to prevent tampering
	signedToken := middleware.SignImpersonationToken(req.TargetUserID, adminID.(string))
	csrfToken := middleware.SignImpersonationCSRFToken(adminID.(string))

	// Set impersonation cookie with maximum security:
	// HttpOnly=true — prevents JavaScript access (XSS protection)
	// Secure=true — only sent over HTTPS
	// SameSite=Strict — prevents CSRF attacks (never sent for cross-site requests)
	// CSRF cookie is NOT HttpOnly so the frontend can read and send it as a header
	cfg := config.Load()
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("impersonate_user_id", signedToken, 3600, "/", cfg.CookieDomain, true, true)
	c.SetCookie("impersonate_csrf_token", csrfToken, 3600, "/", cfg.CookieDomain, true, false)

	// Force an immediate audit log entry for security tracking
	analyticsservice.GetAuditService().LogAsync(
		adminID.(string),
		analyticsservice.AuditEventImpersonationStart,
		"user",
		req.TargetUserID,
		map[string]interface{}{
			"action":         "impersonation_start",
			"target_user_id": req.TargetUserID,
			"target_email":   user.Email,
			"admin_role":     adminRoleStr,
			"target_role":    targetRoleStr,
		},
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	api_response.Success(c, gin.H{
		"success": true,
		"message": fmt.Sprintf("أنت الآن تنتحل شخصية %s", user.Email),
	})
}
