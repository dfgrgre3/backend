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

func (h *AuthHandler) setAuthTokenCookies(c *gin.Context, accessToken, refreshToken string, rememberMe bool) {
	cfg := config.Load()
	secure := secureCookie(c)

	// Access token cookie (15 minutes expiration)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("access_token", accessToken, 15*60, "/", cfg.CookieDomain, secure, true)

	// Refresh token cookie (30 days default, 90 days if remember me)
	refreshMaxAge := 30 * 24 * 60 * 60
	if rememberMe {
		refreshMaxAge = 90 * 24 * 60 * 60
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("refresh_token", refreshToken, refreshMaxAge, "/", cfg.CookieDomain, secure, true)
}

func (h *AuthHandler) clearAuthCookies(c *gin.Context) {
	cfg := config.Load()
	secure := secureCookie(c)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("access_token", "", -1, "/", cfg.CookieDomain, secure, true)
	c.SetCookie("refresh_token", "", -1, "/", cfg.CookieDomain, secure, true)
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
				"challengeId": ticket,
			})
			return
		}
		if strings.HasPrefix(err.Error(), "ACCOUNT_LOCKED:") {
			// Keep the same "error" string field the frontend already reads
			// (see auth-api-service.ts login()), just with a parseable
			// ACCOUNT_LOCKED:<minutes> prefix so the UI can render a
			// dedicated lockout countdown instead of a generic message.
			response.Error(c, http.StatusTooManyRequests, err.Error())
			return
		}
		response.Error(c, http.StatusUnauthorized, err.Error())
		return
	}

	// Set unified auth cookies
	h.setAuthTokenCookies(c, res.AccessToken, res.RefreshToken, req.RememberMe)

	// SECURITY: the refresh token is intentionally NOT included in the JSON
	// body — it is already delivered as an HttpOnly cookie above, and it is
	// the long-lived, high-value credential (30-90 days) that must never be
	// readable by page JS. The short-lived (15 min) access token is still
	// returned here because the frontend has no other way to authenticate a
	// WebSocket handshake (browsers can't attach cookies/headers to it), but
	// it is now kept only in an in-memory mirror, not localStorage — see
	// token-mirror.ts on the frontend.
	response.Success(c, gin.H{
		"accessToken": res.AccessToken,
		"user":        res.User,
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

	// Clear unified cookies
	h.clearAuthCookies(c)

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
			h.clearAuthCookies(c)
			response.Error(c, http.StatusUnauthorized, "Refresh token is required")
			return
		}
		refreshToken = req.RefreshToken
	}

	userAgent := c.Request.UserAgent()
	ip := c.ClientIP()

	res, err := h.authService.RefreshToken(c.Request.Context(), refreshToken, userAgent, ip)
	if err != nil {
		h.clearAuthCookies(c)
		response.Error(c, http.StatusUnauthorized, err.Error())
		return
	}

	// Set unified cookies
	h.setAuthTokenCookies(c, res.AccessToken, res.RefreshToken, false)

	// See the SECURITY note in Login above — refresh token stays cookie-only.
	response.Success(c, gin.H{
		"accessToken": res.AccessToken,
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
			h.clearAuthCookies(c)
			response.Error(c, http.StatusUnauthorized, "Refresh token is required")
			return
		}
		refreshToken = req.RefreshToken
	}

	userAgent := c.Request.UserAgent()
	ip := c.ClientIP()

	res, err := h.authService.RefreshToken(c.Request.Context(), refreshToken, userAgent, ip)
	if err != nil {
		h.clearAuthCookies(c)
		response.Error(c, http.StatusUnauthorized, err.Error())
		return
	}

	h.setAuthTokenCookies(c, res.AccessToken, res.RefreshToken, false)

	// Get user data cleanly
	userID := c.GetString("userId")
	var user *authdto.UserDTO
	if userID != "" {
		user, _ = h.authService.GetCurrentUser(c.Request.Context(), userID)
	}

	// See the SECURITY note in Login above — refresh token stays cookie-only.
	response.Success(c, gin.H{
		"accessToken": res.AccessToken,
		"user":        user,
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
