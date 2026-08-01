package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"thanawy-backend/internal/api/dto"
	"thanawy-backend/internal/config"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"
	"thanawy-backend/internal/repository"

	"github.com/google/uuid"
)

// Lockout Policy Configuration
const (
	maxFailedAttempts    = 5                // عدد المحاولات الفاشلة المسموحة
	lockoutDuration      = 15 * time.Minute // مدة قفل الحساب (15 دقيقة)
	failedAttemptsPrefix = "failed_attempts:"
	lockoutPrefix        = "lockout:"
)

// getFailedAttemptsKey returns Redis key for counting failed login attempts
func getFailedAttemptsKey(userID string) string {
	return fmt.Sprintf("%s%s", failedAttemptsPrefix, userID)
}

// getLockoutKey returns Redis key for account lockout
func getLockoutKey(userID string) string {
	return fmt.Sprintf("%s%s", lockoutPrefix, userID)
}

// checkAccountLockout checks if the account is currently locked out
// Returns nil if not locked, or an error with remaining time message
func checkAccountLockout(ctx context.Context, userID string) error {
	if db.Redis == nil {
		return nil // Lockout is disabled without Redis
	}

	lockoutKey := getLockoutKey(userID)
	ttl, err := db.Redis.TTL(ctx, lockoutKey).Result()
	if err != nil || ttl <= 0 {
		return nil // Not locked out
	}

	minutesRemaining := int(ttl.Minutes()) + 1
	return fmt.Errorf("account locked due to too many failed attempts. Try again in %d minute(s)", minutesRemaining)
}

// recordFailedAttempt increments the failed attempt counter.
// Locks the account if threshold is reached.
func recordFailedAttempt(ctx context.Context, userID string) {
	if db.Redis == nil || userID == "" {
		return
	}

	failedKey := getFailedAttemptsKey(userID)
	lockoutKey := getLockoutKey(userID)

	// Increment failed attempts counter
	count, err := db.Redis.Incr(ctx, failedKey).Result()
	if err != nil {
		return
	}

	// Set expiry on first attempt to track over rolling window
	if count == 1 {
		db.Redis.Expire(ctx, failedKey, lockoutDuration)
	}

	// Lock account if threshold is reached
	if count >= maxFailedAttempts {
		db.Redis.Set(ctx, lockoutKey, "locked", lockoutDuration)
		db.Redis.Del(ctx, failedKey) // Reset failed counter after locking
	}
}

// resetFailedAttempts clears the failed attempt counter on successful login
func resetFailedAttempts(ctx context.Context, userID string) {
	if db.Redis == nil || userID == "" {
		return
	}
	db.Redis.Del(ctx, getFailedAttemptsKey(userID))
	db.Redis.Del(ctx, getLockoutKey(userID))
}

// ─────────────────────────────────────────────
//  Password Policy
// ─────────────────────────────────────────────

// validatePasswordPolicy enforces password complexity requirements.
// Rules: min 8 chars, at least 1 uppercase, 1 lowercase, 1 digit, 1 special char.
// Also rejects commonly breached passwords.
func validatePasswordPolicy(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}
	if len(password) > 128 {
		return errors.New("password must not exceed 128 characters")
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if !hasDigit {
		return errors.New("password must contain at least one digit")
	}
	if !hasSpecial {
		return errors.New("password must contain at least one special character (!@#$%^&*...)")
	}

	// Reject common weak passwords
	lower := strings.ToLower(password)
	commonPasswords := []string{
		"password", "12345678", "qwerty12", "admin123", "letmein1",
		"welcome1", "password1", "123456789", "1234567890",
	}
	for _, weak := range commonPasswords {
		if lower == weak {
			return errors.New("this password is too common, please choose a stronger one")
		}
	}

	return nil
}

// Simple user agent parser for basic info
func parseUserAgent(ua string) (os string, browser string) {
	uaLower := strings.ToLower(ua)

	// Basic OS detection
	if strings.Contains(uaLower, "windows") {
		os = "Windows"
	} else if strings.Contains(uaLower, "mac os") || strings.Contains(uaLower, "macos") {
		os = "MacOS"
	} else if strings.Contains(uaLower, "linux") {
		os = "Linux"
	} else if strings.Contains(uaLower, "android") {
		os = "Android"
	} else if strings.Contains(uaLower, "iphone") || strings.Contains(uaLower, "ipad") {
		os = "iOS"
	} else {
		os = "Unknown OS"
	}

	// Basic Browser detection
	if strings.Contains(uaLower, "chrome") && !strings.Contains(uaLower, "edg") {
		browser = "Chrome"
	} else if strings.Contains(uaLower, "safari") && !strings.Contains(uaLower, "chrome") {
		browser = "Safari"
	} else if strings.Contains(uaLower, "firefox") {
		browser = "Firefox"
	} else if strings.Contains(uaLower, "edg") {
		browser = "Edge"
	} else {
		browser = "Unknown Browser"
	}

	return os, browser
}

type AuthService interface {
	Register(ctx context.Context, req *dto.RegisterRequest) (*dto.RegisterResponse, error)
	Login(ctx context.Context, req *dto.LoginRequest, userAgent, ip string) (*dto.LoginResponse, error)
	RefreshToken(ctx context.Context, refreshToken, userAgent, ip string) (*dto.RefreshTokenResponse, error)
	Logout(ctx context.Context, refreshToken string) error
	GetUserSessions(ctx context.Context, userID string) ([]*models.UserSession, error)
	RevokeSession(ctx context.Context, userID, sessionID string) error
	RevokeAllSessions(ctx context.Context, userID string) error
	ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error
	ForgotPassword(ctx context.Context, email string) error
	VerifyForgotPasswordCode(ctx context.Context, email, code string) (string, error)
	ResetPassword(ctx context.Context, token, newPassword string) error
	VerifyEmail(ctx context.Context, userID, code string) error
	ResendVerificationEmail(ctx context.Context, userID string) error
	GetCurrentUser(ctx context.Context, userID string) (*dto.UserDTO, error)
	UpdateProfile(ctx context.Context, userID string, req *dto.UpdateProfileRequest) (*dto.UserDTO, error)
	DeleteAccount(ctx context.Context, userID, password, reason string) error
	GetOAuthRedirectURL(ctx context.Context, provider string) (string, error)
	HandleOAuthCallback(ctx context.Context, provider, code, state, userAgent, ip string) (*dto.LoginResponse, error)
	LinkOAuthProvider(ctx context.Context, userID, provider, code, state string) error
	UnlinkOAuthProvider(ctx context.Context, userID, provider string) error
	GetLinkedAccounts(ctx context.Context, userID string) ([]dto.LinkedAccountDTO, error)
	ValidateAccessToken(ctx context.Context, token string) (*dto.ValidateTokenResponse, error)
	InitiateAccountRecovery(ctx context.Context, email, method string) (string, error)
	FinalizeAccountRecovery(ctx context.Context, ticket, code, newPassword string) error
}

type authService struct {
	authRepo     repository.AuthRepository
	tokenService AuthTokenService
	oauthService OAuthService
	auditService *AuditService
	cfg          *config.Config
	mailQueue    *MailQueueWorker
}

func NewAuthService(authRepo repository.AuthRepository, tokenService AuthTokenService, oauthService OAuthService, cfg *config.Config, mailQueue *MailQueueWorker) AuthService {
	return &authService{
		authRepo:     authRepo,
		tokenService: tokenService,
		oauthService: oauthService,
		auditService: GetAuditService(),
		cfg:          cfg,
		mailQueue:    mailQueue,
	}
}

func (s *authService) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.RegisterResponse, error) {
	// Validate password policy
	if err := validatePasswordPolicy(req.Password); err != nil {
		return nil, err
	}

	// Check if user exists
	var existingUser models.User
	err := db.DB.WithContext(ctx).Where("email = ?", req.Email).First(&existingUser).Error
	if err == nil {
		return nil, errors.New("user with this email already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
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

	// Create user and credential in a transaction
	var user models.User
	if err := db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user = models.User{
			Email:  req.Email,
			Name:   &fullName,
			Role:   models.RoleStudent,
			Status: models.StatusActive,
		}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		credential := models.UserCredential{
			UserID:       user.ID,
			PasswordHash: string(hashedPassword),
		}
		return tx.Create(&credential).Error
	}); err != nil {
		return nil, err
	}

	// Send email verification code via mail queue worker
	if s.mailQueue != nil {
		userName := user.Email
		if user.Name != nil {
			userName = *user.Name
		}
		codeBytes := make([]byte, 3)
		if _, err := rand.Read(codeBytes); err == nil {
			code := fmt.Sprintf("%06d", int(codeBytes[0])<<16|int(codeBytes[1])<<8|int(codeBytes[2])%1000000)
			if len(code) > 6 {
				code = code[:6]
			}
			verificationCode := &models.VerificationCode{
				UserID:    user.ID,
				Code:      code,
				Type:      "email_verification",
				ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
			}
			if err := s.authRepo.CreateVerificationCode(ctx, verificationCode); err == nil {
				body := GetVerificationEmailTemplate(userName, code)
				_ = s.mailQueue.Enqueue(user.Email, "تأكيد البريد الإلكتروني - Tolo Platform", body)
			}
		}
	}

	return &dto.RegisterResponse{
		Message: "User registered successfully",
		User: dto.UserDTO{
			ID:    user.ID,
			Email: user.Email,
			Name:  *user.Name,
			Role:  string(user.Role),
		},
	}, nil
}

func (s *authService) Login(ctx context.Context, req *dto.LoginRequest, userAgent, ip string) (*dto.LoginResponse, error) {
	// Find user
	var user models.User
	if err := db.DB.WithContext(ctx).Where("email = ?", req.Email).First(&user).Error; err != nil {
		s.logFailedLogin(ctx, "", ip, userAgent, "Invalid credentials")
		return nil, errors.New("invalid email or password")
	}

	// Check account lockout before password verification
	if err := checkAccountLockout(ctx, user.ID); err != nil {
		s.logFailedLogin(ctx, user.ID, ip, userAgent, "Account locked")
		return nil, err
	}

	// Get user credential for password verification
	var credential models.UserCredential
	if err := db.DB.WithContext(ctx).Where("user_id = ?", user.ID).First(&credential).Error; err != nil {
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
		if db.Redis == nil {
			return nil, errors.New("MFA is enabled but session cache is unavailable")
		}
		ticket := uuid.New().String()
		ticketKey := fmt.Sprintf("mfa_ticket:%s", ticket)
		if err := db.Redis.Set(ctx, ticketKey, user.ID, 3*time.Minute).Err(); err != nil {
			return nil, fmt.Errorf("failed to generate MFA challenge: %w", err)
		}
		return nil, fmt.Errorf("MFA_REQUIRED:%s", ticket)
	}

	// Generate tokens
	tokenPair, err := s.tokenService.GenerateTokenPair(&user)
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

	return &dto.LoginResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		User: dto.UserDTO{
			ID:    user.ID,
			Email: user.Email,
			Name:  name,
			Role:  string(user.Role),
		},
	}, nil
}

func (s *authService) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	// Validate new password policy
	if err := validatePasswordPolicy(newPassword); err != nil {
		return err
	}

	// Get user credential
	var credential models.UserCredential
	if err := db.DB.WithContext(ctx).Where("user_id = ?", userID).First(&credential).Error; err != nil {
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

	// Update credential
	credential.PasswordHash = string(hashedPassword)
	now := time.Now().UTC()
	credential.LastChangedAt = now

	// Save new password
	if err := db.DB.WithContext(ctx).Save(&credential).Error; err != nil {
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

func (s *authService) RefreshToken(ctx context.Context, refreshToken, userAgent, ip string) (*dto.RefreshTokenResponse, error) {
	// Replay Attack Protection: Check if this token was already revoked/rotated.
	// Look up by hash to avoid querying with the raw refresh token.
	var oldSession models.UserSession
	oldHash := models.ComputeRefreshTokenHash(refreshToken)
	if err := db.DB.WithContext(ctx).Where("refresh_token_hash = ?", oldHash).First(&oldSession).Error; err == nil {
		if !oldSession.IsActive {
			// Check if it was revoked/rotated within the 10-second grace period (e.g. slow client double-requests)
			if oldSession.Status == "revoked" && oldSession.RevokedAt != nil && time.Since(*oldSession.RevokedAt) <= 10*time.Second {
				var replacementSession models.UserSession
				err := db.DB.WithContext(ctx).
					Where("user_id = ? AND is_active = ? AND created_at >= ?", oldSession.UserID, true, oldSession.RevokedAt.Add(-2*time.Second)).
					Order("created_at DESC").
					First(&replacementSession).Error
				if err == nil {
					var user models.User
					if err := db.DB.WithContext(ctx).Where("id = ?", oldSession.UserID).First(&user).Error; err == nil {
						// Generate a new access token for this replacement session
						accessToken, err := s.tokenService.GenerateAccessTokenForSession(&user, replacementSession.ID)
						if err == nil {
							return &dto.RefreshTokenResponse{
								AccessToken:  accessToken,
								RefreshToken: replacementSession.RefreshToken,
							}, nil
						}
					}
				}
			}

			// Token reuse outside grace period detected! Revoke all sessions for this user immediately!
			_ = s.authRepo.RevokeAllUserSessions(ctx, oldSession.UserID)
			return nil, errors.New("security alert: reuse of rotated refresh token detected, all sessions revoked")
		}
	}

	// Find session with user in a single query using JOIN to reduce round trips
	var session models.UserSession
	var user models.User
	err := db.DB.WithContext(ctx).
		Model(&models.UserSession{}).
		Where("refresh_token_hash = ? AND is_active = ?",
			models.ComputeRefreshTokenHash(refreshToken), true).
		First(&session).Error
	if err != nil {
		return nil, errors.New("invalid or expired refresh token")
	}

	// Fetch user (we already have user_id from session)
	if err := db.DB.WithContext(ctx).Where("id = ?", session.UserID).First(&user).Error; err != nil {
		return nil, errors.New("user not found")
	}

	if session.ExpiresAt.Before(time.Now()) {
		s.authRepo.RevokeSession(ctx, session.ID)
		return nil, errors.New("refresh token expired")
	}

	// Generate new tokens
	tokenPair, err := s.tokenService.GenerateTokenPair(&user)
	if err != nil {
		return nil, err
	}

	// Rotate refresh token: Invalidate old, create new - use transaction for atomicity
	err = db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Revoke old session
		if err := tx.Model(&models.UserSession{}).
			Where("id = ?", session.ID).
			Updates(map[string]interface{}{
				"is_active":  false,
				"status":     "revoked",
				"revoked_at": tx.NowFunc(),
			}).Error; err != nil {
			return err
		}

		// Create new session
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
		if err := tx.Create(newSession).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Blacklist the old access token (since its JTI is session.ID) - fire and forget
	go func(jti string) {
		_, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		s.tokenService.BlacklistJTI(jti, 15*time.Minute)
	}(session.ID)

	return &dto.RefreshTokenResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	}, nil
}

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
	var session models.UserSession
	if err := db.DB.WithContext(ctx).Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
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

// detectDeviceType detects device type from user agent string
func detectDeviceType(ua string) string {
	uaLower := strings.ToLower(ua)
	if strings.Contains(uaLower, "mobile") || strings.Contains(uaLower, "android") || strings.Contains(uaLower, "iphone") {
		return "mobile"
	}
	if strings.Contains(uaLower, "tablet") || strings.Contains(uaLower, "ipad") {
		return "tablet"
	}
	return "web"
}

// ─────────────────────────────────────────────
//  Forgot Password / Reset Password
// ─────────────────────────────────────────────

func (s *authService) ForgotPassword(ctx context.Context, email string) error {
	var user models.User
	if err := db.DB.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		// Return nil even if user not found — prevents user enumeration
		return nil
	}

	// Generate a 6-digit verification code
	codeBytes := make([]byte, 3)
	if _, err := rand.Read(codeBytes); err != nil {
		return errors.New("failed to generate verification code")
	}
	code := fmt.Sprintf("%06d", int(codeBytes[0])<<16|int(codeBytes[1])<<8|int(codeBytes[2])%1000000)
	if len(code) > 6 {
		code = code[:6]
	}

	// Store the verification code in Redis with 15-minute expiry
	if db.Redis != nil {
		key := fmt.Sprintf("forgot_password_code:%s", email)
		if err := db.Redis.Set(ctx, key, code, 15*time.Minute).Err(); err != nil {
			return errors.New("failed to store verification code")
		}
	}

	// Send verification code via email
	userName := user.Email
	if user.Name != nil {
		userName = *user.Name
	}
	body := GetForgotPasswordCodeEmailTemplate(userName, code)
	_ = GetMailQueueWorker().Enqueue(user.Email, "رمز استعادة كلمة المرور - Tolo Platform", body)

	return nil
}

func (s *authService) VerifyForgotPasswordCode(ctx context.Context, email, code string) (string, error) {
	var user models.User
	if err := db.DB.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return "", errors.New("invalid email")
	}

	// Verify the code from Redis
	if db.Redis == nil {
		return "", errors.New("verification service unavailable")
	}

	key := fmt.Sprintf("forgot_password_code:%s", email)
	storedCode, err := db.Redis.Get(ctx, key).Result()
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

	// Store the reset token with 15-minute expiry
	tokenHash := sha256.Sum256([]byte(tokenStr))
	tokenHashStr := hex.EncodeToString(tokenHash[:])

	resetToken := &models.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: tokenHashStr,
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	}
	if err := db.DB.WithContext(ctx).Create(resetToken).Error; err != nil {
		return "", errors.New("failed to create reset token")
	}

	// Delete the verification code after successful verification
	db.Redis.Del(ctx, key)

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

	var resetToken models.PasswordResetToken
	if err := db.DB.WithContext(ctx).
		Where("token_hash = ? AND is_used = ? AND expires_at > ?", tokenHashStr, false, time.Now().UTC()).
		First(&resetToken).Error; err != nil {
		return errors.New("invalid or expired reset token")
	}

	// Mark token as used
	db.DB.WithContext(ctx).Model(&resetToken).Update("is_used", true)

	// Hash new password (max 14 for security)
	bcryptCost := s.cfg.BCryptCost
	if bcryptCost < bcrypt.MinCost || bcryptCost > 14 {
		bcryptCost = 12
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return errors.New("failed to hash password")
	}

	// Update user credential
	now := time.Now().UTC()
	if err := db.DB.WithContext(ctx).Model(&models.UserCredential{}).
		Where("user_id = ?", resetToken.UserID).
		Updates(map[string]interface{}{
			"password_hash":    string(hashedPassword),
			"last_changed_at": now,
		}).Error; err != nil {
		return errors.New("failed to update password")
	}

	// Revoke all sessions for this user (force re-login)
	_ = s.RevokeAllSessions(ctx, resetToken.UserID)

	return nil
}

// ─────────────────────────────────────────────
//  Email Verification
// ─────────────────────────────────────────────

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

	// Update user email verified status
	if err := db.DB.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", userID).
		Update("email_verified", true).Error; err != nil {
		return errors.New("failed to verify email")
	}

	// Send Welcome Email
	var user models.User
	if err := db.DB.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err == nil {
		userName := user.Email
		if user.Name != nil {
			userName = *user.Name
		}
		welcomeBody := GetWelcomeEmailTemplate(userName, string(user.Role))
		_ = GetMailQueueWorker().Enqueue(user.Email, "مرحباً بك في منصة Tolo التعليمية", welcomeBody)
	}

	return nil
}

func (s *authService) ResendVerificationEmail(ctx context.Context, userID string) error {
	var user models.User
	if err := db.DB.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		return errors.New("user not found")
	}

	if user.EmailVerified {
		return errors.New("email is already verified")
	}

	// Generate a 6-digit verification code
	codeBytes := make([]byte, 3)
	if _, err := rand.Read(codeBytes); err != nil {
		return errors.New("failed to generate verification code")
	}
	code := fmt.Sprintf("%06d", int(codeBytes[0])<<16|int(codeBytes[1])<<8|int(codeBytes[2])%1000000)
	// Ensure 6 digits
	if len(code) > 6 {
		code = code[:6]
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
	body := GetVerificationEmailTemplate(userName, code)
	_ = GetMailQueueWorker().Enqueue(user.Email, "تأكيد البريد الإلكتروني - Tolo Platform", body)

	return nil
}

func (s *authService) GetCurrentUser(ctx context.Context, userID string) (*dto.UserDTO, error) {
	var user models.User
	if err := db.DB.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", userID).First(&user).Error; err != nil {
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

	return &dto.UserDTO{
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

func (s *authService) UpdateProfile(ctx context.Context, userID string, req *dto.UpdateProfileRequest) (*dto.UserDTO, error) {
	var user models.User
	if err := db.DB.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", userID).First(&user).Error; err != nil {
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

	if err := db.DB.WithContext(ctx).Save(&user).Error; err != nil {
		return nil, errors.New("failed to update profile")
	}

	var profile models.Profile
	err := db.DB.WithContext(ctx).Where("user_id = ?", userID).First(&profile).Error
	if err == nil {
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
		db.DB.WithContext(ctx).Save(&profile)
	}

	return s.GetCurrentUser(ctx, userID)
}

func (s *authService) DeleteAccount(ctx context.Context, userID, password, reason string) error {
	var user models.User
	if err := db.DB.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", userID).First(&user).Error; err != nil {
		return errors.New("user not found")
	}

	// Get user credential for password verification
	var credential models.UserCredential
	if err := db.DB.WithContext(ctx).Where("user_id = ?", userID).First(&credential).Error; err != nil {
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
	if err := db.DB.WithContext(ctx).Save(&user).Error; err != nil {
		return errors.New("failed to delete account")
	}

	return nil
}

func (s *authService) GetOAuthRedirectURL(ctx context.Context, provider string) (string, error) {
	if s.oauthService == nil {
		return "", errors.New("OAuth service is not configured")
	}

	state, err := s.oauthService.GenerateOAuthState(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to generate OAuth state: %w", err)
	}

	switch strings.ToLower(provider) {
	case "google":
		redirectURL := s.oauthService.GetGoogleAuthURL(state)
		if redirectURL == "" {
			return "", errors.New("Google OAuth is not configured")
		}
		return redirectURL, nil
	case "apple":
		redirectURL := s.oauthService.GetAppleAuthURL(state)
		if redirectURL == "" {
			return "", errors.New("Apple OAuth is not configured")
		}
		return redirectURL, nil
	default:
		return "", fmt.Errorf("unsupported OAuth provider: %s", provider)
	}
}

func (s *authService) HandleOAuthCallback(ctx context.Context, provider, code, state, userAgent, ip string) (*dto.LoginResponse, error) {
	// ───────────────────────────────────────────────────────────────────────────────
	// Step 1: Validate OAuth state (CSRF protection)
	// ───────────────────────────────────────────────────────────────────────────────
	if s.oauthService == nil {
		return nil, errors.New("OAuth service not configured")
	}

	valid, err := s.oauthService.ValidateOAuthState(ctx, state)
	if err != nil || !valid {
		s.auditService.LogEvent("", AuditEventLoginFailed, "oauth", "", map[string]interface{}{
			"provider": provider,
			"reason":   "invalid_state",
		}, ip, "")
		return nil, errors.New("invalid or expired OAuth state - possible CSRF attack")
	}

	// ───────────────────────────────────────────────────────────────────────────────
	// Step 2: Exchange authorization code for real user info
	// ───────────────────────────────────────────────────────────────────────────────
	var oauthUserInfo *OAuthUserInfo
	var oauthErr error

	switch provider {
	case "google":
		oauthUserInfo, oauthErr = s.oauthService.ExchangeGoogleCode(ctx, code)
	case "apple":
		oauthUserInfo, oauthErr = s.oauthService.ExchangeAppleCode(ctx, code)
	default:
		s.auditService.LogEvent("", AuditEventLoginFailed, "oauth", "", map[string]interface{}{
			"provider": provider,
			"reason":   "unsupported_provider",
		}, ip, "")
		return nil, fmt.Errorf("unsupported OAuth provider: %s", provider)
	}

	if oauthErr != nil {
		s.auditService.LogEvent("", AuditEventLoginFailed, "oauth", "", map[string]interface{}{
			"provider": provider,
			"reason":   "exchange_failed",
			"error":    oauthErr.Error(),
		}, ip, "")
		return nil, fmt.Errorf("failed to get user info from %s: %w", provider, oauthErr)
	}

	if oauthUserInfo == nil {
		return nil, fmt.Errorf("no user info returned from %s", provider)
	}

	// ───────────────────────────────────────────────────────────────────────────────
	// Step 3: Create user or link to existing account
	// ───────────────────────────────────────────────────────────────────────────────
	user, _, err := CreateUserFromOAuth(ctx, oauthUserInfo)
	if err != nil {
		s.auditService.LogEvent("", AuditEventLoginFailed, "oauth", "", map[string]interface{}{
			"provider": provider,
			"email":    oauthUserInfo.Email,
			"reason":   "user_creation_failed",
			"error":    err.Error(),
		}, ip, "")
		return nil, fmt.Errorf("failed to create or link OAuth user: %w", err)
	}

	if user == nil {
		return nil, errors.New("user creation returned nil")
	}

	// ───────────────────────────────────────────────────────────────────────────────
	// Step 4: Generate JWT tokens
	// ───────────────────────────────────────────────────────────────────────────────
	tokenPair, err := s.tokenService.GenerateTokenPair(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// ───────────────────────────────────────────────────────────────────────────────
	// Step 5: Create user session
	// ───────────────────────────────────────────────────────────────────────────────
	session := &models.UserSession{
		UserID:            user.ID,
		RefreshToken:      tokenPair.RefreshToken,
		RefreshTokenHash:  models.HashToken(tokenPair.RefreshToken),
		UserAgent:         userAgent,
		IP:                ip,
		DeviceType:        "oauth_" + provider,
		IsActive:          true,
		ExpiresAt:         time.Now().Add(30 * 24 * time.Hour),
		AbsoluteExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}

	if err := s.authRepo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// ───────────────────────────────────────────────────────────────────────────────
	// Step 6: Log successful OAuth login and return response
	// ───────────────────────────────────────────────────────────────────────────────
	s.auditService.LogEvent(user.ID, AuditEventLogin, "oauth", user.ID, map[string]interface{}{
		"provider": provider,
		"email":    user.Email,
	}, ip, "")

	userDTO := dto.UserDTO{
		ID:            user.ID,
		Email:         user.Email,
		Name:          *user.Name,
		Role:          string(user.Role),
		EmailVerified: user.EmailVerified,
		Permissions:   user.GetEffectivePermissions(),
	}

	return &dto.LoginResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		User:         userDTO,
	}, nil
}

func (s *authService) LinkOAuthProvider(ctx context.Context, userID, provider, code, state string) error {
	if s.oauthService == nil {
		return errors.New("OAuth service is not configured")
	}

	// Validate state parameter (CSRF protection)
	valid, err := s.oauthService.ValidateOAuthState(ctx, state)
	if err != nil || !valid {
		return errors.New("invalid or expired OAuth state")
	}

	// Exchange the authorization code for user info from the provider
	var oauthUserInfo *OAuthUserInfo
	switch strings.ToLower(provider) {
	case "google":
		oauthUserInfo, err = s.oauthService.ExchangeGoogleCode(ctx, code)
	case "apple":
		oauthUserInfo, err = s.oauthService.ExchangeAppleCode(ctx, code)
	default:
		return fmt.Errorf("unsupported OAuth provider: %s", provider)
	}

	if err != nil {
		return fmt.Errorf("failed to exchange code with %s: %w", provider, err)
	}

	if oauthUserInfo == nil {
		return fmt.Errorf("no user info returned from %s", provider)
	}

	// Check if this OAuth account is already linked to another user
	var existingLink models.OAuthAccount
	err = db.DB.WithContext(ctx).
		Where("provider = ? AND provider_user_id = ?", oauthUserInfo.Provider, oauthUserInfo.ProviderUserID).
		First(&existingLink).Error
	if err == nil {
		return errors.New("this social account is already linked to another user")
	} else if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("database error: %w", err)
	}

	// Link the OAuth account to the user
	link := models.OAuthAccount{
		UserID:         userID,
		Provider:       oauthUserInfo.Provider,
		ProviderUserID: oauthUserInfo.ProviderUserID,
		Email:          &oauthUserInfo.Email,
	}
	if err := db.DB.WithContext(ctx).Create(&link).Error; err != nil {
		return fmt.Errorf("failed to link account: %w", err)
	}
	return nil
}

func (s *authService) UnlinkOAuthProvider(ctx context.Context, userID, provider string) error {
	if err := db.DB.WithContext(ctx).Where("user_id = ? AND provider = ?", userID, provider).Delete(&models.OAuthAccount{}).Error; err != nil {
		return errors.New("failed to unlink provider")
	}
	return nil
}

func (s *authService) GetLinkedAccounts(ctx context.Context, userID string) ([]dto.LinkedAccountDTO, error) {
	var accounts []models.OAuthAccount
	if err := db.DB.WithContext(ctx).Where("user_id = ?", userID).Find(&accounts).Error; err != nil {
		return nil, err
	}

	result := make([]dto.LinkedAccountDTO, 0, len(accounts))
	for _, acc := range accounts {
		email := ""
		if acc.Email != nil {
			email = *acc.Email
		}
		name := ""
		if acc.Name != nil {
			name = *acc.Name
		}
		avatar := ""
		if acc.AvatarURL != nil {
			avatar = *acc.AvatarURL
		}
		result = append(result, dto.LinkedAccountDTO{
			Provider: acc.Provider,
			Email:    email,
			Name:     name,
			Avatar:   avatar,
			LinkedAt: acc.CreatedAt.Format(time.RFC3339),
		})
	}
	return result, nil
}

func (s *authService) ValidateAccessToken(ctx context.Context, token string) (*dto.ValidateTokenResponse, error) {
	claims, err := s.tokenService.ValidateAccessToken(token)
	if err != nil {
		return nil, err
	}

	var user models.User
	if err := db.DB.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", claims.UserID).First(&user).Error; err != nil {
		return nil, errors.New("user not found")
	}

	expiresAtStr := ""
	if claims.ExpiresAt != nil {
		expiresAtStr = claims.ExpiresAt.Time.Format(time.RFC3339)
	}

	return &dto.ValidateTokenResponse{
		Valid:       true,
		UserID:      claims.UserID,
		Role:        claims.Role,
		Permissions: user.GetEffectivePermissions(),
		ExpiresAt:   expiresAtStr,
	}, nil
}

func (s *authService) InitiateAccountRecovery(ctx context.Context, email, method string) (string, error) {
	var user models.User
	if err := db.DB.WithContext(ctx).Where("email = ? AND deleted_at IS NULL", email).First(&user).Error; err != nil {
		return "", errors.New("user not found")
	}

	ticket := uuid.New().String()
	codeBytes := make([]byte, 3)
	if _, err := rand.Read(codeBytes); err != nil {
		return "", err
	}
	code := fmt.Sprintf("%06d", int(codeBytes[0])<<16|int(codeBytes[1])<<8|int(codeBytes[2])%1000000)
	if len(code) > 6 {
		code = code[:6]
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
	body := GetVerificationEmailTemplate(userName, code)
	_ = GetMailQueueWorker().Enqueue(user.Email, "رمز استعادة الحساب - Tolo Platform", body)

	return ticket, nil
}

func (s *authService) FinalizeAccountRecovery(ctx context.Context, ticket, code, newPassword string) error {
	var vCode models.VerificationCode
	err := db.DB.WithContext(ctx).
		Where("code = ? AND type = ? AND is_used = ? AND expires_at > ?", code, "account_recovery:"+ticket, false, time.Now()).
		First(&vCode).Error
	if err != nil {
		return errors.New("invalid or expired verification code")
	}

	vCode.IsUsed = true
	db.DB.WithContext(ctx).Save(&vCode)

	var user models.User
	if err := db.DB.WithContext(ctx).Where("id = ?", vCode.UserID).First(&user).Error; err != nil {
		return errors.New("user not found")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return err
	}

	// Update user credential
	if err := db.DB.WithContext(ctx).Model(&models.UserCredential{}).
		Where("user_id = ?", user.ID).
		Update("password_hash", string(hashedPassword)).Error; err != nil {
		return errors.New("failed to save new password")
	}

	return nil
}
