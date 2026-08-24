package authservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("microsoft token exchange failed with status: %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}
	if tokenResp.AccessToken == "" {
		return nil, errors.New("microsoft token exchange returned no access token")
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
