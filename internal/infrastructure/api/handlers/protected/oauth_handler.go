package protected

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
	authservice "thanawy-backend/internal/domain/auth/service"
	models "thanawy-backend/internal/domain/common"
	"time"

	"thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type OAuthHandler struct {
	oauthFactory *authservice.OAuthFactory
	tokenSvc     authservice.AuthTokenService
}

func NewOAuthHandler(tokenSvc authservice.AuthTokenService) *OAuthHandler {
	factory := authservice.NewOAuthFactory()

	// Register providers using environment variables
	redirectBase := os.Getenv("OAUTH_REDIRECT_BASE_URL")
	if redirectBase == "" {
		redirectBase = "http://localhost:8080"
	}

	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if googleClientID != "" && googleClientSecret != "" {
		factory.Register("google", authservice.NewGoogleProvider(
			googleClientID,
			googleClientSecret,
			redirectBase+"/api/v1/auth/callback/google",
		))
	}

	githubClientID := os.Getenv("GITHUB_CLIENT_ID")
	githubClientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	if githubClientID != "" && githubClientSecret != "" {
		factory.Register("github", authservice.NewGitHubProvider(
			githubClientID,
			githubClientSecret,
			redirectBase+"/api/v1/auth/callback/github",
		))
	}

	msClientID := os.Getenv("MICROSOFT_CLIENT_ID")
	msClientSecret := os.Getenv("MICROSOFT_CLIENT_SECRET")
	if msClientID != "" && msClientSecret != "" {
		factory.Register("microsoft", authservice.NewMicrosoftProvider(
			msClientID,
			msClientSecret,
			redirectBase+"/api/v1/auth/callback/microsoft",
		))
	}

	appleClientID := os.Getenv("APPLE_CLIENT_ID")
	appleTeamID := os.Getenv("APPLE_TEAM_ID")
	appleKeyID := os.Getenv("APPLE_KEY_ID")
	appleSecret := os.Getenv("APPLE_SECRET")
	if appleClientID != "" {
		factory.Register("apple", authservice.NewAppleProvider(
			appleClientID,
			appleTeamID,
			appleKeyID,
			appleSecret,
			redirectBase+"/api/v1/auth/callback/apple",
		))
	}

	return &OAuthHandler{
		oauthFactory: factory,
		tokenSvc:     tokenSvc,
	}
}

func generateStateToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *OAuthHandler) RedirectToProvider(c *gin.Context) {
	// SECURITY (not SSRF): providerName is only used as a lookup key into
	// oauthFactory's allowlisted provider map (google/github/microsoft/apple,
	// registered in NewOAuthHandler). Unknown values fail the GetProvider
	// lookup below and return 400 before reaching the redirect. Each
	// provider's GetAuthURL builds authURL from a hardcoded scheme+host
	// (e.g. https://accounts.google.com/...) plus server-configured
	// ClientID/RedirectURL and a server-generated state token — no request
	// input ever contributes to the target host. c.Redirect also only makes
	// the browser navigate; the server itself never fetches authURL.
	providerName := c.Param("provider")
	provider, err := h.oauthFactory.GetProvider(providerName)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "OAuth Provider not configured: "+err.Error())
		return
	}

	state := generateStateToken()

	// Store state token in temporary short-lived cookie for CSRF protection
	c.SetCookie("oauth_state", state, 300, "/api/v1/auth", "", secureCookie(c), true)

	authURL := provider.GetAuthURL(state)
	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

func (h *OAuthHandler) OAuthCallback(c *gin.Context) {
	providerName := c.Param("provider")
	code := c.Query("code")
	state := c.Query("state")

	// Validate CSRF state
	stateCookie, err := c.Cookie("oauth_state")
	if err != nil || stateCookie == "" || stateCookie != state {
		response.Error(c, http.StatusBadRequest, "CSRF validation failed: invalid state token")
		return
	}

	// Delete state cookie
	c.SetCookie("oauth_state", "", -1, "/api/v1/auth", "", secureCookie(c), true)

	provider, err := h.oauthFactory.GetProvider(providerName)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "OAuth Provider not configured")
		return
	}

	ctx := c.Request.Context()
	oauthUser, err := provider.ExchangeCode(ctx, code)
	if err != nil {
		response.ErrorDetail(c, http.StatusInternalServerError, "Failed to exchange auth code", err)
		return
	}

	if oauthUser.Email == "" {
		response.Error(c, http.StatusBadRequest, "OAuth Provider did not return a verified email address")
		return
	}

	var user models.User
	var isNewUser bool

	// Begin Transaction to ensure user and mappings are saved atomically
	tx := db.DB.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 1. Look up existing oauth account mapping
	var oauthAccount models.OAuthAccount
	err = tx.Where("provider = ? AND provider_user_id = ?", providerName, oauthUser.ID).First(&oauthAccount).Error
	if err == nil {
		// OAuth account exists, load user
		if err := tx.Where("id = ?", oauthAccount.UserID).First(&user).Error; err != nil {
			tx.Rollback()
			response.Error(c, http.StatusInternalServerError, "Associated user account not found")
			return
		}
	} else {
		// 2. No direct OAuth mapping, check by email
		err = tx.Where("email = ?", oauthUser.Email).First(&user).Error
		if err != nil {
			// User does not exist, create new user (Register)
			isNewUser = true

			// Generate random password hash
			randomPassword := generateStateToken() + generateStateToken()
			_, err := bcrypt.GenerateFromPassword([]byte(randomPassword), bcrypt.DefaultCost)
			if err != nil {
				tx.Rollback()
				response.Error(c, http.StatusInternalServerError, "Registration failure")
				return
			}

			user = models.User{
				Email:         oauthUser.Email,
				Role:          models.RoleStudent,
				Status:        models.StatusActive,
				EmailVerified: true,
				Name:          &oauthUser.Name,
				Avatar:        &oauthUser.AvatarURL,
			}

			if err := tx.Create(&user).Error; err != nil {
				tx.Rollback()
				response.Error(c, http.StatusInternalServerError, "Failed to create user account")
				return
			}

			// Create Profile
			profile := models.Profile{
				UserID:    user.ID,
				Name:      oauthUser.Name,
				AvatarURL: &oauthUser.AvatarURL,
			}
			if err := tx.Create(&profile).Error; err != nil {
				tx.Rollback()
				response.Error(c, http.StatusInternalServerError, "Failed to create user profile")
				return
			}
		} else {
			// SECURITY: an existing account matched by email, but with no
			// prior OAuth link for this provider, must NOT be silently
			// linked and logged into. Doing so would let anyone who can get
			// an OAuth provider to report a given email address (e.g. by
			// adding that email, verified or not, to their own provider
			// account — see the GitHub email-fallback note in
			// oauth_provider.go) take over the matching Thanawy account
			// without ever proving they control it. Require the user to log
			// in normally first and link the provider explicitly from
			// account settings (LinkOAuthProvider), which operates on an
			// already-authenticated session.
			tx.Rollback()
			response.Error(c, http.StatusConflict, "An account with this email already exists. Please log in and link this provider from your account settings.")
			return
		}

		// 3. Create OAuth link (new user only — see the SECURITY note above
		// for the existing-account-by-email case, which returns early).
		oauthAccount = models.OAuthAccount{
			UserID:         user.ID,
			Provider:       providerName,
			ProviderUserID: oauthUser.ID,
			AccessToken:    &oauthUser.AccessToken,
		}
		if err := tx.Create(&oauthAccount).Error; err != nil {
			tx.Rollback()
			response.Error(c, http.StatusInternalServerError, "Failed to link oauth account")
			return
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "Transaction commit failure")
		return
	}

	// 4. Create Session & Tokens
	tokenPair, err := h.tokenSvc.GenerateTokenPair(&user)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to generate tokens")
		return
	}

	// Parse user agent and metadata
	userAgent := c.Request.UserAgent()
	ip := c.ClientIP()

	uaLower := strings.ToLower(userAgent)
	osVal := "Unknown OS"
	if strings.Contains(uaLower, "windows") {
		osVal = "Windows"
	} else if strings.Contains(uaLower, "mac") {
		osVal = "MacOS"
	} else if strings.Contains(uaLower, "linux") {
		osVal = "Linux"
	}
	browser := "Unknown Browser"
	if strings.Contains(uaLower, "chrome") {
		browser = "Chrome"
	} else if strings.Contains(uaLower, "firefox") {
		browser = "Firefox"
	}

	userSession := &models.UserSession{
		ID:           tokenPair.JTI, // Match access token JTI to session ID
		UserID:       user.ID,
		RefreshToken: tokenPair.RefreshToken,
		UserAgent:    userAgent,
		IP:           ip,
		Browser:      browser,
		OS:           osVal,
		DeviceType:   "web",
		Status:       "active",
		IsActive:     true,
		LastAccessed: time.Now(),
		ExpiresAt:    time.Now().Add(30 * 24 * time.Hour),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := db.DB.Create(userSession).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create session")
		return
	}

	// Set cookies
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("access_token", tokenPair.AccessToken, 15*60, "/", "", secureCookie(c), true)
	c.SetCookie("refresh_token", tokenPair.RefreshToken, 30*24*60*60, "/", "", secureCookie(c), true)

	// Redirect to frontend dashboard or return payload
	frontendURL := os.Getenv("FRONTEND_DASHBOARD_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000/dashboard"
	}

	if isNewUser {
		c.Redirect(http.StatusTemporaryRedirect, frontendURL+"?new_user=true")
	} else {
		c.Redirect(http.StatusTemporaryRedirect, frontendURL)
	}
}
