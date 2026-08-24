package authservice

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"

	"golang.org/x/crypto/bcrypt"
)

func (s *authService) VerifyForgotPasswordCode(ctx context.Context, email, code string) (string, error) {
	user, err := s.authRepo.FindUserByEmail(ctx, email)
	if err != nil {
		return "", errors.New("invalid email")
	}

	if cache.Redis == nil {
		return "", errors.New("verification service unavailable")
	}

	key := fmt.Sprintf("forgot_password_code:%s", email)
	storedCode, err := cache.Redis.Get(ctx, key).Result()
	if err != nil {
		return "", errors.New("invalid or expired verification code")
	}
	if storedCode != code {
		return "", errors.New("invalid verification code")
	}

	rawToken := make([]byte, 32)
	if _, err := rand.Read(rawToken); err != nil {
		return "", errors.New("failed to generate reset token")
	}
	tokenStr := hex.EncodeToString(rawToken)

	tokenHash := sha256.Sum256([]byte(tokenStr))
	tokenHashStr := hex.EncodeToString(tokenHash[:])

	resetToken := &models.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: tokenHashStr,
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	}
	if err := s.authRepo.CreatePasswordResetToken(ctx, resetToken); err != nil {
		return "", errors.New("failed to create reset token")
	}

	cache.Redis.Del(ctx, key)
	return tokenStr, nil
}

func (s *authService) ResetPassword(ctx context.Context, token, newPassword string) error {
	if err := validatePasswordPolicy(newPassword); err != nil {
		return err
	}

	tokenHash := sha256.Sum256([]byte(token))
	tokenHashStr := hex.EncodeToString(tokenHash[:])

	resetToken, err := s.authRepo.FindPasswordResetTokenByHash(ctx, tokenHashStr)
	if err != nil {
		return errors.New("invalid or expired reset token")
	}

	_ = s.authRepo.MarkPasswordResetTokenUsed(ctx, resetToken.ID)

	bcryptCost := s.cfg.BCryptCost
	if bcryptCost < bcrypt.MinCost || bcryptCost > 14 {
		bcryptCost = 12
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return errors.New("failed to hash password")
	}

	if err := s.authRepo.UpdatePasswordHash(ctx, resetToken.UserID, string(hashedPassword)); err != nil {
		return errors.New("failed to update password")
	}

	_ = s.RevokeAllSessions(ctx, resetToken.UserID)
	return nil
}
