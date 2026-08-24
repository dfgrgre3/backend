package authservice

import (
	"context"
	"errors"
	"fmt"
	"time"
	"thanawy-backend/internal/application/services"
	notificationservice "thanawy-backend/internal/domain/notification/service"
	"thanawy-backend/internal/infrastructure/cache"

	"golang.org/x/crypto/bcrypt"
)

// ChangePassword validates the old password, hashes the new one, updates credentials,
// and revokes all existing sessions to force re-login on other devices.
func (s *authService) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	if err := validatePasswordPolicy(newPassword); err != nil {
		return err
	}

	credential, err := s.authRepo.GetCredentialByUserID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(credential.PasswordHash), []byte(oldPassword)); err != nil {
		return errors.New("current password is incorrect")
	}

	bcryptCost := s.cfg.BCryptCost
	if bcryptCost < bcrypt.MinCost || bcryptCost > 14 {
		bcryptCost = 12
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return errors.New("failed to hash password")
	}

	credential.PasswordHash = string(hashedPassword)
	credential.LastChangedAt = time.Now().UTC()
	if err := s.authRepo.UpdateCredential(ctx, credential); err != nil {
		return errors.New("failed to update password")
	}

	sessions, _ := s.authRepo.GetUserSessions(ctx, userID)
	for _, sess := range sessions {
		if sess.ID != "" {
			s.tokenService.BlacklistJTI(sess.ID, 15*time.Minute)
		}
	}
	s.authRepo.RevokeAllUserSessions(ctx, userID)

	return nil
}

// ForgotPassword generates a 6-digit verification code and emails it to the user.
// Returns nil even when the email does not exist to prevent user enumeration.
func (s *authService) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.authRepo.FindUserByEmail(ctx, email)
	if err != nil {
		return nil
	}

	code, err := generateSixDigitCode()
	if err != nil {
		return errors.New("failed to generate verification code")
	}

	if cache.Redis != nil {
		key := fmt.Sprintf("forgot_password_code:%s", email)
		if err := cache.Redis.Set(ctx, key, code, 15*time.Minute).Err(); err != nil {
			return errors.New("failed to store verification code")
		}
	}

	userName := user.Email
	if user.Name != nil {
		userName = *user.Name
	}
	body := notificationservice.GetForgotPasswordCodeEmailTemplate(userName, code)
	_ = services.GetMailQueueWorker().Enqueue(user.Email, "رمز استعادة كلمة المرور - Tolo Platform", body)

	return nil
}
