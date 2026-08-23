package authservice

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"thanawy-backend/internal/application/services"
	models "thanawy-backend/internal/domain/common"
	notificationservice "thanawy-backend/internal/domain/notification/service"
	"thanawy-backend/internal/infrastructure/cache"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Password and account-recovery flows: change/forgot/reset password, and the
// email-based account recovery ticket flow. Split out of auth_service.go for
// readability — same package, same *authService receiver, no behavior change.

func (s *authService) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	// Validate new password policy
	if err := validatePasswordPolicy(newPassword); err != nil {
		return err
	}

	// Get user credential via repository
	credential, err := s.authRepo.GetCredentialByUserID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(credential.PasswordHash), []byte(oldPassword)); err != nil {
		return errors.New("current password is incorrect")
	}

	// Hash new password with configured cost (max 14 for security)
	bcryptCost := s.cfg.BCryptCost
	if bcryptCost < bcrypt.MinCost || bcryptCost > 14 {
		bcryptCost = 12
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return errors.New("failed to hash password")
	}

	// Update credential via repository
	credential.PasswordHash = string(hashedPassword)
	credential.LastChangedAt = time.Now().UTC()
	if err := s.authRepo.UpdateCredential(ctx, credential); err != nil {
		return errors.New("failed to update password")
	}

	// Revoke all sessions (force re-login on other devices)
	sessions, _ := s.authRepo.GetUserSessions(ctx, userID)
	for _, sess := range sessions {
		if sess.ID != "" {
			s.tokenService.BlacklistJTI(sess.ID, 15*time.Minute)
		}
	}
	s.authRepo.RevokeAllUserSessions(ctx, userID)

	return nil
}

func (s *authService) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.authRepo.FindUserByEmail(ctx, email)
	if err != nil {
		// Return nil even if user not found — prevents user enumeration
		return nil
	}

	// Generate a 6-digit verification code
	code, err := generateSixDigitCode()
	if err != nil {
		return errors.New("failed to generate verification code")
	}

	// Store the verification code in Redis with 15-minute expiry
	if cache.Redis != nil {
		key := fmt.Sprintf("forgot_password_code:%s", email)
		if err := cache.Redis.Set(ctx, key, code, 15*time.Minute).Err(); err != nil {
			return errors.New("failed to store verification code")
		}
	}

	// Send verification code via email
	userName := user.Email
	if user.Name != nil {
		userName = *user.Name
	}
	body := notificationservice.GetForgotPasswordCodeEmailTemplate(userName, code)
	_ = services.GetMailQueueWorker().Enqueue(user.Email, "رمز استعادة كلمة المرور - Tolo Platform", body)

	return nil
}

func (s *authService) VerifyForgotPasswordCode(ctx context.Context, email, code string) (string, error) {
	user, err := s.authRepo.FindUserByEmail(ctx, email)
	if err != nil {
		return "", errors.New("invalid email")
	}

	// Verify the code from Redis
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

	// Generate a reset token for the password reset step
	rawToken := make([]byte, 32)
	if _, err := rand.Read(rawToken); err != nil {
		return "", errors.New("failed to generate reset token")
	}
	tokenStr := hex.EncodeToString(rawToken)

	// Store the reset token with 15-minute expiry via repository
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

	// Delete the verification code after successful verification
	cache.Redis.Del(ctx, key)

	return tokenStr, nil
}

func (s *authService) ResetPassword(ctx context.Context, token, newPassword string) error {
	// Validate password policy
	if err := validatePasswordPolicy(newPassword); err != nil {
		return err
	}

	// Hash the provided token and look up the matching record
	tokenHash := sha256.Sum256([]byte(token))
	tokenHashStr := hex.EncodeToString(tokenHash[:])

	resetToken, err := s.authRepo.FindPasswordResetTokenByHash(ctx, tokenHashStr)
	if err != nil {
		return errors.New("invalid or expired reset token")
	}

	// Mark token as used
	_ = s.authRepo.MarkPasswordResetTokenUsed(ctx, resetToken.ID)

	// Hash new password (max 14 for security)
	bcryptCost := s.cfg.BCryptCost
	if bcryptCost < bcrypt.MinCost || bcryptCost > 14 {
		bcryptCost = 12
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return errors.New("failed to hash password")
	}

	// Update user credential via repository
	if err := s.authRepo.UpdatePasswordHash(ctx, resetToken.UserID, string(hashedPassword)); err != nil {
		return errors.New("failed to update password")
	}

	// Revoke all sessions for this user (force re-login)
	_ = s.RevokeAllSessions(ctx, resetToken.UserID)

	return nil
}

func (s *authService) InitiateAccountRecovery(ctx context.Context, email, method string) (string, error) {
	user, err := s.authRepo.FindUserByEmail(ctx, email)
	if err != nil {
		return "", errors.New("user not found")
	}

	ticket := uuid.New().String()
	code, err := generateSixDigitCode()
	if err != nil {
		return "", err
	}

	verificationCode := &models.VerificationCode{
		UserID:    user.ID,
		Code:      code,
		Type:      "account_recovery:" + ticket,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	if err := s.authRepo.CreateVerificationCode(ctx, verificationCode); err != nil {
		return "", err
	}

	// Send recovery code via queue worker
	userName := user.Email
	if user.Name != nil {
		userName = *user.Name
	}
	body := notificationservice.GetVerificationEmailTemplate(userName, code)
	_ = services.GetMailQueueWorker().Enqueue(user.Email, "رمز استعادة الحساب - Tolo Platform", body)

	return ticket, nil
}

func (s *authService) FinalizeAccountRecovery(ctx context.Context, ticket, code, newPassword string) error {
	vCode, err := s.authRepo.FindVerificationCodeByCodeAndType(ctx, code, "account_recovery:"+ticket)
	if err != nil {
		return errors.New("invalid or expired verification code")
	}

	// Mark code as used
	_ = s.authRepo.MarkCodeAsUsed(ctx, vCode.ID)

	user, err := s.authRepo.FindUserByID(ctx, vCode.UserID)
	if err != nil {
		return errors.New("user not found")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return err
	}

	// Update user credential via repository
	if err := s.authRepo.UpdatePasswordHash(ctx, user.ID, string(hashedPassword)); err != nil {
		return errors.New("failed to save new password")
	}

	return nil
}
