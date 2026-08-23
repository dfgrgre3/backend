package authservice

import (
	"context"
	"errors"
	models "thanawy-backend/internal/domain/common"
	"time"
)

// Session management: listing, revoking, and logging sign-in activity for an
// authenticated account. Split out of auth_service.go for readability — same
// package, same *authService receiver, no behavior change.

func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	session, err := s.authRepo.GetSessionByToken(ctx, refreshToken)
	if err != nil {
		// If not found, it's already logged out or invalid, consider it success
		return nil
	}

	return s.authRepo.RevokeSession(ctx, session.ID)
}

func (s *authService) GetUserSessions(ctx context.Context, userID string) ([]*models.UserSession, error) {
	return s.authRepo.GetUserSessions(ctx, userID)
}

func (s *authService) RevokeSession(ctx context.Context, userID, sessionID string) error {
	if _, err := s.authRepo.GetSessionByIDAndUser(ctx, sessionID, userID); err != nil {
		return errors.New("session not found")
	}
	err := s.authRepo.RevokeSession(ctx, sessionID)
	if err == nil {
		s.tokenService.BlacklistJTI(sessionID, 15*time.Minute)
	}
	return err
}

func (s *authService) RevokeAllSessions(ctx context.Context, userID string) error {
	sessions, err := s.authRepo.GetUserSessions(ctx, userID)
	if err == nil {
		for _, sess := range sessions {
			s.tokenService.BlacklistJTI(sess.ID, 15*time.Minute)
		}
	}
	return s.authRepo.RevokeAllUserSessions(ctx, userID)
}

func (s *authService) logFailedLogin(ctx context.Context, userID, ip, userAgent, reason string) {
	if userID == "" {
		return
	}

	history := &models.LoginHistory{
		UserID:    userID,
		IP:        &ip,
		UserAgent: &userAgent,
		Status:    "FAILED",
		Reason:    &reason,
	}
	s.authRepo.LogLoginHistory(ctx, history)
}
