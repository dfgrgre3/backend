package protected

import (
	"net/http"
	authdto "thanawy-backend/internal/application/dto"
	"thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/config"

	"github.com/gin-gonic/gin"
)

// Profile and token-validation endpoints: update profile, delete account,
// and validate-token. Split out of auth_handler.go for readability — same
// package, same *AuthHandler receiver, no behavior change.

// ─────────────────────────────────────────────
//  Update Profile
// ─────────────────────────────────────────────

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req authdto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	user, err := h.authService.UpdateProfile(c.Request.Context(), userID.(string), &req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Profile updated successfully", "user": user})
}

// ─────────────────────────────────────────────
//  Delete Account
// ─────────────────────────────────────────────

func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req authdto.DeleteAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if err := h.authService.DeleteAccount(c.Request.Context(), userID.(string), req.Password, req.Reason); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Clear cookies
	cfg := config.Load()
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("access_token", "", -1, "/", cfg.CookieDomain, secureCookie(c), true)
	c.SetCookie("refresh_token", "", -1, "/", cfg.CookieDomain, secureCookie(c), true)

	response.Success(c, gin.H{"message": "Account deleted successfully"})
}

// ─────────────────────────────────────────────
//  Token Validation
// ─────────────────────────────────────────────

func (h *AuthHandler) ValidateToken(c *gin.Context) {
	var req authdto.ValidateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	claims, err := h.authService.ValidateAccessToken(c.Request.Context(), req.Token)
	if err != nil {
		response.Success(c, authdto.ValidateTokenResponse{Valid: false})
		return
	}

	response.Success(c, authdto.ValidateTokenResponse{
		Valid:       true,
		UserID:      claims.UserID,
		Role:        claims.Role,
		Permissions: claims.Permissions,
		ExpiresAt:   claims.ExpiresAt,
	})
}
