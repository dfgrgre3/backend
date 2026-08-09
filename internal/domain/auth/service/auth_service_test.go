package authservice

import (
	"context"
	"testing"

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
