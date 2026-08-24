package authservice

import (
	"context"
	authdto "thanawy-backend/internal/application/dto"
	"thanawy-backend/internal/application/services"
	analyticsservice "thanawy-backend/internal/domain/analytics/service"
	models "thanawy-backend/internal/domain/common"
	authrepo "thanawy-backend/internal/infrastructure/persistence/repositories"

	"golang.org/x/sync/singleflight"

	"thanawy-backend/internal/infrastructure/config"
)

// AuthService and its authService implementation are split across several
// files in this package (all sharing package authservice), grouped by area:
// this file (interface + construction), auth_service_lockout.go (account
// lockout policy), auth_service_verification.go (verification code
// generation), auth_service_password_policy.go (password complexity
// validation), auth_service_useragent.go (user-agent parsing),
// auth_service_register.go (Register), auth_service_login.go (Login /
// RefreshToken), and the pre-existing auth_service_oauth.go,
// auth_service_password.go, auth_service_profile.go,
// auth_service_sessions.go and auth_service_token.go.
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
