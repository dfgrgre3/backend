package authservice

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

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

// Google Provider implementation
type GoogleProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func NewGoogleProvider(clientID, clientSecret, redirectURL string) *GoogleProvider {
	return &GoogleProvider{ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL}
}

func (p *GoogleProvider) GetAuthURL(state string) string {
	return fmt.Sprintf(
		"https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=openid%%20profile%%20email&state=%s",
		url.QueryEscape(p.ClientID),
		url.QueryEscape(p.RedirectURL),
		url.QueryEscape(state),
	)
}

func (p *GoogleProvider) ExchangeCode(ctx context.Context, code string) (*OAuthUser, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", p.ClientID)
	data.Set("client_secret", p.ClientSecret)
	data.Set("redirect_uri", p.RedirectURL)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google token exchange failed with status: %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	// Fetch user info using the access token
	reqInfo, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}
	reqInfo.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)

	respInfo, err := http.DefaultClient.Do(reqInfo)
	if err != nil {
		return nil, err
	}
	defer respInfo.Body.Close()

	var googleUser struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(respInfo.Body).Decode(&googleUser); err != nil {
		return nil, err
	}

	return &OAuthUser{
		ID:          googleUser.ID,
		Email:       googleUser.Email,
		Name:        googleUser.Name,
		AvatarURL:   googleUser.Picture,
		Provider:    "google",
		AccessToken: tokenResp.AccessToken,
	}, nil
}

// GitHub Provider implementation
type GitHubProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func NewGitHubProvider(clientID, clientSecret, redirectURL string) *GitHubProvider {
	return &GitHubProvider{ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL}
}

func (p *GitHubProvider) GetAuthURL(state string) string {
	return fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=user:email&state=%s",
		url.QueryEscape(p.ClientID),
		url.QueryEscape(p.RedirectURL),
		url.QueryEscape(state),
	)
}

func (p *GitHubProvider) ExchangeCode(ctx context.Context, code string) (*OAuthUser, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", p.ClientID)
	data.Set("client_secret", p.ClientSecret)
	data.Set("redirect_uri", p.RedirectURL)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	// Fetch user details
	reqUser, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	reqUser.Header.Set("Authorization", "token "+tokenResp.AccessToken)
	reqUser.Header.Set("Accept", "application/json")

	respUser, err := http.DefaultClient.Do(reqUser)
	if err != nil {
		return nil, err
	}
	defer respUser.Body.Close()

	var githubUser struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Email     string `json:"email"`
	}
	if err := json.NewDecoder(respUser.Body).Decode(&githubUser); err != nil {
		return nil, err
	}

	// GitHub may return empty email if private, fetch emails list
	email := githubUser.Email
	if email == "" {
		reqEmails, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user/emails", nil)
		if err == nil {
			reqEmails.Header.Set("Authorization", "token "+tokenResp.AccessToken)
			respEmails, err := http.DefaultClient.Do(reqEmails)
			if err == nil {
				defer respEmails.Body.Close()
				var emails []struct {
					Email    string `json:"email"`
					Primary  bool   `json:"primary"`
					Verified bool   `json:"verified"`
				}
				if err := json.NewDecoder(respEmails.Body).Decode(&emails); err == nil {
					for _, e := range emails {
						if e.Primary && e.Verified {
							email = e.Email
							break
						}
					}
					if email == "" && len(emails) > 0 {
						email = emails[0].Email
					}
				}
			}
		}
	}

	name := githubUser.Name
	if name == "" {
		name = githubUser.Login
	}

	return &OAuthUser{
		ID:          fmt.Sprintf("%d", githubUser.ID),
		Email:       email,
		Name:        name,
		AvatarURL:   githubUser.AvatarURL,
		Provider:    "github",
		AccessToken: tokenResp.AccessToken,
	}, nil
}

// Microsoft Provider implementation
type MicrosoftProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func NewMicrosoftProvider(clientID, clientSecret, redirectURL string) *MicrosoftProvider {
	return &MicrosoftProvider{ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL}
}

func (p *MicrosoftProvider) GetAuthURL(state string) string {
	return fmt.Sprintf(
		"https://login.microsoftonline.com/common/oauth2/v2.0/authorize?client_id=%s&redirect_uri=%s&response_type=code&scope=User.Read&state=%s",
		url.QueryEscape(p.ClientID),
		url.QueryEscape(p.RedirectURL),
		url.QueryEscape(state),
	)
}

func (p *MicrosoftProvider) ExchangeCode(ctx context.Context, code string) (*OAuthUser, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", p.ClientID)
	data.Set("client_secret", p.ClientSecret)
	data.Set("redirect_uri", p.RedirectURL)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, "POST", "https://login.microsoftonline.com/common/oauth2/v2.0/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	// Fetch Microsoft graph profile
	reqUser, err := http.NewRequestWithContext(ctx, "GET", "https://graph.microsoft.com/v1.0/me", nil)
	if err != nil {
		return nil, err
	}
	reqUser.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)

	respUser, err := http.DefaultClient.Do(reqUser)
	if err != nil {
		return nil, err
	}
	defer respUser.Body.Close()

	var msUser struct {
		ID                string `json:"id"`
		DisplayName       string `json:"displayName"`
		Mail              string `json:"mail"`
		UserPrincipalName string `json:"userPrincipalName"`
	}
	if err := json.NewDecoder(respUser.Body).Decode(&msUser); err != nil {
		return nil, err
	}

	email := msUser.Mail
	if email == "" {
		email = msUser.UserPrincipalName
	}

	return &OAuthUser{
		ID:          msUser.ID,
		Email:       email,
		Name:        msUser.DisplayName,
		AvatarURL:   "", // Microsoft Graph requires a separate request for avatar
		Provider:    "microsoft",
		AccessToken: tokenResp.AccessToken,
	}, nil
}

// Apple Provider implementation — verifies ID token via Apple's public JWKS endpoint.
type AppleProvider struct {
	ClientID    string
	TeamID      string
	KeyID       string
	Secret      string // Private key content
	RedirectURL string
}

func NewAppleProvider(clientID, teamID, keyID, secret, redirectURL string) *AppleProvider {
	return &AppleProvider{ClientID: clientID, TeamID: teamID, KeyID: keyID, Secret: secret, RedirectURL: redirectURL}
}

func (p *AppleProvider) GetAuthURL(state string) string {
	return fmt.Sprintf(
		"https://appleid.apple.com/auth/authorize?client_id=%s&redirect_uri=%s&response_type=code%%20id_token&scope=name%%20email&response_mode=form_post&state=%s",
		url.QueryEscape(p.ClientID),
		url.QueryEscape(p.RedirectURL),
		url.QueryEscape(state),
	)
}

// appleJWK represents a single JSON Web Key from Apple's JWKS endpoint.
type appleJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// fetchApplePublicKey fetches Apple's JWKS and returns the RSA public key
// matching the kid in the token header.
func fetchApplePublicKey(ctx context.Context, kid string) (interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://appleid.apple.com/auth/keys", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Apple JWKS request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Apple JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple JWKS returned status %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []appleJWK `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode Apple JWKS: %w", err)
	}

	for _, key := range jwks.Keys {
		if key.Kid == kid && key.Kty == "RSA" {
			// Decode n and e from base64url
			nBytes, err := base64URLDecode(key.N)
			if err != nil {
				return nil, fmt.Errorf("failed to decode Apple key N: %w", err)
			}
			eBytes, err := base64URLDecode(key.E)
			if err != nil {
				return nil, fmt.Errorf("failed to decode Apple key E: %w", err)
			}

			// Convert e bytes to int
			var eInt int
			for _, b := range eBytes {
				eInt = eInt<<8 + int(b)
			}

			pubKey := &rsa.PublicKey{
				N: new(big.Int).SetBytes(nBytes),
				E: eInt,
			}
			return pubKey, nil
		}
	}

	return nil, fmt.Errorf("no matching Apple public key found for kid=%s", kid)
}

// base64URLDecode decodes a base64url-encoded string (no padding).
func base64URLDecode(s string) ([]byte, error) {
	// Add padding if needed
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}

func (p *AppleProvider) ExchangeCode(ctx context.Context, idToken string) (*OAuthUser, error) {
	// Parse the token header to get the kid without verifying (we verify below).
	parser := new(jwt.Parser)
	unverified, _, err := parser.ParseUnverified(idToken, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse Apple ID token header: %w", err)
	}

	kid, ok := unverified.Header["kid"].(string)
	if !ok || kid == "" {
		return nil, errors.New("apple ID token missing kid header")
	}

	// Fetch the matching RSA public key from Apple's JWKS endpoint.
	publicKey, err := fetchApplePublicKey(ctx, kid)
	if err != nil {
		return nil, err
	}

	// Now verify the token signature and claims properly.
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(idToken, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	}, jwt.WithIssuer("https://appleid.apple.com"), jwt.WithAudience(p.ClientID))
	if err != nil {
		return nil, fmt.Errorf("apple ID token verification failed: %w", err)
	}
	if !token.Valid {
		return nil, errors.New("apple ID token is invalid")
	}

	email, _ := claims["email"].(string)
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, errors.New("apple ID token missing sub claim")
	}

	return &OAuthUser{
		ID:          sub,
		Email:       email,
		Name:        "", // Apple only returns the name on the very first login via form_post parameters
		AvatarURL:   "",
		Provider:    "apple",
		AccessToken: "",
	}, nil
}

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
