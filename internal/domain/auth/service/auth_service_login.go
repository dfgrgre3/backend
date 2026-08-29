package authservice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	authdto "thanawy-backend/internal/application/dto"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/google/uuid"
)

func (s *authService) Login(ctx context.Context, req *authdto.LoginRequest, userAgent, ip string) (*authdto.LoginResponse, error) {
	// Find user via repository
	user, err := s.authRepo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		s.logFailedLogin(ctx, "", ip, userAgent, "Invalid credentials")
		return nil, errors.New("invalid email or password")
	}

	// Check account lockout before password verification
	if err := checkAccountLockout(ctx, user.ID); err != nil {
		s.logFailedLogin(ctx, user.ID, ip, userAgent, "Account locked")
		return nil, err
	}

	// Get user credential for password verification
	credential, err := s.authRepo.GetCredentialByUserID(ctx, user.ID)
	if err != nil {
		s.logFailedLogin(ctx, user.ID, ip, userAgent, "Invalid credentials")
		return nil, errors.New("invalid email or password")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(credential.PasswordHash), []byte(req.Password)); err != nil {
		recordFailedAttempt(ctx, user.ID)
		s.logFailedLogin(ctx, user.ID, ip, userAgent, "Invalid credentials")
		return nil, errors.New("invalid email or password")
	}

	// Check if active
	if user.Status != models.StatusActive {
		s.logFailedLogin(ctx, user.ID, ip, userAgent, "Account not active")
		return nil, errors.New("account is not active")
	}

	// Reset failed attempts on successful password verification
	resetFailedAttempts(ctx, user.ID)

	// Check MFA
	if user.TwoFactorEnabled {
		if cache.Redis == nil {
			return nil, errors.New("MFA is enabled but session cache is unavailable")
		}
		ticket := uuid.New().String()
		ticketKey := fmt.Sprintf("mfa_ticket:%s", ticket)
		if err := cache.Redis.Set(ctx, ticketKey, user.ID, 3*time.Minute).Err(); err != nil {
			return nil, fmt.Errorf("failed to generate MFA challenge: %w", err)
		}
		return nil, fmt.Errorf("MFA_REQUIRED:%s", ticket)
	}

	// Generate tokens
	tokenPair, err := s.tokenService.GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}

	// Parse UserAgent for OS and Browser
	os, browser := parseUserAgent(userAgent)

	// Save session
	session := &models.UserSession{
		ID:           tokenPair.JTI,
		UserID:       user.ID,
		RefreshToken: tokenPair.RefreshToken,
		UserAgent:    userAgent,
		Browser:      browser,
		OS:           os,
		IP:           ip,
		DeviceType:   detectDeviceType(userAgent),
		Status:       "active",
		IsActive:     true,
		LastAccessed: time.Now().UTC(),
		ExpiresAt:    time.Now().UTC().Add(30 * 24 * time.Hour),
	}

	if err := s.authRepo.CreateSession(ctx, session); err != nil {
		return nil, errors.New("failed to create session")
	}

	// Log successful login
	s.authRepo.LogLoginHistory(ctx, &models.LoginHistory{
		UserID:    user.ID,
		IP:        &ip,
		UserAgent: &userAgent,
		Status:    "SUCCESS",
	})

	name := ""
	if user.Name != nil {
		name = *user.Name
	}

	return &authdto.LoginResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		User: authdto.UserDTO{
			ID:            user.ID,
			Email:         user.Email,
			Name:          name,
			Role:          string(user.Role),
			EmailVerified: user.EmailVerified,
			PhoneVerified: user.PhoneVerified,
			Permissions:   user.GetEffectivePermissions(),
		},
	}, nil
}

func (s *authService) RefreshToken(ctx context.Context, refreshToken, userAgent, ip string) (*authdto.RefreshTokenResponse, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, errors.New("refresh token is required")
	}

	oldHash := models.ComputeRefreshTokenHash(refreshToken)

	res, err, _ := s.refreshSF.Do(oldHash, func() (interface{}, error) {
		return s.doRefreshToken(ctx, oldHash, userAgent, ip)
	})
	if err != nil {
		return nil, err
	}
	return res.(*authdto.RefreshTokenResponse), nil
}

func (s *authService) doRefreshToken(ctx context.Context, oldHash, userAgent, ip string) (*authdto.RefreshTokenResponse, error) {
	session, err := s.authRepo.GetSessionByHashOrdered(ctx, oldHash)
	if err != nil {
		return nil, errors.New("invalid or expired refresh token")
	}

	if !session.IsActive {
		// Check if it was revoked/rotated within the 15-second grace period (e.g. concurrent/rapid client requests)
		if session.Status == "revoked" && session.RevokedAt != nil && time.Since(*session.RevokedAt) <= 15*time.Second {
			replacementSession, lookupErr := s.authRepo.GetActiveReplacementSession(ctx, session.UserID, session.RevokedAt.Add(-2*time.Second))
			if lookupErr == nil && replacementSession != nil && replacementSession.IsActive {
				user, userErr := s.authRepo.FindUserByID(ctx, session.UserID)
				if userErr == nil {
					accessToken, tokenErr := s.tokenService.GenerateAccessTokenForSession(user, replacementSession.ID)
					if tokenErr == nil {
						return &authdto.RefreshTokenResponse{
							AccessToken:  accessToken,
							RefreshToken: replacementSession.RefreshToken,
						}, nil
					}
				}
			}
		}

		// Token reuse outside grace period detected! Revoke all sessions for this user immediately
		_ = s.authRepo.RevokeAllUserSessions(ctx, session.UserID)
		return nil, errors.New("security alert: reuse of rotated refresh token detected, all sessions revoked")
	}

	user, err := s.authRepo.FindUserByID(ctx, session.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if session.ExpiresAt.Before(time.Now()) {
		s.authRepo.RevokeSession(ctx, session.ID)
		return nil, errors.New("refresh token expired")
	}

	// Generate new tokens
	tokenPair, err := s.tokenService.GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}

	// Build the new session
	os, browser := parseUserAgent(userAgent)
	newSession := &models.UserSession{
		ID:           tokenPair.JTI,
		UserID:       user.ID,
		RefreshToken: tokenPair.RefreshToken,
		UserAgent:    userAgent,
		Browser:      browser,
		OS:           os,
		IP:           ip,
		DeviceType:   detectDeviceType(userAgent),
		Status:       "active",
		IsActive:     true,
		LastAccessed: time.Now().UTC(),
		ExpiresAt:    time.Now().UTC().Add(30 * 24 * time.Hour),
	}

	// Rotate refresh token atomically via repository
	if err := s.authRepo.RotateSession(ctx, session.ID, newSession); err != nil {
		return nil, err
	}

	// Blacklist the old access token (since its JTI is session.ID) - fire and forget
	go func(jti string) {
		_, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		s.tokenService.BlacklistJTI(jti, 15*time.Minute)
	}(session.ID)

	return &authdto.RefreshTokenResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	}, nil
}
