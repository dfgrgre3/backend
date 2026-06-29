package services

import (
	"context"
	"testing"
	"thanawy-backend/internal/api/dto"
)

type mockAuthRepo struct{}

func (m *mockAuthRepo) CreateSession(ctx context.Context, session interface{}) error { return nil }
func (m *mockAuthRepo) GetSessionByToken(ctx context.Context, token string) (interface{}, error) {
	return nil, nil
}
func (m *mockAuthRepo) RevokeSession(ctx context.Context, sessionID string) error      { return nil }
func (m *mockAuthRepo) RevokeAllUserSessions(ctx context.Context, userID string) error { return nil }
func (m *mockAuthRepo) LogLoginHistory(ctx context.Context, history interface{}) error  { return nil }
func (m *mockAuthRepo) CreateVerificationCode(ctx context.Context, code interface{}) error {
	return nil
}
func (m *mockAuthRepo) GetVerificationCode(ctx context.Context, userID, codeType, code string) (interface{}, error) {
	return nil, nil
}
func (m *mockAuthRepo) MarkCodeAsUsed(ctx context.Context, codeID string) error            { return nil }
func (m *mockAuthRepo) CreateOAuthAccount(ctx context.Context, account interface{}) error { return nil }
func (m *mockAuthRepo) GetOAuthAccount(ctx context.Context, provider, providerUserID string) (interface{}, error) {
	return nil, nil
}
func (m *mockAuthRepo) GetUserSessions(ctx context.Context, userID string) ([]interface{}, error) {
	return nil, nil
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
	req := &dto.RegisterRequest{
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
