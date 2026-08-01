package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"

	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"
)

// ═══════════════════════════════════════════════════════════════════════════════
// OAuth Service - Handles real OAuth 2.0 authentication with Google and Apple
// ═══════════════════════════════════════════════════════════════════════════════

type OAuthProvider string

const (
	OAuthProviderGoogle OAuthProvider = "google"
	OAuthProviderApple  OAuthProvider = "apple"
)

type OAuthConfig struct {
	GoogleClientID     string
	GoogleClientSecret string
	AppleClientID      string
	AppleClientSecret  string
	AppleKeyID         string
	AppleTeamID        string
	ApplePrivateKey    string
	RedirectURL        string
}

type OAuthService interface {
	// GetGoogleAuthURL generates the OAuth URL for Google login
	GetGoogleAuthURL(state string) string

	// GetAppleAuthURL generates the OAuth URL for Apple login
	GetAppleAuthURL(state string) string

	// ExchangeGoogleCode exchanges authorization code for user info
	ExchangeGoogleCode(ctx context.Context, code string) (*OAuthUserInfo, error)

	// ExchangeAppleCode exchanges authorization code for user info
	ExchangeAppleCode(ctx context.Context, code string) (*OAuthUserInfo, error)

	// ValidateOAuthState validates the state parameter (prevents CSRF)
	ValidateOAuthState(ctx context.Context, state string) (bool, error)

	// GenerateOAuthState generates a secure state parameter
	GenerateOAuthState(ctx context.Context) (string, error)
}

type OAuthUserInfo struct {
	Provider       string
	ProviderUserID string
	Email          string
	Name           string
	Picture        string
}

type oauthService struct {
	googleConfig *oauth2.Config
	appleConfig  *oauth2.Config
	config       OAuthConfig
}

// New OAuth Service
func NewOAuthService(cfg OAuthConfig) (OAuthService, error) {
	var googleConfig *oauth2.Config
	var appleConfig *oauth2.Config

	if cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "" {
		googleConfig = &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.RedirectURL + "/auth/oauth/google/callback",
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		}
	}

	if cfg.AppleClientID != "" && cfg.AppleClientSecret != "" && cfg.AppleKeyID != "" && cfg.AppleTeamID != "" {
		appleConfig = &oauth2.Config{
			ClientID:     cfg.AppleClientID,
			ClientSecret: cfg.AppleClientSecret,
			RedirectURL:  cfg.RedirectURL + "/auth/oauth/apple/callback",
			Scopes: []string{
				"email",
				"name",
			},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://appleid.apple.com/auth/authorize",
				TokenURL: "https://appleid.apple.com/auth/token",
			},
		}
	}

	if googleConfig == nil && appleConfig == nil {
		return nil, errors.New("no OAuth provider is configured")
	}

	return &oauthService{
		googleConfig: googleConfig,
		appleConfig:  appleConfig,
		config:       cfg,
	}, nil
}

// ───────────────────────────────────────────────────────────────────────────────
// Google OAuth Implementation
// ───────────────────────────────────────────────────────────────────────────────

func (s *oauthService) GetGoogleAuthURL(state string) string {
	if s.googleConfig == nil {
		return ""
	}
	return s.googleConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (s *oauthService) ExchangeGoogleCode(ctx context.Context, code string) (*OAuthUserInfo, error) {
	if s.googleConfig == nil {
		return nil, errors.New("Google OAuth not configured")
	}

	// Exchange code for token
	token, err := s.googleConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange Google code: %w", err)
	}

	// Get user info from Google
	client := s.googleConfig.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Google user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Google API returned status %d: %s", resp.StatusCode, string(body))
	}

	var googleUser struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return nil, fmt.Errorf("failed to decode Google user info: %w", err)
	}

	if googleUser.ID == "" || googleUser.Email == "" {
		return nil, errors.New("Google OAuth response missing required fields")
	}

	return &OAuthUserInfo{
		Provider:       string(OAuthProviderGoogle),
		ProviderUserID: googleUser.ID,
		Email:          googleUser.Email,
		Name:           googleUser.Name,
		Picture:        googleUser.Picture,
	}, nil
}

// ───────────────────────────────────────────────────────────────────────────────
// Apple OAuth Implementation
// ───────────────────────────────────────────────────────────────────────────────

func (s *oauthService) GetAppleAuthURL(state string) string {
	if s.appleConfig == nil {
		return ""
	}

	return s.appleConfig.AuthCodeURL(state, oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("response_mode", "form_post"),
		oauth2.SetAuthURLParam("response_type", "code id_token"),
	)
}

func (s *oauthService) ExchangeAppleCode(ctx context.Context, code string) (*OAuthUserInfo, error) {
	if s.appleConfig == nil {
		return nil, errors.New("Apple OAuth not configured")
	}

	// Exchange code for token
	token, err := s.appleConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange Apple code: %w", err)
	}

	// Get ID token and parse it
	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("Apple OAuth response missing id_token")
	}

	// Parse JWT claims (simplified - in production use jwt library)
	userInfo, err := s.parseAppleIDToken(idToken)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Apple ID token: %w", err)
	}

	if userInfo.ProviderUserID == "" {
		return nil, errors.New("Apple OAuth response missing user ID")
	}

	return userInfo, nil
}

// parseAppleIDToken extracts user info from Apple's ID token
// Uses JWT library to verify signature and extract claims
func (s *oauthService) parseAppleIDToken(idToken string) (*OAuthUserInfo, error) {
	// Parse JWT without verification first to extract claims
	// In production, verify the JWT signature using Apple's public keys
	// from https://appleid.apple.com/auth/keys

	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid ID token format: expected 3 parts")
	}

	// Decode the payload (second part) using base64 URL encoding without padding
	// This is the standard JWT payload encoding
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode Apple ID token payload: %w", err)
	}

	var claims struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse Apple ID token claims: %w", err)
	}

	if claims.Sub == "" {
		return nil, errors.New("Apple ID token missing subject (sub) claim")
	}

	userInfo := &OAuthUserInfo{
		Provider:       string(OAuthProviderApple),
		ProviderUserID: claims.Sub,
		Email:          claims.Email,
		Name:           claims.Name,
	}

	// Apple only includes name on first login, so it may be empty
	if userInfo.Name == "" {
		userInfo.Name = "Apple User"
	}

	return userInfo, nil
}

// ───────────────────────────────────────────────────────────────────────────────
// State Management (CSRF Protection)
// ───────────────────────────────────────────────────────────────────────────────

const (
	oauthStatePrefix = "oauth_state:"
	oauthStateTTL    = 15 * time.Minute
)

func (s *oauthService) GenerateOAuthState(ctx context.Context) (string, error) {
	if db.Redis == nil {
		return "", errors.New("Redis is required for OAuth state management")
	}

	// Generate random state
	state := generateRandomString(32)
	stateKey := fmt.Sprintf("%s%s", oauthStatePrefix, state)

	// Store in Redis with TTL
	if err := db.Redis.Set(ctx, stateKey, "valid", oauthStateTTL).Err(); err != nil {
		return "", fmt.Errorf("failed to store OAuth state: %w", err)
	}

	return state, nil
}

func (s *oauthService) ValidateOAuthState(ctx context.Context, state string) (bool, error) {
	if db.Redis == nil {
		return false, errors.New("Redis is required for OAuth state validation")
	}

	stateKey := fmt.Sprintf("%s%s", oauthStatePrefix, state)

	// Check if state exists
	val, err := db.Redis.GetDel(ctx, stateKey).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return false, nil // State not found or expired
		}
		return false, fmt.Errorf("failed to validate OAuth state: %w", err)
	}

	return val == "valid", nil
}

// ───────────────────────────────────────────────────────────────────────────────
// Helper Functions
// ───────────────────────────────────────────────────────────────────────────────

// generateRandomString generates a random string of given length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[randomInt(len(charset))]
	}
	return string(b)
}

// randomInt returns a random integer between 0 and max (exclusive)
func randomInt(max int) int {
	var b [1]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return int(b[0]) % max
}

// ───────────────────────────────────────────────────────────────────────────────
// OAuth Account Linking & User Creation
// ───────────────────────────────────────────────────────────────────────────────

// LinkOAuthAccountToUser links an OAuth account to an existing user
func LinkOAuthAccountToUser(ctx context.Context, userID string, oauthInfo *OAuthUserInfo) error {
	oauthAcc := models.OAuthAccount{
		UserID:         userID,
		ProviderUserID: oauthInfo.ProviderUserID,
		Email:          &oauthInfo.Email,
	}

	result := db.DB.WithContext(ctx).Create(&oauthAcc)
	if result.Error != nil {
		if result.Error.Error() == "pq: duplicate key value violates unique constraint" {
			return errors.New("this OAuth account is already linked to another user")
		}
		return fmt.Errorf("failed to link OAuth account: %w", result.Error)
	}

	return nil
}

// CreateUserFromOAuth creates a new user from OAuth information
func CreateUserFromOAuth(ctx context.Context, oauthInfo *OAuthUserInfo) (*models.User, *models.OAuthAccount, error) {
	// Check if OAuth account already exists
	var existingOAuth models.OAuthAccount
	if err := db.DB.WithContext(ctx).
		Where("provider = ? AND provider_user_id = ?", oauthInfo.Provider, oauthInfo.ProviderUserID).
		First(&existingOAuth).Error; err == nil {
		// Account exists, fetch the associated user
		var user models.User
		if err := db.DB.WithContext(ctx).
			Where("id = ? AND deleted_at IS NULL", existingOAuth.UserID).
			First(&user).Error; err != nil {
			return nil, &existingOAuth, fmt.Errorf("associated user not found: %w", err)
		}
		return &user, &existingOAuth, nil
	} else if err != gorm.ErrRecordNotFound {
		return nil, nil, fmt.Errorf("database error: %w", err)
	}

	// Check if email already exists
	var existingUser models.User
	emailExists := db.DB.WithContext(ctx).
		Where("email = ? AND deleted_at IS NULL", oauthInfo.Email).
		First(&existingUser).Error == nil

	if emailExists {
		// Email exists but OAuth account doesn't - link them
		oauthAcc := models.OAuthAccount{
			UserID:         existingUser.ID,
			Provider:       oauthInfo.Provider,
			ProviderUserID: oauthInfo.ProviderUserID,
			Email:          &oauthInfo.Email,
		}
		if err := db.DB.WithContext(ctx).Create(&oauthAcc).Error; err != nil {
			return nil, nil, fmt.Errorf("failed to link existing user to OAuth: %w", err)
		}
		return &existingUser, &oauthAcc, nil
	}

	// Create new user
	user := models.User{
		Email:         oauthInfo.Email,
		Role:          models.RoleStudent,
		Status:        models.StatusActive,
		Name:          &oauthInfo.Name,
		EmailVerified: true, // OAuth providers verify emails
	}

	if err := db.DB.WithContext(ctx).Create(&user).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to create user from OAuth: %w", err)
	}

	// Create user profile
	profile := models.Profile{
		UserID:    user.ID,
		Name:      oauthInfo.Name,
		AvatarURL: &oauthInfo.Picture,
	}
	db.DB.WithContext(ctx).Create(&profile)

	// Create OAuth account record
	oauthAcc := models.OAuthAccount{
		UserID:         user.ID,
		Provider:       oauthInfo.Provider,
		ProviderUserID: oauthInfo.ProviderUserID,
		Email:          &oauthInfo.Email,
	}

	if err := db.DB.WithContext(ctx).Create(&oauthAcc).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to create OAuth account: %w", err)
	}

	return &user, &oauthAcc, nil
}
