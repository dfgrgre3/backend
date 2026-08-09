package protected

import (
	"log"
	"net/http"
	"strings"
	authdto "thanawy-backend/internal/application/dto"
	authservice "thanawy-backend/internal/domain/auth/service"
	"time"

	"thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/config"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService authservice.AuthService
}

func secureCookie(c *gin.Context) bool {
	return c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
}

func NewAuthHandler(authService authservice.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// ─────────────────────────────────────────────
//  Register
// ─────────────────────────────────────────────

func (h *AuthHandler) Register(c *gin.Context) {
	var req authdto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	res, err := h.authService.Register(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Created(c, res)
}

// ─────────────────────────────────────────────
//  Login
// ─────────────────────────────────────────────

func (h *AuthHandler) Login(c *gin.Context) {
	var req authdto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	userAgent := c.Request.UserAgent()
	ip := c.ClientIP()

	res, err := h.authService.Login(c.Request.Context(), &req, userAgent, ip)
	if err != nil {
		if strings.HasPrefix(err.Error(), "MFA_REQUIRED:") {
			ticket := strings.TrimPrefix(err.Error(), "MFA_REQUIRED:")
			response.Success(c, gin.H{
				"mfaRequired": true,
				"ticket":      ticket,
			})
			return
		}
		response.Error(c, http.StatusUnauthorized, err.Error())
		return
	}

	// Set access token cookie
	cfg := config.Load()
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("access_token", res.AccessToken, 15*60, "/", cfg.CookieDomain, secureCookie(c), true)

	// Set refresh token cookie
	refreshMaxAge := 30 * 24 * 60 * 60 // 30 days default
	if req.RememberMe {
		refreshMaxAge = 90 * 24 * 60 * 60 // 90 days if remember me
	}
	c.SetCookie("refresh_token", res.RefreshToken, refreshMaxAge, "/", cfg.CookieDomain, secureCookie(c), true)

	response.Success(c, gin.H{
		"accessToken":  res.AccessToken,
		"refreshToken": res.RefreshToken,
		"user":         res.User,
	})
}

// ─────────────────────────────────────────────
//  Logout
// ─────────────────────────────────────────────

func (h *AuthHandler) Logout(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err == nil && refreshToken != "" {
		if err := h.authService.Logout(c.Request.Context(), refreshToken); err != nil {
			log.Printf("[WARN] Failed to logout user: %v", err)
		}
	}

	// Blacklist current access token JTI
	jtiVal, exists := c.Get("jti")
	if exists {
		if jti, ok := jtiVal.(string); ok && jti != "" {
			var ttl time.Duration = 15 * time.Minute
			expVal, existsExp := c.Get("accessTokenExpiresAt")
			if existsExp {
				if expTime, ok := expVal.(int64); ok {
					remaining := time.Until(time.Unix(expTime, 0))
					if remaining > 0 {
						ttl = remaining
					}
				}
			}
			tokenSvc := authservice.NewAuthTokenService()
			if err := tokenSvc.BlacklistJTI(jti, ttl); err != nil {
				log.Printf("[WARN] Failed to blacklist JTI: %v", err)
			}
		}
	}

	// Clear cookies
	cfg := config.Load()
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("access_token", "", -1, "/", cfg.CookieDomain, secureCookie(c), true)
	c.SetCookie("refresh_token", "", -1, "/", cfg.CookieDomain, secureCookie(c), true)

	response.Success(c, gin.H{"message": "Logged out successfully"})
}

// ─────────────────────────────────────────────
//  Refresh Token
// ─────────────────────────────────────────────

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	// Try to get from cookie first
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		// Try from body
		var req authdto.RefreshTokenRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusUnauthorized, "Refresh token is required")
			return
		}
		refreshToken = req.RefreshToken
	}

	userAgent := c.Request.UserAgent()
	ip := c.ClientIP()

	res, err := h.authService.RefreshToken(c.Request.Context(), refreshToken, userAgent, ip)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, err.Error())
		return
	}

	// Set new cookies
	cfg := config.Load()
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("access_token", res.AccessToken, 15*60, "/", cfg.CookieDomain, secureCookie(c), true)
	c.SetCookie("refresh_token", res.RefreshToken, 30*24*60*60, "/", cfg.CookieDomain, secureCookie(c), true)

	response.Success(c, gin.H{
		"accessToken":  res.AccessToken,
		"refreshToken": res.RefreshToken,
	})
}

// ─────────────────────────────────────────────
//  Refresh Session (Token + Session data)
// ─────────────────────────────────────────────

func (h *AuthHandler) RefreshSession(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		var req authdto.RefreshTokenRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusUnauthorized, "Refresh token is required")
			return
		}
		refreshToken = req.RefreshToken
	}

	userAgent := c.Request.UserAgent()
	ip := c.ClientIP()

	res, err := h.authService.RefreshToken(c.Request.Context(), refreshToken, userAgent, ip)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, err.Error())
		return
	}

	cfg := config.Load()
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("access_token", res.AccessToken, 15*60, "/", cfg.CookieDomain, secureCookie(c), true)
	c.SetCookie("refresh_token", res.RefreshToken, 30*24*60*60, "/", cfg.CookieDomain, secureCookie(c), true)

	// Get user data for refresh session
	userID := c.GetString("userId")
	user, _ := h.authService.GetCurrentUser(c.Request.Context(), userID)

	response.Success(c, gin.H{
		"accessToken":  res.AccessToken,
		"refreshToken": res.RefreshToken,
		"user":         user,
	})
}

// ─────────────────────────────────────────────
//  Get Current User (Me)
// ─────────────────────────────────────────────

func (h *AuthHandler) Me(c *gin.Context) {
	userIDValue, exists := c.Get("userId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	userID, ok := userIDValue.(string)
	if !ok || userID == "" {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	user, err := h.authService.GetCurrentUser(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, err.Error())
		return
	}

	response.Success(c, gin.H{"user": user})
}

// ─────────────────────────────────────────────
//  Forgot Password
// ─────────────────────────────────────────────

func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req authdto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if err := h.authService.ForgotPassword(c.Request.Context(), req.Email); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, gin.H{
		"message": "If an account with this email exists, a verification code has been sent.",
	})
}

// ─────────────────────────────────────────────
//  Verify Forgot Password Code
// ─────────────────────────────────────────────

func (h *AuthHandler) VerifyForgotPasswordCode(c *gin.Context) {
	var req authdto.VerifyForgotPasswordCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	resetToken, err := h.authService.VerifyForgotPasswordCode(c.Request.Context(), req.Email, req.Code)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, authdto.VerifyForgotPasswordCodeResponse{
		ResetToken: resetToken,
		Message:    "Code verified successfully. You can now reset your password.",
	})
}

// ─────────────────────────────────────────────
//  Reset Password
// ─────────────────────────────────────────────

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req authdto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if err := h.authService.ResetPassword(c.Request.Context(), req.Token, req.NewPassword); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Password reset successfully. Please login with your new password."})
}

// ─────────────────────────────────────────────
//  Verify Email
// ─────────────────────────────────────────────

func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	userIDValue, exists := c.Get("userId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID := userIDValue.(string)

	var req authdto.VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if err := h.authService.VerifyEmail(c.Request.Context(), userID, req.Code); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Email verified successfully"})
}

// ─────────────────────────────────────────────
//  Resend Verification Email
// ─────────────────────────────────────────────

func (h *AuthHandler) ResendVerification(c *gin.Context) {
	userIDValue, exists := c.Get("userId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID := userIDValue.(string)

	var req authdto.ResendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// No body, use user ID
	}

	if err := h.authService.ResendVerificationEmail(c.Request.Context(), userID); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Verification code sent successfully"})
}

// ─────────────────────────────────────────────
//  Change Password
// ─────────────────────────────────────────────

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req authdto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if err := h.authService.ChangePassword(c.Request.Context(), userID.(string), req.OldPassword, req.NewPassword); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Clear cookies to force re-login
	cfg := config.Load()
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("access_token", "", -1, "/", cfg.CookieDomain, secureCookie(c), true)
	c.SetCookie("refresh_token", "", -1, "/", cfg.CookieDomain, secureCookie(c), true)

	response.Success(c, gin.H{"message": "Password changed successfully. Please login again."})
}

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
//  Social Login (Provider redirect)
// ─────────────────────────────────────────────

func (h *AuthHandler) SocialLogin(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		response.Error(c, http.StatusBadRequest, "Provider is required")
		return
	}

	redirectURL, err := h.authService.GetOAuthRedirectURL(c.Request.Context(), provider)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{"redirectUrl": redirectURL})
}

// ─────────────────────────────────────────────
//  OAuth Callback
// ─────────────────────────────────────────────

func (h *AuthHandler) OAuthCallback(c *gin.Context) {
	provider := c.Param("provider")
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		response.Error(c, http.StatusBadRequest, "Missing code or state parameter")
		return
	}

	userAgent := c.Request.UserAgent()
	ip := c.ClientIP()

	res, err := h.authService.HandleOAuthCallback(c.Request.Context(), provider, code, state, userAgent, ip)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, err.Error())
		return
	}

	cfg := config.Load()
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("access_token", res.AccessToken, 15*60, "/", cfg.CookieDomain, secureCookie(c), true)
	c.SetCookie("refresh_token", res.RefreshToken, 30*24*60*60, "/", cfg.CookieDomain, secureCookie(c), true)

	response.Success(c, gin.H{
		"accessToken":  res.AccessToken,
		"refreshToken": res.RefreshToken,
		"user":         res.User,
	})
}

// ─────────────────────────────────────────────
//  Link OAuth Provider
// ─────────────────────────────────────────────

func (h *AuthHandler) LinkProvider(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req authdto.LinkProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if err := h.authService.LinkOAuthProvider(c.Request.Context(), userID.(string), req.Provider, req.Code, req.State); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Provider linked successfully"})
}

// ─────────────────────────────────────────────
//  Unlink OAuth Provider
// ─────────────────────────────────────────────

func (h *AuthHandler) UnlinkProvider(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req authdto.UnlinkProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if err := h.authService.UnlinkOAuthProvider(c.Request.Context(), userID.(string), req.Provider); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Provider unlinked successfully"})
}

// ─────────────────────────────────────────────
//  Get Linked OAuth Accounts
// ─────────────────────────────────────────────

func (h *AuthHandler) GetLinkedAccounts(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}

	accounts, err := h.authService.GetLinkedAccounts(c.Request.Context(), userID.(string))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, gin.H{"accounts": accounts})
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

// ─────────────────────────────────────────────
//  Account Recovery
// ─────────────────────────────────────────────

func (h *AuthHandler) AccountRecovery(c *gin.Context) {
	var req authdto.AccountRecoveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	ticket, err := h.authService.InitiateAccountRecovery(c.Request.Context(), req.Email, req.Method)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, authdto.AccountRecoveryResponse{
		Ticket:  ticket,
		Message: "Recovery initiated. Check your email/phone for verification code.",
	})
}

// ─────────────────────────────────────────────
//  Recover Account (Finalize)
// ─────────────────────────────────────────────

func (h *AuthHandler) RecoverAccount(c *gin.Context) {
	var req authdto.RecoverAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if err := h.authService.FinalizeAccountRecovery(c.Request.Context(), req.Ticket, req.Code, req.NewPassword); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Account recovered successfully. Please login with your new password."})
}
