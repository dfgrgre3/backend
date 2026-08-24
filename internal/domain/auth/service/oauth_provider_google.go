package authservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

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
