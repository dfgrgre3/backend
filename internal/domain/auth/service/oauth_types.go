package authservice

import "context"

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
