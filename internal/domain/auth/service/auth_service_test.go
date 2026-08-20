package authservice

import (
	"context"
	"testing"
	"time"

	authdto "thanawy-backend/internal/application/dto"
	models "thanawy-backend/internal/domain/common"
)

type recordingAuthRepo struct {
	loggedHistory *models.LoginHistory
}

func (r *recordingAuthRepo) CreateSession(ctx context.Context, session *models.UserSession) error {
	return nil
}
func (r *recordingAuthRepo) GetSessionByToken(ctx context.Context, token string) (*models.UserSession, error) {
	return nil, nil
}
func (r *recordingAuthRepo) RevokeSession(ctx context.Context, sessionID string) error { return nil }
func (r *recordingAuthRepo) RevokeAllUserSessions(ctx context.Context, userID string) error {
	return nil
}
func (r *recordingAuthRepo) LogLoginHistory(ctx context.Context, history *models.LoginHistory) error {
	r.loggedHistory = history
	return nil
}
func (r *recordingAuthRepo) CreateVerificationCode(ctx context.Context, code *models.VerificationCode) error {
	return nil
}
func (r *recordingAuthRepo) GetVerificationCode(ctx context.Context, userID, codeType, code string) (*models.VerificationCode, error) {
	return nil, nil
}
func (r *recordingAuthRepo) MarkCodeAsUsed(ctx context.Context, codeID string) error { return nil }
func (r *recordingAuthRepo) CreateOAuthAccount(ctx context.Context, account *models.OAuthAccount) error {
	return nil
}
func (r *recordingAuthRepo) GetOAuthAccount(ctx context.Context, provider, providerUserID string) (*models.OAuthAccount, error) {
	return nil, nil
}
func (r *recordingAuthRepo) GetUserSessions(ctx context.Context, userID string) ([]*models.UserSession, error) {
	return nil, nil
}
func (r *recordingAuthRepo) FindVerificationCodeByCodeAndType(ctx context.Context, code, codeType string) (*models.VerificationCode, error) {
	return nil, nil
}
func (r *recordingAuthRepo) GetSessionByIDAndUser(ctx context.Context, sessionID, userID string) (*models.UserSession, error) {
	return nil, nil
}
func (r *recordingAuthRepo) GetSessionByHashOrdered(ctx context.Context, hash string) (*models.UserSession, error) {
	return nil, nil
}
func (r *recordingAuthRepo) GetActiveReplacementSession(ctx context.Context, userID string, since time.Time) (*models.UserSession, error) {
	return nil, nil
}
func (r *recordingAuthRepo) RotateSession(ctx context.Context, oldSessionID string, newSession *models.UserSession) error {
	return nil
}
func (r *recordingAuthRepo) FindUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return nil, nil
}
func (r *recordingAuthRepo) FindUserByID(ctx context.Context, userID string) (*models.User, error) {
	return nil, nil
}
func (r *recordingAuthRepo) GetCredentialByUserID(ctx context.Context, userID string) (*models.UserCredential, error) {
	return nil, nil
}
func (r *recordingAuthRepo) CreateUserAndCredential(ctx context.Context, user *models.User, credential *models.UserCredential) error {
	return nil
}
func (r *recordingAuthRepo) UpdateCredential(ctx context.Context, credential *models.UserCredential) error {
	return nil
}
func (r *recordingAuthRepo) UpdatePasswordHash(ctx context.Context, userID, hash string) error {
	return nil
}
func (r *recordingAuthRepo) SaveUser(ctx context.Context, user *models.User) error { return nil }
func (r *recordingAuthRepo) GetProfileByUserID(ctx context.Context, userID string) (*models.Profile, error) {
	return nil, nil
}
func (r *recordingAuthRepo) SaveProfile(ctx context.Context, profile *models.Profile) error {
	return nil
}
func (r *recordingAuthRepo) CreatePasswordResetToken(ctx context.Context, token *models.PasswordResetToken) error {
	return nil
}
func (r *recordingAuthRepo) FindPasswordResetTokenByHash(ctx context.Context, tokenHash string) (*models.PasswordResetToken, error) {
	return nil, nil
}
func (r *recordingAuthRepo) MarkPasswordResetTokenUsed(ctx context.Context, tokenID string) error {
	return nil
}
func (r *recordingAuthRepo) FindOAuthAccountByProvider(ctx context.Context, provider, providerUserID string) (*models.OAuthAccount, error) {
	return nil, nil
}
func (r *recordingAuthRepo) DeleteOAuthAccount(ctx context.Context, userID, provider string) error {
	return nil
}
func (r *recordingAuthRepo) GetOAuthAccountsByUser(ctx context.Context, userID string) ([]models.OAuthAccount, error) {
	return nil, nil
}
func (r *recordingAuthRepo) CreateUserWithProfileAndOAuth(ctx context.Context, user *models.User, profile *models.Profile, oauthAcc *models.OAuthAccount) error {
	return nil
}

func TestLogFailedLoginSkipsAnonymousUsers(t *testing.T) {
	repo := &recordingAuthRepo{}
	svc := &authService{authRepo: repo}

	svc.logFailedLogin(context.Background(), "", "127.0.0.1", "test-agent", "Invalid credentials")

	if repo.loggedHistory != nil {
		t.Fatalf("expected no login history to be recorded for anonymous users, got %#v", repo.loggedHistory)
	}
}

func TestPasswordPolicyValidation(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"Too Short", "Ab1!", true},
		{"No Uppercase", "ab12345!", true},
		{"No Lowercase", "AB12345!", true},
		{"No Digit", "Abcdefgh!", true},
		{"No Special", "Abcdefgh1", true},
		{"Common Password", "password", true},
		{"Valid Password", "SecureP@ss123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePasswordPolicy(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePasswordPolicy() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegisterPasswordPolicyRejection(t *testing.T) {
	svc := &authService{}
	req := &authdto.RegisterRequest{
		Email:     "test@example.com",
		Password:  "weak",
		FirstName: "John",
		LastName:  "Doe",
	}

	_, err := svc.Register(context.Background(), req)
	if err == nil {
		t.Error("expected register to fail due to weak password policy")
	}
}
