package protected

import (
	"net/http"
	authdto "thanawy-backend/internal/application/dto"
	"thanawy-backend/internal/infrastructure/api/response"
	"thanawy-backend/internal/infrastructure/config"

	"github.com/gin-gonic/gin"
)

// Social sign-in (OAuth) endpoints: provider redirect, callback,
// link/unlink provider, and listing linked accounts. Split out of
// auth_handler.go for readability — same package, same *AuthHandler
// receiver, no behavior change.

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
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("access_token", res.AccessToken, 15*60, "/", cfg.CookieDomain, secureCookie(c), true)
	c.SetCookie("refresh_token", res.RefreshToken, 30*24*60*60, "/", cfg.CookieDomain, secureCookie(c), true)

	// SECURITY: refresh token stays cookie-only (HttpOnly) — not echoed in
	// the body. See the SECURITY note in auth_handler.go Login for why.
	response.Success(c, gin.H{
		"accessToken": res.AccessToken,
		"user":        res.User,
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
		response.ErrorDetail(c, http.StatusInternalServerError, "Failed to fetch linked accounts", err)
		return
	}

	response.Success(c, gin.H{"accounts": accounts})
}
