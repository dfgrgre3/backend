package authservice

import (
	"context"
)

// OAuth2Provider implementations are split across several files in this
// package (all sharing package authservice), one provider per file: this
// file (shared OAuthUser type + OAuth2Provider interface),
// oauth_provider_google.go, oauth_provider_github.go,
// oauth_provider_microsoft.go, oauth_provider_apple.go (plus its JWKS/JWT
// verification helpers) and oauth_provider_factory.go (OAuthFactory
// registry).

type OAuthUser struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	AvatarURL   string `json:"avatar_url"`
	Provider    string `json:"provider"`
	AccessToken string `json:"access_token"`
}

type OAuth2Provider interface {
	GetAuthURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*OAuthUser, error)
}
