package authservice

import (
	"context"
	"errors"
	authdto "thanawy-backend/internal/application/dto"
	"thanawy-backend/internal/application/services"
	models "thanawy-backend/internal/domain/common"
	notificationservice "thanawy-backend/internal/domain/notification/service"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Profile and email-verification: verifying/resending the email code,
// reading/updating the current user's profile, and account deletion. Split
// out of auth_service.go for readability — same package, same *authService
// receiver, no behavior change.

func (s *authService) VerifyEmail(ctx context.Context, userID, code string) error {
	verificationCode, err := s.authRepo.GetVerificationCode(ctx, userID, "email_verification", code)
	if err != nil {
		return errors.New("invalid or expired verification code")
	}

	if verificationCode.ExpiresAt.Before(time.Now().UTC()) {
		return errors.New("verification code has expired")
	}

	// Mark code as used
	s.authRepo.MarkCodeAsUsed(ctx, verificationCode.ID)

	// Update user email verified status via repository
	user, err := s.authRepo.FindUserByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}
	user.EmailVerified = true
	if err := s.authRepo.SaveUser(ctx, user); err != nil {
		return errors.New("failed to verify email")
	}

	// Send Welcome Email
	if user.ID != "" {
		userName := user.Email
		if user.Name != nil {
			userName = *user.Name
		}
		welcomeBody := notificationservice.GetWelcomeEmailTemplate(userName, string(user.Role))
		_ = services.GetMailQueueWorker().Enqueue(user.Email, "مرحباً بك في منصة Tolo التعليمية", welcomeBody)
	}

	return nil
}

func (s *authService) ResendVerificationEmail(ctx context.Context, userID string) error {
	user, err := s.authRepo.FindUserByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	if user.EmailVerified {
		return errors.New("email is already verified")
	}

	// Generate a 6-digit verification code
	code, err := generateSixDigitCode()
	if err != nil {
		return errors.New("failed to generate verification code")
	}

	verificationCode := &models.VerificationCode{
		UserID:    user.ID,
		Code:      code,
		Type:      "email_verification",
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	}
	if err := s.authRepo.CreateVerificationCode(ctx, verificationCode); err != nil {
		return errors.New("failed to create verification code")
	}

	// Send verification email via queue worker
	userName := user.Email
	if user.Name != nil {
		userName = *user.Name
	}
	body := notificationservice.GetVerificationEmailTemplate(userName, code)
	_ = services.GetMailQueueWorker().Enqueue(user.Email, "تأكيد البريد الإلكتروني - Tolo Platform", body)

	return nil
}

func (s *authService) GetCurrentUser(ctx context.Context, userID string) (*authdto.UserDTO, error) {
	user, err := s.authRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	name := ""
	if user.Name != nil {
		name = *user.Name
	}
	username := ""
	if user.Username != nil {
		username = *user.Username
	}
	avatar := ""
	if user.Avatar != nil {
		avatar = *user.Avatar
	}

	return &authdto.UserDTO{
		ID:            user.ID,
		Email:         user.Email,
		Name:          name,
		Username:      username,
		Avatar:        avatar,
		Role:          string(user.Role),
		Status:        string(user.Status),
		EmailVerified: user.EmailVerified,
		PhoneVerified: user.PhoneVerified,
		Permissions:   user.GetEffectivePermissions(),
	}, nil
}

func (s *authService) UpdateProfile(ctx context.Context, userID string, req *authdto.UpdateProfileRequest) (*authdto.UserDTO, error) {
	user, err := s.authRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if req.Name != "" {
		user.Name = &req.Name
	}
	if req.Username != "" {
		user.Username = &req.Username
	}
	if req.Avatar != "" {
		user.Avatar = &req.Avatar
	}
	if req.Phone != "" {
		user.Phone = &req.Phone
	}
	if req.Bio != "" {
		user.Bio = &req.Bio
	}
	if req.Country != "" {
		user.Country = &req.Country
	}

	if err := s.authRepo.SaveUser(ctx, user); err != nil {
		return nil, errors.New("failed to update profile")
	}

	profile, profileErr := s.authRepo.GetProfileByUserID(ctx, userID)
	if profileErr == nil {
		if req.Name != "" {
			profile.Name = req.Name
		}
		if req.Avatar != "" {
			profile.AvatarURL = &req.Avatar
		}
		if req.Language != "" {
			profile.Language = req.Language
		}
		if req.Timezone != "" {
			profile.Timezone = req.Timezone
		}
		_ = s.authRepo.SaveProfile(ctx, profile)
	}

	return s.GetCurrentUser(ctx, userID)
}

func (s *authService) DeleteAccount(ctx context.Context, userID, password, reason string) error {
	user, err := s.authRepo.FindUserByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	// Get user credential via repository for password verification
	credential, err := s.authRepo.GetCredentialByUserID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(credential.PasswordHash), []byte(password)); err != nil {
		return errors.New("invalid password")
	}

	s.RevokeAllSessions(ctx, userID)

	now := time.Now().UTC()
	user.DeletedAt = gorm.DeletedAt{Time: now, Valid: true}
	user.Status = "INACTIVE"
	user.ArchiveReason = &reason
	if err := s.authRepo.SaveUser(ctx, user); err != nil {
		return errors.New("failed to delete account")
	}

	return nil
}
