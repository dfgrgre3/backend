package services

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"strings"
	"thanawy-backend/internal/config"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"
	"thanawy-backend/internal/repository"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var dummyHash string

func init() {
	// Generate a dummy hash at startup to use for timing attack protection when a user is not found.
	// SECURITY: Any previous hard-coded secret is revoked and replaced with runtime-generated random bytes
	// to prevent credential exposure and address Sonar rule S6437.
	randomBytes := make([]byte, 64)
	if _, err := rand.Read(randomBytes); err != nil {
		// Panic if we cannot generate secure random bytes, as this is a critical security failure.
		panic("failed to generate secure random bytes for auth timing protection")
	}
	h, _ := bcrypt.GenerateFromPassword(randomBytes, 12)
	dummyHash = string(h)
}

type AuthService struct {
	repo *repository.UserRepository
}

func NewAuthService(repo *repository.UserRepository) *AuthService {
	return &AuthService{repo: repo}
}

func (s *AuthService) getRepo() *repository.UserRepository {
	return s.repo
}

type RegisterInput struct {
	Email          string
	Username       string
	Password       string
	Role           models.UserRole
	IP             string
	UserAgent      string
	Phone          string
	GradeLevel     string
	EducationType  string
	Section        string
	ReferredByCode string
}

func (s *AuthService) Register(input RegisterInput) (*models.User, error) {
	// 1. Normalize email
	email := strings.ToLower(strings.TrimSpace(input.Email))

	// 2. Check if user exists
	_, err := s.getRepo().FindByEmail(email)
	if err == nil {
		// User already exists
		return nil, errors.New("user already exists")
	}

	// 3. Hash password
	cfg := config.Load()
	cost := cfg.BCryptCost
	if cost < 12 {
		cost = 12
	}
	if cost > bcrypt.MaxCost {
		cost = bcrypt.MaxCost
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), cost)
	if err != nil {
		return nil, err
	}

	// 4. Create user
	role := input.Role
	if role == models.RoleAdmin || role == models.RoleSuperAdmin {
		role = models.RoleStudent
	}

	user := models.User{
		Email:         email,
		Username:      &input.Username,
		PasswordHash:  string(hashedPassword),
		Role:          role,
		Status:        models.StatusActive,
		EmailVerified: false,
		Phone:         &input.Phone,
		GradeLevel:    &input.GradeLevel,
		EducationType: &input.EducationType,
		Section:       &input.Section,
	}

	if err := s.getRepo().Create(&user); err != nil {
		return nil, err
	}

	// 6. Log security event (TBD)

	return &user, nil
}

func (s *AuthService) Login(email, password, ip, userAgent string) (*models.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.getRepo().FindByEmailNoCache(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Timing safe: still do a bcrypt compare
			bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
			return nil, errors.New("invalid email or password")
		}
		return nil, err
	}

	// Check password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		log.Printf("Password mismatch for session ip=%s", ip)
		return nil, errors.New("invalid email or password")
	}

	// Check account is active (do not leak account status to unauthenticated callers)
	if user.Status != models.StatusActive {
		return nil, errors.New("invalid email or password")
	}

	// Require email verification before allowing login
	if !user.EmailVerified {
		return nil, errors.New("email not verified")
	}

	return user, nil
}

func (s *AuthService) generateRandomString(n int) (string, error) {
	byteLen := n
	if byteLen < 32 {
		byteLen = 32
	}
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate secure token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	if len(token) > n {
		return token[:n], nil
	}
	return token, nil
}

func (s *AuthService) RequestMagicLink(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := s.getRepo().FindByEmailNoCache(email)
	if err != nil {
		return "", err
	}

	token, err := s.generateRandomString(32)
	if err != nil {
		return "", err
	}
	expires := time.Now().Add(15 * time.Minute)

	user.MagicLinkToken = &token
	user.MagicLinkExpires = &expires

	if err := s.getRepo().Update(user); err != nil {
		return "", err
	}

	return token, nil
}

func (s *AuthService) VerifyMagicLink(token string) (*models.User, error) {
	if !IsValidToken(token) {
		return nil, errors.New("invalid or expired magic link")
	}
	var user models.User
	if err := db.DB.Where("magic_link_token = ? AND magic_link_expires > ?", token, time.Now()).First(&user).Error; err != nil {
		return nil, errors.New("invalid or expired magic link")
	}

	// Clear token
	user.MagicLinkToken = nil
	user.MagicLinkExpires = nil
	if err := s.getRepo().Update(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *AuthService) RequestPasswordReset(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := s.getRepo().FindByEmailNoCache(email)
	if err != nil {
		return "", err
	}

	token, err := s.generateRandomString(32)
	if err != nil {
		return "", err
	}
	expires := time.Now().Add(1 * time.Hour)

	user.ResetPasswordToken = &token
	user.ResetPasswordExpires = &expires

	if err := s.getRepo().Update(user); err != nil {
		return "", err
	}

	return token, nil
}

func (s *AuthService) ResetPassword(token, newPassword string) error {
	if !IsValidToken(token) {
		return errors.New("invalid or expired reset token")
	}
	var user models.User
	if err := db.DB.Where("reset_password_token = ? AND reset_password_expires > ?", token, time.Now()).First(&user).Error; err != nil {
		return errors.New("invalid or expired reset token")
	}

	cfg := config.Load()
	cost := cfg.BCryptCost
	if cost < 12 {
		cost = 12
	}
	if cost > bcrypt.MaxCost {
		cost = bcrypt.MaxCost
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), cost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hashedPassword)
	user.ResetPasswordToken = nil
	user.ResetPasswordExpires = nil

	return s.getRepo().Update(&user)
}

func (s *AuthService) RequestEmailVerification(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := s.getRepo().FindByEmailNoCache(email)
	if err != nil {
		return "", err
	}

	token, err := s.generateRandomString(32)
	if err != nil {
		return "", err
	}
	expires := time.Now().Add(24 * time.Hour)

	user.VerificationToken = &token
	user.VerificationExpires = &expires

	if err := s.getRepo().Update(user); err != nil {
		return "", err
	}

	return token, nil
}

func (s *AuthService) VerifyEmail(token string) error {
	if !IsValidToken(token) {
		return errors.New("invalid or expired verification token")
	}
	var user models.User
	if err := db.DB.Where("verification_token = ? AND verification_expires > ?", token, time.Now()).First(&user).Error; err != nil {
		return errors.New("invalid or expired verification token")
	}

	user.EmailVerified = true
	user.VerificationToken = nil
	user.VerificationExpires = nil

	return s.getRepo().Update(&user)
}

func IsValidToken(token string) bool {
	if len(token) != 43 {
		return false
	}
	for _, r := range token {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
