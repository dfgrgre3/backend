package repositories

import (
	"context"
	models "thanawy-backend/internal/domain/common"

	db "thanawy-backend/internal/infrastructure/database"
)

type AuthRepository interface {
	CreateSession(ctx context.Context, session *models.UserSession) error
	GetSessionByToken(ctx context.Context, token string) (*models.UserSession, error)
	RevokeSession(ctx context.Context, sessionID string) error
	RevokeAllUserSessions(ctx context.Context, userID string) error
	LogLoginHistory(ctx context.Context, history *models.LoginHistory) error
	CreateVerificationCode(ctx context.Context, code *models.VerificationCode) error
	GetVerificationCode(ctx context.Context, userID, codeType, code string) (*models.VerificationCode, error)
	MarkCodeAsUsed(ctx context.Context, codeID string) error
	CreateOAuthAccount(ctx context.Context, account *models.OAuthAccount) error
	GetOAuthAccount(ctx context.Context, provider, providerUserID string) (*models.OAuthAccount, error)
	GetUserSessions(ctx context.Context, userID string) ([]*models.UserSession, error)
}

type authRepository struct{}

func NewAuthRepository() AuthRepository {
	return &authRepository{}
}

func (r *authRepository) CreateSession(ctx context.Context, session *models.UserSession) error {
	return db.DB.WithContext(ctx).Create(session).Error
}

func (r *authRepository) GetSessionByToken(ctx context.Context, token string) (*models.UserSession, error) {
	var session models.UserSession
	// Look up by the SHA-256 hash, never the raw refresh token, so a database
	// compromise does not expose live refresh tokens.
	hash := models.ComputeRefreshTokenHash(token)
	err := db.DB.WithContext(ctx).Where("refresh_token_hash = ? AND is_active = ?", hash, true).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *authRepository) RevokeSession(ctx context.Context, sessionID string) error {
	return db.DB.WithContext(ctx).Model(&models.UserSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]interface{}{
			"is_active":  false,
			"status":     "revoked",
			"revoked_at": db.DB.NowFunc(),
		}).Error
}

func (r *authRepository) RevokeAllUserSessions(ctx context.Context, userID string) error {
	return db.DB.WithContext(ctx).Model(&models.UserSession{}).
		Where("user_id = ? AND is_active = ?", userID, true).
		Updates(map[string]interface{}{
			"is_active":  false,
			"status":     "revoked",
			"revoked_at": db.DB.NowFunc(),
		}).Error
}

func (r *authRepository) LogLoginHistory(ctx context.Context, history *models.LoginHistory) error {
	// login_history is an internal system table written by the backend itself,
	// not by end users, so it does not require multi-tenant row isolation.
	// Use RawWriteDB to ensure login history is always persisted even when
	// RLS is enabled on the table without a matching policy.
	return db.RawWriteDB(ctx).Create(history).Error
}

func (r *authRepository) CreateVerificationCode(ctx context.Context, code *models.VerificationCode) error {
	return db.DB.WithContext(ctx).Create(code).Error
}

func (r *authRepository) GetVerificationCode(ctx context.Context, userID, codeType, code string) (*models.VerificationCode, error) {
	var verificationCode models.VerificationCode
	err := db.DB.WithContext(ctx).
		Where("user_id = ? AND type = ? AND code = ? AND is_used = ?", userID, codeType, code, false).
		First(&verificationCode).Error
	return &verificationCode, err
}

func (r *authRepository) MarkCodeAsUsed(ctx context.Context, codeID string) error {
	return db.DB.WithContext(ctx).Model(&models.VerificationCode{}).
		Where("id = ?", codeID).
		Update("is_used", true).Error
}

func (r *authRepository) CreateOAuthAccount(ctx context.Context, account *models.OAuthAccount) error {
	return db.DB.WithContext(ctx).Create(account).Error
}

func (r *authRepository) GetOAuthAccount(ctx context.Context, provider, providerUserID string) (*models.OAuthAccount, error) {
	var account models.OAuthAccount
	err := db.DB.WithContext(ctx).
		Where("provider = ? AND provider_user_id = ?", provider, providerUserID).
		First(&account).Error
	return &account, err
}

func (r *authRepository) GetUserSessions(ctx context.Context, userID string) ([]*models.UserSession, error) {
	var sessions []models.UserSession
	err := db.DB.WithContext(ctx).Where("user_id = ? AND is_active = ?", userID, true).Order("last_accessed DESC").Find(&sessions).Error
	if err != nil {
		return nil, err
	}

	result := make([]*models.UserSession, len(sessions))
	for i := range sessions {
		result[i] = &sessions[i]
	}
	return result, nil
}
