package authservice

import (
	"fmt"
)

// Factory registry for OAuth Providers
type OAuthFactory struct {
	providers map[string]OAuth2Provider
}

func NewOAuthFactory() *OAuthFactory {
	return &OAuthFactory{
		providers: make(map[string]OAuth2Provider),
	}
}

func (f *OAuthFactory) Register(name string, provider OAuth2Provider) {
	f.providers[name] = provider
}

func (f *OAuthFactory) GetProvider(name string) (OAuth2Provider, error) {
	provider, exists := f.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not configured", name)
	}
	return provider, nil
}
