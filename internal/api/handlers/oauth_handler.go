package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"
	"thanawy-backend/internal/services"
)

// generateOAuthState generates a cryptographically random state token and
// stores it in a short-lived HttpOnly cookie for CSRF protection.
func generateOAuthState(c *gin.Context) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := base64.RawURLEncoding.EncodeToString(b)
	// Store in cookie: HttpOnly, SameSite=Lax, expires in 10 minutes
	secure := isProduction()
	c.SetCookie("oauth_state", state, int((10 * time.Minute).Seconds()), "/", "", secure, true)
	return state, nil
}

// verifyOAuthState checks that the state returned by the OAuth provider
// matches the one stored in the cookie, then clears it.
func verifyOAuthState(c *gin.Context, stateParam string) bool {
	cookieState, err := c.Cookie("oauth_state")
	// Clear immediately regardless of result (one-time use)
	c.SetCookie("oauth_state", "", -1, "/", "", isProduction(), true)
	if err != nil || cookieState == "" || cookieState != stateParam {
		return false
	}
	return true
}

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

	// Generate CSRF-protection state token
	state, err := generateOAuthState(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initiate OAuth flow"})
		return
	}

	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid email profile")
	params.Set("access_type", "offline")
	params.Set("prompt", "select_account")
	params.Set("state", state)

	authURL := "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
	c.Redirect(http.StatusFound, authURL)
}

// googleTokenResponse is the response body from Google's token endpoint.
type googleTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// googleUserInfo holds the fields we care about from the Google userinfo API.
type googleUserInfo struct {
	Sub           string `json:"sub"`   // Google user ID
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// exchangeGoogleCode exchanges an authorization code for tokens from Google.
func exchangeGoogleCode(code, redirectURI string) (*googleTokenResponse, error) {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("Google OAuth credentials are not configured")
	}

	body := url.Values{}
	body.Set("code", code)
	body.Set("client_id", clientID)
	body.Set("client_secret", clientSecret)
	body.Set("redirect_uri", redirectURI)
	body.Set("grant_type", "authorization_code")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(
		"https://oauth2.googleapis.com/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(body.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(raw))
	}

	var tokens googleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}
	return &tokens, nil
}

// fetchGoogleUserInfo retrieves the authenticated user's profile from Google.
func fetchGoogleUserInfo(accessToken string) (*googleUserInfo, error) {
	req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userinfo endpoint returned %d: %s", resp.StatusCode, string(raw))
	}

	var info googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode userinfo: %w", err)
	}
	return &info, nil
}

// OAuthGoogleCallback handles the callback from Google after user authorizes.
// GET /api/auth/oauth/google/callback
func OAuthGoogleCallback(c *gin.Context) {
	errParam := c.Query("error")
	stateParam := c.Query("state")
	code := c.Query("code")

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "https://tolo-blond.vercel.app"
	}

	// Always handle error from provider first
	if errParam != "" {
		c.Redirect(http.StatusFound, frontendURL+"/sign-in?error=oauth_denied")
		return
	}

	// Verify CSRF state token before processing code
	if !verifyOAuthState(c, stateParam) {
		c.Redirect(http.StatusFound, frontendURL+"/sign-in?error=oauth_state_mismatch")
		return
	}

	if code == "" {
		c.Redirect(http.StatusFound, frontendURL+"/sign-in?error=oauth_no_code")
		return
	}

	// Build redirect URI (must match what was sent in the redirect step)
	redirectURI := os.Getenv("GOOGLE_REDIRECT_URI")
	if redirectURI == "" {
		scheme := "https"
		if c.Request.Header.Get("X-Forwarded-Proto") == "" && c.Request.TLS == nil {
			scheme = "http"
		}
		redirectURI = fmt.Sprintf("%s://%s/api/auth/oauth/google/callback", scheme, c.Request.Host)
	}

	// Exchange authorization code for Google tokens
	googleTokens, err := exchangeGoogleCode(code, redirectURI)
	if err != nil {
		log.Printf("[OAuthGoogleCallback] token exchange error: %v", err)
		c.Redirect(http.StatusFound, frontendURL+"/sign-in?error=oauth_token_exchange_failed")
		return
	}

	// Fetch the authenticated user's Google profile
	gInfo, err := fetchGoogleUserInfo(googleTokens.AccessToken)
	if err != nil {
		log.Printf("[OAuthGoogleCallback] userinfo error: %v", err)
		c.Redirect(http.StatusFound, frontendURL+"/sign-in?error=oauth_userinfo_failed")
		return
	}

	if gInfo.Email == "" {
		c.Redirect(http.StatusFound, frontendURL+"/sign-in?error=oauth_no_email")
		return
	}

	if db.DB == nil {
		c.Redirect(http.StatusFound, frontendURL+"/sign-in?error=server_error")
		return
	}

	// --- Upsert user ---
	// 1. Try to find by Google ID first (fastest / most accurate)
	// 2. Fall back to email lookup (links existing account)
	// 3. Create new account on first sign-in
	var user models.User

	err = db.DB.Where("google_id = ?", gInfo.Sub).First(&user).Error
	if err != nil {
		// Not found by google_id – try email
		err = db.DB.Where("email ILIKE ?", gInfo.Email).First(&user).Error
		if err != nil {
			// Brand new user – create account
			name := gInfo.Name
			picture := gInfo.Picture
			user = models.User{
				ID:            uuid.New().String(),
				Email:         gInfo.Email,
				Name:          &name,
				Avatar:        &picture,
				GoogleID:      &gInfo.Sub,
				Role:          models.RoleStudent,
				EmailVerified: gInfo.EmailVerified,
				PasswordHash:  "", // No password for OAuth users
			}
			if createErr := db.DB.Create(&user).Error; createErr != nil {
				log.Printf("[OAuthGoogleCallback] create user error: %v", createErr)
				c.Redirect(http.StatusFound, frontendURL+"/sign-in?error=server_error")
				return
			}
		} else {
			// Existing email – link Google ID if not already linked
			if user.GoogleID == nil || *user.GoogleID == "" {
				if linkErr := db.DB.Model(&user).Update("google_id", gInfo.Sub).Error; linkErr != nil {
					log.Printf("[OAuthGoogleCallback] link google_id error: %v", linkErr)
				}
			}
		}
	}

	// Issue JWT token pair
	svc := &services.TokenService{}
	tokens, err := svc.GenerateTokenPair(user.ID, string(user.Role), user.Email)
	if err != nil {
		log.Printf("[OAuthGoogleCallback] token generation error: %v", err)
		c.Redirect(http.StatusFound, frontendURL+"/sign-in?error=server_error")
		return
	}

	// Persist session
	ip := c.ClientIP()
	userAgent := c.Request.UserAgent()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = ctx // db.DB is sync; context used for cleanup only

	session := &models.UserSession{
		ID:           tokens.JTI,
		UserID:       user.ID,
		RefreshToken: tokens.RefreshToken,
		UserAgent:    userAgent,
		IP:           ip,
		ExpiresAt:    time.Now().Add(30 * 24 * time.Hour),
		LastAccessed: time.Now(),
	}
	if err := db.DB.Create(session).Error; err != nil {
		log.Printf("[OAuthGoogleCallback] session create error: %v", err)
		// Non-fatal; continue with redirect
	}

	// Set auth cookies
	c.SetCookie("access_token", tokens.AccessToken, 3600*24, "/", "", isProduction(), true)
	c.SetCookie("refresh_token", tokens.RefreshToken, 3600*24*30, "/", "", isProduction(), true)

	// Redirect back to frontend with success indicator
	c.Redirect(http.StatusFound, frontendURL+"/dashboard?oauth=success")
}
