package authservice

import (
	"context"
	"errors"
	authdto "thanawy-backend/internal/application/dto"
	models "thanawy-backend/internal/domain/common"
	notificationservice "thanawy-backend/internal/domain/notification/service"
	authrepo "thanawy-backend/internal/infrastructure/persistence/repositories"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func (s *authService) Register(ctx context.Context, req *authdto.RegisterRequest) (*authdto.RegisterResponse, error) {
	// Validate password policy
	if err := validatePasswordPolicy(req.Password); err != nil {
		return nil, err
	}

	// Check if user exists via repository
	existingUser, err := s.authRepo.FindUserByEmail(ctx, req.Email)
	if err == nil && existingUser != nil {
		return nil, errors.New("user with this email already exists")
	} else if err != nil && !authrepo.IsNotFound(err) {
		return nil, err
	}

	// Hash password with configured cost (defaults to 12, max 14 for security)
	bcryptCost := s.cfg.BCryptCost
	if bcryptCost < bcrypt.MinCost || bcryptCost > 14 {
		bcryptCost = 12
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		return nil, errors.New("failed to hash password")
	}

	fullName := req.FirstName + " " + req.LastName

	// Determine role (fix: use req.Role instead of always defaulting to STUDENT)
	role := models.RoleStudent
	if req.Role != "" {
		role = models.UserRole(req.Role)
		if !models.IsValidUserRole(role) {
			role = models.RoleStudent
		}
	}

	// Create user and credential in a single transaction via repository
	user := models.User{
		Email:  req.Email,
		Name:   &fullName,
		Role:   role,
		Status: models.StatusActive,
	}
	if req.Username != "" {
		user.Username = &req.Username
	}
	if req.Phone != "" {
		user.Phone = &req.Phone
	}
	credential := models.UserCredential{
		PasswordHash: string(hashedPassword),
	}

	if err := s.authRepo.CreateUserAndCredential(ctx, &user, &credential); err != nil {
		return nil, err
	}

	// Send email verification code via mail queue worker
	if s.mailQueue != nil {
		userName := user.Email
		if user.Name != nil {
			userName = *user.Name
		}
		if code, err := generateSixDigitCode(); err == nil {
			verificationCode := &models.VerificationCode{
				UserID:    user.ID,
				Code:      code,
				Type:      "email_verification",
				ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
			}
			if err := s.authRepo.CreateVerificationCode(ctx, verificationCode); err == nil {
				body := notificationservice.GetVerificationEmailTemplate(userName, code)
				_ = s.mailQueue.Enqueue(user.Email, "تأكيد البريد الإلكتروني - Tolo Platform", body)
			}
		}
	}

	return &authdto.RegisterResponse{
		Message: "User registered successfully",
		User: authdto.UserDTO{
			ID:    user.ID,
			Email: user.Email,
			Name:  *user.Name,
			Role:  string(user.Role),
		},
	}, nil
}
