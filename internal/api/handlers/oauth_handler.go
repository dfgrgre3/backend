package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/gin-gonic/gin"
)

// OAuthGoogleRedirect initiates the Google OAuth 2.0 authorization flow.
// GET /api/auth/oauth/google
func OAuthGoogleRedirect(c *gin.Context) {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	if clientID == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Google OAuth is not configured on this server",
		})
		return
	}

	redirectURI := os.Getenv("GOOGLE_REDIRECT_URI")
	if redirectURI == "" {
		// Build a sensible default from the request host
		scheme := "https"
		if c.Request.Header.Get("X-Forwarded-Proto") == "" && c.Request.TLS == nil {
			scheme = "http"
		}
		redirectURI = fmt.Sprintf("%s://%s/api/auth/oauth/google/callback", scheme, c.Request.Host)
	}

	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid email profile")
	params.Set("access_type", "offline")
	params.Set("prompt", "select_account")

	authURL := "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
	c.Redirect(http.StatusFound, authURL)
}

// OAuthGoogleCallback handles the callback from Google after user authorizes.
// GET /api/auth/oauth/google/callback
func OAuthGoogleCallback(c *gin.Context) {
	code := c.Query("code")
	errParam := c.Query("error")

	if errParam != "" {
		frontendURL := os.Getenv("FRONTEND_URL")
		if frontendURL == "" {
			frontendURL = "https://tolo-blond.vercel.app"
		}
		c.Redirect(http.StatusFound, frontendURL+"/sign-in?error=oauth_denied")
		return
	}

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No authorization code received"})
		return
	}

	// TODO: Exchange code for tokens, fetch user profile, create/update user,
	// generate JWT, set cookies and redirect to frontend.
	// For now return a placeholder response.
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "Google OAuth callback not fully implemented yet",
		"message": "Please use email/password login",
	})
}
