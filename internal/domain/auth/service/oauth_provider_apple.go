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

	"github.com/golang-jwt/jwt/v5"
)

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
