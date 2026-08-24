package authservice

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"thanawy-backend/internal/infrastructure/cache"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

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

// Google OAuth Implementation
func (s *oauthService) GetGoogleAuthURL(state string) string {
	if s.googleConfig == nil {
		return ""
	}
	return s.googleConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (s *oauthService) ExchangeGoogleCode(ctx context.Context, code string) (*OAuthUserInfo, error) {
	if s.googleConfig == nil {
		return nil, errors.New("google OAuth not configured")
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
		return nil, fmt.Errorf("google API returned status %d: %s", resp.StatusCode, string(body))
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
		return nil, errors.New("google OAuth response missing required fields")
	}

	return &OAuthUserInfo{
		Provider:       string(OAuthProviderGoogle),
		ProviderUserID: googleUser.ID,
		Email:          googleUser.Email,
		Name:           googleUser.Name,
		Picture:        googleUser.Picture,
	}, nil
}

// Apple OAuth Implementation
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
		return nil, errors.New("apple OAuth not configured")
	}

	// Exchange code for token
	token, err := s.appleConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange Apple code: %w", err)
	}

	// Get ID token and parse it
	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("apple OAuth response missing id_token")
	}

	// Parse JWT claims
	userInfo, err := s.parseAppleIDToken(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Apple ID token: %w", err)
	}

	if userInfo.ProviderUserID == "" {
		return nil, errors.New("apple OAuth response missing user ID")
	}

	return userInfo, nil
}

// parseAppleIDToken verifies an Apple-issued ID token's signature (against
// Apple's published JWKS), issuer, audience and expiry before trusting any
// claim inside it. This mirrors the verification already implemented in
// AppleProvider.ExchangeCode (oauth_provider_apple.go), which this function
// reuses (fetchApplePublicKey) rather than duplicating.
//
// SECURITY: a previous version of this function only base64-decoded the
// token payload with no signature/issuer/audience/expiry check, meaning any
// well-formed-looking JWT with a `sub`/`email` claim was accepted as proof
// of identity — including tokens issued to a different Apple client, or a
// leaked/replayed token that had long since expired. Do not regress this.
func (s *oauthService) parseAppleIDToken(ctx context.Context, idToken string) (*OAuthUserInfo, error) {
	parser := new(jwt.Parser)
	unverified, _, err := parser.ParseUnverified(idToken, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse Apple ID token header: %w", err)
	}

	kid, ok := unverified.Header["kid"].(string)
	if !ok || kid == "" {
		return nil, errors.New("apple ID token missing kid header")
	}

	publicKey, err := fetchApplePublicKey(ctx, kid)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Apple public key: %w", err)
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(idToken, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	}, jwt.WithIssuer("https://appleid.apple.com"), jwt.WithAudience(s.config.AppleClientID))
	if err != nil {
		return nil, fmt.Errorf("apple ID token verification failed: %w", err)
	}
	if !token.Valid {
		return nil, errors.New("apple ID token is invalid")
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, errors.New("apple ID token missing subject (sub) claim")
	}
	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)

	userInfo := &OAuthUserInfo{
		Provider:       string(OAuthProviderApple),
		ProviderUserID: sub,
		Email:          email,
		Name:           name,
	}

	if userInfo.Name == "" {
		userInfo.Name = "apple User"
	}

	return userInfo, nil
}

// State Management (CSRF Protection)
const (
	oauthStatePrefix = "oauth_state:"
	oauthStateTTL    = 15 * time.Minute
)

func (s *oauthService) GenerateOAuthState(ctx context.Context) (string, error) {
	if cache.Redis == nil {
		return "", errors.New("redis is required for OAuth state management")
	}

	state := generateRandomString(32)
	stateKey := fmt.Sprintf("%s%s", oauthStatePrefix, state)

	if err := cache.Redis.Set(ctx, stateKey, "valid", oauthStateTTL).Err(); err != nil {
		return "", fmt.Errorf("failed to store OAuth state: %w", err)
	}

	return state, nil
}

func (s *oauthService) ValidateOAuthState(ctx context.Context, state string) (bool, error) {
	if cache.Redis == nil {
		return false, errors.New("redis is required for OAuth state validation")
	}

	stateKey := fmt.Sprintf("%s%s", oauthStatePrefix, state)

	val, err := cache.Redis.GetDel(ctx, stateKey).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return false, nil // State not found or expired
		}
		return false, fmt.Errorf("failed to validate OAuth state: %w", err)
	}

	return val == "valid", nil
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[randomInt(len(charset))]
	}
	return string(b)
}

func randomInt(max int) int {
	if max <= 0 {
		return 0
	}
	var b [1]byte
	if _, err := rand.Read(b[:]); err != nil {
		return int(time.Now().UnixNano()) % max
	}
	return int(b[0]) % max
}
