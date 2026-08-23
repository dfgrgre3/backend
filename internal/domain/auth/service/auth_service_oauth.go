package authservice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	authdto "thanawy-backend/internal/application/dto"
	analyticsservice "thanawy-backend/internal/domain/analytics/service"
	models "thanawy-backend/internal/domain/common"
	authrepo "thanawy-backend/internal/infrastructure/persistence/repositories"
	"time"
)

// Social sign-in (OAuth): redirect URL generation, callback handling,
// linking/unlinking provider accounts, and listing linked accounts. Split
// out of auth_service.go for readability — same package, same *authService
// receiver, no behavior change.

func (s *authService) GetOAuthRedirectURL(ctx context.Context, provider string) (string, error) {
	if s.oauthService == nil {
		return "", errors.New("oauth service is not configured")
	}

	state, err := s.oauthService.GenerateOAuthState(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to generate OAuth state: %w", err)
	}

	switch strings.ToLower(provider) {
	case "google":
		redirectURL := s.oauthService.GetGoogleAuthURL(state)
		if redirectURL == "" {
			return "", errors.New("google OAuth is not configured")
		}
		return redirectURL, nil
	case "apple":
		redirectURL := s.oauthService.GetAppleAuthURL(state)
		if redirectURL == "" {
			return "", errors.New("apple OAuth is not configured")
		}
		return redirectURL, nil
	default:
		return "", fmt.Errorf("unsupported OAuth provider: %s", provider)
	}
}

func (s *authService) HandleOAuthCallback(ctx context.Context, provider, code, state, userAgent, ip string) (*authdto.LoginResponse, error) {
	// ───────────────────────────────────────────────────────────────────────────────
	// Step 1: Validate OAuth state (CSRF protection)
	// ───────────────────────────────────────────────────────────────────────────────
	if s.oauthService == nil {
		return nil, errors.New("oauth service not configured")
	}

	valid, err := s.oauthService.ValidateOAuthState(ctx, state)
	if err != nil || !valid {
		s.auditService.LogEvent("", analyticsservice.AuditEventLoginFailed, "oauth", "", map[string]interface{}{
			"provider": provider,
			"reason":   "invalid_state",
		}, ip, "")
		return nil, errors.New("invalid or expired OAuth state - possible CSRF attack")
	}

	// ───────────────────────────────────────────────────────────────────────────────
	// Step 2: Exchange authorization code for real user info
	// ───────────────────────────────────────────────────────────────────────────────
	var oauthUserInfo *OAuthUserInfo
	var oauthErr error

	switch provider {
	case "google":
		oauthUserInfo, oauthErr = s.oauthService.ExchangeGoogleCode(ctx, code)
	case "apple":
		oauthUserInfo, oauthErr = s.oauthService.ExchangeAppleCode(ctx, code)
	default:
		s.auditService.LogEvent("", analyticsservice.AuditEventLoginFailed, "oauth", "", map[string]interface{}{
			"provider": provider,
			"reason":   "unsupported_provider",
		}, ip, "")
		return nil, fmt.Errorf("unsupported OAuth provider: %s", provider)
	}

	if oauthErr != nil {
		s.auditService.LogEvent("", analyticsservice.AuditEventLoginFailed, "oauth", "", map[string]interface{}{
			"provider": provider,
			"reason":   "exchange_failed",
			"error":    oauthErr.Error(),
		}, ip, "")
		return nil, fmt.Errorf("failed to get user info from %s: %w", provider, oauthErr)
	}

	if oauthUserInfo == nil {
		return nil, fmt.Errorf("no user info returned from %s", provider)
	}

	// ───────────────────────────────────────────────────────────────────────────────
	// Step 3: Create user or link to existing account
	// ───────────────────────────────────────────────────────────────────────────────
	user, _, err := CreateUserFromOAuth(ctx, oauthUserInfo)
	if err != nil {
		s.auditService.LogEvent("", analyticsservice.AuditEventLoginFailed, "oauth", "", map[string]interface{}{
			"provider": provider,
			"email":    oauthUserInfo.Email,
			"reason":   "user_creation_failed",
			"error":    err.Error(),
		}, ip, "")
		return nil, fmt.Errorf("failed to create or link OAuth user: %w", err)
	}

	if user == nil {
		return nil, errors.New("user creation returned nil")
	}

	// ───────────────────────────────────────────────────────────────────────────────
	// Step 4: Generate JWT tokens
	// ───────────────────────────────────────────────────────────────────────────────
	tokenPair, err := s.tokenService.GenerateTokenPair(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// ───────────────────────────────────────────────────────────────────────────────
	// Step 5: Create user session
	// ───────────────────────────────────────────────────────────────────────────────
	session := &models.UserSession{
		UserID:            user.ID,
		RefreshToken:      tokenPair.RefreshToken,
		RefreshTokenHash:  models.ComputeRefreshTokenHash(tokenPair.RefreshToken),
		UserAgent:         userAgent,
		IP:                ip,
		DeviceType:        "oauth_" + provider,
		IsActive:          true,
		ExpiresAt:         time.Now().Add(30 * 24 * time.Hour),
		AbsoluteExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}

	if err := s.authRepo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// ───────────────────────────────────────────────────────────────────────────────
	// Step 6: Log successful OAuth login and return response
	// ───────────────────────────────────────────────────────────────────────────────
	s.auditService.LogEvent(user.ID, analyticsservice.AuditEventLogin, "oauth", user.ID, map[string]interface{}{
		"provider": provider,
		"email":    user.Email,
	}, ip, "")

	userDTO := authdto.UserDTO{
		ID:            user.ID,
		Email:         user.Email,
		Name:          *user.Name,
		Role:          string(user.Role),
		EmailVerified: user.EmailVerified,
		Permissions:   user.GetEffectivePermissions(),
	}

	return &authdto.LoginResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		User:         userDTO,
	}, nil
}

func (s *authService) LinkOAuthProvider(ctx context.Context, userID, provider, code, state string) error {
	if s.oauthService == nil {
		return errors.New("oauth service is not configured")
	}

	// Validate state parameter (CSRF protection)
	valid, err := s.oauthService.ValidateOAuthState(ctx, state)
	if err != nil || !valid {
		return errors.New("invalid or expired OAuth state")
	}

	// Exchange the authorization code for user info from the provider
	var oauthUserInfo *OAuthUserInfo
	switch strings.ToLower(provider) {
	case "google":
		oauthUserInfo, err = s.oauthService.ExchangeGoogleCode(ctx, code)
	case "apple":
		oauthUserInfo, err = s.oauthService.ExchangeAppleCode(ctx, code)
	default:
		return fmt.Errorf("unsupported OAuth provider: %s", provider)
	}

	if err != nil {
		return fmt.Errorf("failed to exchange code with %s: %w", provider, err)
	}

	if oauthUserInfo == nil {
		return fmt.Errorf("no user info returned from %s", provider)
	}

	// Check if this OAuth account is already linked to another user
	existingLink, err := s.authRepo.FindOAuthAccountByProvider(ctx, oauthUserInfo.Provider, oauthUserInfo.ProviderUserID)
	if err == nil && existingLink != nil {
		return errors.New("this social account is already linked to another user")
	} else if err != nil && !authrepo.IsNotFound(err) {
		return fmt.Errorf("database error: %w", err)
	}

	// Link the OAuth account to the user
	link := models.OAuthAccount{
		UserID:         userID,
		Provider:       oauthUserInfo.Provider,
		ProviderUserID: oauthUserInfo.ProviderUserID,
		Email:          &oauthUserInfo.Email,
	}
	if err := s.authRepo.CreateOAuthAccount(ctx, &link); err != nil {
		return fmt.Errorf("failed to link account: %w", err)
	}
	return nil
}

func (s *authService) UnlinkOAuthProvider(ctx context.Context, userID, provider string) error {
	if err := s.authRepo.DeleteOAuthAccount(ctx, userID, provider); err != nil {
		return errors.New("failed to unlink provider")
	}
	return nil
}

func (s *authService) GetLinkedAccounts(ctx context.Context, userID string) ([]authdto.LinkedAccountDTO, error) {
	accounts, err := s.authRepo.GetOAuthAccountsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]authdto.LinkedAccountDTO, 0, len(accounts))
	for _, acc := range accounts {
		email := ""
		if acc.Email != nil {
			email = *acc.Email
		}
		name := ""
		if acc.Name != nil {
			name = *acc.Name
		}
		avatar := ""
		if acc.AvatarURL != nil {
			avatar = *acc.AvatarURL
		}
		result = append(result, authdto.LinkedAccountDTO{
			Provider: acc.Provider,
			Email:    email,
			Name:     name,
			Avatar:   avatar,
			LinkedAt: acc.CreatedAt.Format(time.RFC3339),
		})
	}
	return result, nil
}
