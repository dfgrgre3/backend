package authservice

import (
	"context"
	"errors"
	"time"
	"thanawy-backend/internal/application/services"
	models "thanawy-backend/internal/domain/common"
	notificationservice "thanawy-backend/internal/domain/notification/service"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func (s *authService) InitiateAccountRecovery(ctx context.Context, email, method string) (string, error) {
	ticket := uuid.New().String()

	user, err := s.authRepo.FindUserByEmail(ctx, email)
	if err != nil {
		// Return a ticket even when the account doesn't exist — user-enumeration protection.
		return ticket, nil
	}

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

	_ = s.authRepo.MarkCodeAsUsed(ctx, vCode.ID)

	user, err := s.authRepo.FindUserByID(ctx, vCode.UserID)
	if err != nil {
		return errors.New("user not found")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return err
	}

	if err := s.authRepo.UpdatePasswordHash(ctx, user.ID, string(hashedPassword)); err != nil {
		return errors.New("failed to save new password")
	}

	return nil
}
