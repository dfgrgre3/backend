package authservice

import (
	"context"
	"errors"
	authdto "thanawy-backend/internal/application/dto"
	"time"
)

// ValidateAccessToken: the authService-level wrapper around
// AuthTokenService.ValidateAccessToken, returning the DTO shape callers
// expect. Split out of auth_service.go for readability — same package, same
// *authService receiver, no behavior change.

func (s *authService) ValidateAccessToken(ctx context.Context, token string) (*authdto.ValidateTokenResponse, error) {
	claims, err := s.tokenService.ValidateAccessToken(token)
	if err != nil {
		return nil, err
	}

	user, err := s.authRepo.FindUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	expiresAtStr := ""
	if claims.ExpiresAt != nil {
		expiresAtStr = claims.ExpiresAt.Time.Format(time.RFC3339)
	}

	return &authdto.ValidateTokenResponse{
		Valid:       true,
		UserID:      claims.UserID,
		Role:        claims.Role,
		Permissions: user.GetEffectivePermissions(),
		ExpiresAt:   expiresAtStr,
	}, nil
}
