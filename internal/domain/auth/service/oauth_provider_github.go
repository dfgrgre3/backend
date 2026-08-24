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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github token exchange failed with status: %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}
	if tokenResp.AccessToken == "" {
		return nil, errors.New("github token exchange returned no access token")
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

	// GitHub may return empty email if private, fetch emails list. We only
	// ever trust a verified email here: this value is later used both to
	// match/auto-link an existing Thanawy account by email (oauth_handler.go)
	// and to mark the resulting user as EmailVerified=true. Falling back to
	// an unverified or non-primary address would let an attacker take over
	// any account by adding that account's email (unverified) to their own
	// GitHub account and completing OAuth — GitHub never confirmed they
	// actually control it.
	email := githubUser.Email
	if email == "" {
		reqEmails, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user/emails", nil)
		if err == nil {
			reqEmails.Header.Set("Authorization", "token "+tokenResp.AccessToken)
			respEmails, err := http.DefaultClient.Do(reqEmails)
			if err == nil {
				defer respEmails.Body.Close()
				if respEmails.StatusCode == http.StatusOK {
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
						// Deliberately no fallback to an unverified/non-primary
						// email here. If no verified primary email is found,
						// email stays "" and the caller must reject the login
						// rather than trust an unconfirmed address.
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
