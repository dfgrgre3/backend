package authservice

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	authdto "thanawy-backend/internal/application/dto"
	"thanawy-backend/internal/application/services"
	analyticsservice "thanawy-backend/internal/domain/analytics/service"
	models "thanawy-backend/internal/domain/common"
	notificationservice "thanawy-backend/internal/domain/notification/service"
	"thanawy-backend/internal/infrastructure/cache"
	authrepo "thanawy-backend/internal/infrastructure/persistence/repositories"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/singleflight"

	"thanawy-backend/internal/infrastructure/config"

	"github.com/google/uuid"
)

// generateSixDigitCode returns a uniformly random 6-digit numeric code
// ("000000"-"999999") using a CSPRNG. It replaces a previous ad-hoc
// bit-packing scheme (byte0<<16 | byte1<<8 | byte2%1000000) whose modulo
// bound only the last byte (Go's % has higher precedence than |), so the
// combined 24-bit value was never actually reduced mod 1,000,000 — codes
// were instead truncated to their first 6 characters, producing a biased,
// non-uniform code space instead of the intended 1-in-1,000,000 code.
func generateSixDigitCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// Lockout Policy Configuration
//
// SECURITY: this entire policy is backed by Redis (see checkAccountLockout /
// recordFailedAttempt below) and fails OPEN — with cache.Redis == nil, login
// attempts are never counted and accounts can never be locked out, with no
// warning at request time. Redis is therefore a hard security dependency in
// production, not just a performance cache; deploying without it silently
// disables brute-force protection on the login endpoint.
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
	if cache.Redis == nil {
		// Fails OPEN: without Redis there is no lockout store to check, so
		// every login is treated as "not locked out" regardless of prior
		// failed attempts. See the SECURITY note on the Lockout Policy
		// Configuration block above — this is a deployment requirement, not
		// a caller-visible error, so it stays silent here by design.
		return nil
	}

	lockoutKey := getLockoutKey(userID)
	ttl, err := cache.Redis.TTL(ctx, lockoutKey).Result()
	if err != nil || ttl <= 0 {
		return nil // Not locked out
	}

	minutesRemaining := int(ttl.Minutes()) + 1
	return fmt.Errorf("account locked due to too many failed attempts. Try again in %d minute(s)", minutesRemaining)
}

// recordFailedAttempt increments the failed attempt counter.
// Locks the account if threshold is reached.
func recordFailedAttempt(ctx context.Context, userID string) {
	if cache.Redis == nil || userID == "" {
		return
	}

	failedKey := getFailedAttemptsKey(userID)
	lockoutKey := getLockoutKey(userID)

	// Increment failed attempts counter
	count, err := cache.Redis.Incr(ctx, failedKey).Result()
	if err != nil {
		return
	}

	// Set expiry on first attempt to track over rolling window
	if count == 1 {
		cache.Redis.Expire(ctx, failedKey, lockoutDuration)
	}

	// Lock account if threshold is reached
	if count >= maxFailedAttempts {
		cache.Redis.Set(ctx, lockoutKey, "locked", lockoutDuration)
		cache.Redis.Del(ctx, failedKey) // Reset failed counter after locking
	}
}

// resetFailedAttempts clears the failed attempt counter on successful login
func resetFailedAttempts(ctx context.Context, userID string) {
	if cache.Redis == nil || userID == "" {
		return
	}
	cache.Redis.Del(ctx, getFailedAttemptsKey(userID))
	cache.Redis.Del(ctx, getLockoutKey(userID))
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
	Register(ctx context.Context, req *authdto.RegisterRequest) (*authdto.RegisterResponse, error)
	Login(ctx context.Context, req *authdto.LoginRequest, userAgent, ip string) (*authdto.LoginResponse, error)
	RefreshToken(ctx context.Context, refreshToken, userAgent, ip string) (*authdto.RefreshTokenResponse, error)
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
	GetCurrentUser(ctx context.Context, userID string) (*authdto.UserDTO, error)
	UpdateProfile(ctx context.Context, userID string, req *authdto.UpdateProfileRequest) (*authdto.UserDTO, error)
	DeleteAccount(ctx context.Context, userID, password, reason string) error
	GetOAuthRedirectURL(ctx context.Context, provider string) (string, error)
	HandleOAuthCallback(ctx context.Context, provider, code, state, userAgent, ip string) (*authdto.LoginResponse, error)
	LinkOAuthProvider(ctx context.Context, userID, provider, code, state string) error
	UnlinkOAuthProvider(ctx context.Context, userID, provider string) error
	GetLinkedAccounts(ctx context.Context, userID string) ([]authdto.LinkedAccountDTO, error)
	ValidateAccessToken(ctx context.Context, token string) (*authdto.ValidateTokenResponse, error)
	InitiateAccountRecovery(ctx context.Context, email, method string) (string, error)
	FinalizeAccountRecovery(ctx context.Context, ticket, code, newPassword string) error
}

type authService struct {
	authRepo     authrepo.AuthRepository
	tokenService AuthTokenService
	oauthService OAuthService
	auditService *analyticsservice.AuditService
	cfg          *config.Config
	mailQueue    *services.MailQueueWorker
	refreshSF    singleflight.Group
}

func NewAuthService(authRepo authrepo.AuthRepository, tokenService AuthTokenService, oauthService OAuthService, cfg *config.Config, mailQueue *services.MailQueueWorker) AuthService {
	return &authService{
		authRepo:     authRepo,
		tokenService: tokenService,
		oauthService: oauthService,
		auditService: analyticsservice.GetAuditService(),
		cfg:          cfg,
		mailQueue:    mailQueue,
	}
}

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

func (s *authService) Login(ctx context.Context, req *authdto.LoginRequest, userAgent, ip string) (*authdto.LoginResponse, error) {
	// Find user via repository
	user, err := s.authRepo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		s.logFailedLogin(ctx, "", ip, userAgent, "Invalid credentials")
		return nil, errors.New("invalid email or password")
	}

	// Check account lockout before password verification
	if err := checkAccountLockout(ctx, user.ID); err != nil {
		s.logFailedLogin(ctx, user.ID, ip, userAgent, "Account locked")
		return nil, err
	}

	// Get user credential for password verification
	credential, err := s.authRepo.GetCredentialByUserID(ctx, user.ID)
	if err != nil {
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
		if cache.Redis == nil {
			return nil, errors.New("MFA is enabled but session cache is unavailable")
		}
		ticket := uuid.New().String()
		ticketKey := fmt.Sprintf("mfa_ticket:%s", ticket)
		if err := cache.Redis.Set(ctx, ticketKey, user.ID, 3*time.Minute).Err(); err != nil {
			return nil, fmt.Errorf("failed to generate MFA challenge: %w", err)
		}
		return nil, fmt.Errorf("MFA_REQUIRED:%s", ticket)
	}

	// Generate tokens
	tokenPair, err := s.tokenService.GenerateTokenPair(user)
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

	return &authdto.LoginResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		User: authdto.UserDTO{
			ID:            user.ID,
			Email:         user.Email,
			Name:          name,
			Role:          string(user.Role),
			EmailVerified: user.EmailVerified,
			PhoneVerified: user.PhoneVerified,
			Permissions:   user.GetEffectivePermissions(),
		},
	}, nil
}

func (s *authService) RefreshToken(ctx context.Context, refreshToken, userAgent, ip string) (*authdto.RefreshTokenResponse, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, errors.New("refresh token is required")
	}

	oldHash := models.ComputeRefreshTokenHash(refreshToken)

	res, err, _ := s.refreshSF.Do(oldHash, func() (interface{}, error) {
		return s.doRefreshToken(ctx, refreshToken, oldHash, userAgent, ip)
	})
	if err != nil {
		return nil, err
	}
	return res.(*authdto.RefreshTokenResponse), nil
}

func (s *authService) doRefreshToken(ctx context.Context, refreshToken, oldHash, userAgent, ip string) (*authdto.RefreshTokenResponse, error) {
	session, err := s.authRepo.GetSessionByHashOrdered(ctx, oldHash)
	if err != nil {
		return nil, errors.New("invalid or expired refresh token")
	}

	if !session.IsActive {
		// Check if it was revoked/rotated within the 15-second grace period (e.g. concurrent/rapid client requests)
		if session.Status == "revoked" && session.RevokedAt != nil && time.Since(*session.RevokedAt) <= 15*time.Second {
			replacementSession, lookupErr := s.authRepo.GetActiveReplacementSession(ctx, session.UserID, session.RevokedAt.Add(-2*time.Second))
			if lookupErr == nil && replacementSession != nil && replacementSession.IsActive {
				user, userErr := s.authRepo.FindUserByID(ctx, session.UserID)
				if userErr == nil {
					accessToken, tokenErr := s.tokenService.GenerateAccessTokenForSession(user, replacementSession.ID)
					if tokenErr == nil {
						return &authdto.RefreshTokenResponse{
							AccessToken:  accessToken,
							RefreshToken: replacementSession.RefreshToken,
						}, nil
					}
				}
			}
		}

		// Token reuse outside grace period detected! Revoke all sessions for this user immediately
		_ = s.authRepo.RevokeAllUserSessions(ctx, session.UserID)
		return nil, errors.New("security alert: reuse of rotated refresh token detected, all sessions revoked")
	}

	user, err := s.authRepo.FindUserByID(ctx, session.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if session.ExpiresAt.Before(time.Now()) {
		s.authRepo.RevokeSession(ctx, session.ID)
		return nil, errors.New("refresh token expired")
	}

	// Generate new tokens
	tokenPair, err := s.tokenService.GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}

	// Build the new session
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

	// Rotate refresh token atomically via repository
	if err := s.authRepo.RotateSession(ctx, session.ID, newSession); err != nil {
		return nil, err
	}

	// Blacklist the old access token (since its JTI is session.ID) - fire and forget
	go func(jti string) {
		_, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		s.tokenService.BlacklistJTI(jti, 15*time.Minute)
	}(session.ID)

	return &authdto.RefreshTokenResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	}, nil
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
