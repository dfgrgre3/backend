package repositories

import (
	"context"
	"errors"
	"time"
	models "thanawy-backend/internal/domain/common"

	db "thanawy-backend/internal/infrastructure/database"

	"gorm.io/gorm"
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
	FindVerificationCodeByCodeAndType(ctx context.Context, code, codeType string) (*models.VerificationCode, error)
	CreateOAuthAccount(ctx context.Context, account *models.OAuthAccount) error
	GetOAuthAccount(ctx context.Context, provider, providerUserID string) (*models.OAuthAccount, error)
	GetUserSessions(ctx context.Context, userID string) ([]*models.UserSession, error)
	GetSessionByIDAndUser(ctx context.Context, sessionID, userID string) (*models.UserSession, error)
	GetSessionByHashOrdered(ctx context.Context, hash string) (*models.UserSession, error)
	GetActiveReplacementSession(ctx context.Context, userID string, since time.Time) (*models.UserSession, error)
	// RotateSession atomically revokes the old session and persists the new one.
	RotateSession(ctx context.Context, oldSessionID string, newSession *models.UserSession) error

	// ── User & Credential (Auth domain) ──────────────────────────────
	// FindUserByEmail returns a user by email or gorm.ErrRecordNotFound.
	FindUserByEmail(ctx context.Context, email string) (*models.User, error)
	// FindUserByID returns an active (non soft-deleted) user by id.
	FindUserByID(ctx context.Context, userID string) (*models.User, error)
	GetCredentialByUserID(ctx context.Context, userID string) (*models.UserCredential, error)
	// CreateUserAndCredential persists a user and its auth credential atomically.
	CreateUserAndCredential(ctx context.Context, user *models.User, credential *models.UserCredential) error
	UpdateCredential(ctx context.Context, credential *models.UserCredential) error
	UpdatePasswordHash(ctx context.Context, userID, hash string) error
	SaveUser(ctx context.Context, user *models.User) error

	// ── Profile ──────────────────────────────────────────────────────
	GetProfileByUserID(ctx context.Context, userID string) (*models.Profile, error)
	SaveProfile(ctx context.Context, profile *models.Profile) error

	// ── Password Reset ───────────────────────────────────────────────
	CreatePasswordResetToken(ctx context.Context, token *models.PasswordResetToken) error
	FindPasswordResetTokenByHash(ctx context.Context, tokenHash string) (*models.PasswordResetToken, error)
	MarkPasswordResetTokenUsed(ctx context.Context, tokenID string) error

	// ── OAuth Accounts ───────────────────────────────────────────────
	FindOAuthAccountByProvider(ctx context.Context, provider, providerUserID string) (*models.OAuthAccount, error)
	DeleteOAuthAccount(ctx context.Context, userID, provider string) error
	GetOAuthAccountsByUser(ctx context.Context, userID string) ([]models.OAuthAccount, error)
	// CreateUserWithProfileAndOAuth persists a new user, profile and OAuth
	// account in a single transaction (used during OAuth first-time login).
	CreateUserWithProfileAndOAuth(ctx context.Context, user *models.User, profile *models.Profile, oauthAcc *models.OAuthAccount) error
}

// authRepository implements auth persistence on top of the shared GORM handle.
// It reuses the cached UserRepository for user lookups so profile reads benefit
// from the existing multi-layer cache while auth-specific tables stay here.
type authRepository struct {
	userRepo *UserRepository
}

func NewAuthRepository() AuthRepository {
	return &authRepository{
		userRepo: NewUserRepository(db.DB),
	}
}

// IsNotFound reports whether the error indicates a missing row.
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
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
// ── User & Credential ──────────────────────────────────────────────────

func (r *authRepository) FindUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return r.userRepo.FindByEmail(email)
}

func (r *authRepository) FindUserByID(ctx context.Context, userID string) (*models.User, error) {
	return r.userRepo.FindByID(userID)
}

func (r *authRepository) GetCredentialByUserID(ctx context.Context, userID string) (*models.UserCredential, error) {
	var credential models.UserCredential
	if err := db.DB.WithContext(ctx).Where("user_id = ?", userID).First(&credential).Error; err != nil {
		return nil, err
	}
	return &credential, nil
}

func (r *authRepository) CreateUserAndCredential(ctx context.Context, user *models.User, credential *models.UserCredential) error {
	return db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		credential.UserID = user.ID
		if err := tx.Create(credential).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *authRepository) UpdateCredential(ctx context.Context, credential *models.UserCredential) error {
	return db.DB.WithContext(ctx).Save(credential).Error
}

func (r *authRepository) UpdatePasswordHash(ctx context.Context, userID, hash string) error {
	return db.DB.WithContext(ctx).Model(&models.UserCredential{}).
		Where("user_id = ?", userID).
		Update("password_hash", hash).Error
}

func (r *authRepository) SaveUser(ctx context.Context, user *models.User) error {
	return r.userRepo.Update(user)
}

// ── Profile ────────────────────────────────────────────────────────────

func (r *authRepository) GetProfileByUserID(ctx context.Context, userID string) (*models.Profile, error) {
	var profile models.Profile
	if err := db.DB.WithContext(ctx).Where("user_id = ?", userID).First(&profile).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *authRepository) SaveProfile(ctx context.Context, profile *models.Profile) error {
	return db.DB.WithContext(ctx).Save(profile).Error
}

// ── Password Reset ─────────────────────────────────────────────────────

func (r *authRepository) CreatePasswordResetToken(ctx context.Context, token *models.PasswordResetToken) error {
	return db.DB.WithContext(ctx).Create(token).Error
}

func (r *authRepository) FindPasswordResetTokenByHash(ctx context.Context, tokenHash string) (*models.PasswordResetToken, error) {
	var token models.PasswordResetToken
	err := db.DB.WithContext(ctx).
		Where("token_hash = ? AND is_used = ? AND expires_at > ?", tokenHash, false, time.Now().UTC()).
		First(&token).Error
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *authRepository) MarkPasswordResetTokenUsed(ctx context.Context, tokenID string) error {
	return db.DB.WithContext(ctx).Model(&models.PasswordResetToken{}).
		Where("id = ?", tokenID).Update("is_used", true).Error
}

// ── OAuth Accounts ─────────────────────────────────────────────────────

func (r *authRepository) FindOAuthAccountByProvider(ctx context.Context, provider, providerUserID string) (*models.OAuthAccount, error) {
	return r.GetOAuthAccount(ctx, provider, providerUserID)
}

func (r *authRepository) DeleteOAuthAccount(ctx context.Context, userID, provider string) error {
	return db.DB.WithContext(ctx).
		Where("user_id = ? AND provider = ?", userID, provider).
		Delete(&models.OAuthAccount{}).Error
}

func (r *authRepository) GetOAuthAccountsByUser(ctx context.Context, userID string) ([]models.OAuthAccount, error) {
	var accounts []models.OAuthAccount
	err := db.DB.WithContext(ctx).Where("user_id = ?", userID).Find(&accounts).Error
	return accounts, err
}

func (r *authRepository) CreateUserWithProfileAndOAuth(ctx context.Context, user *models.User, profile *models.Profile, oauthAcc *models.OAuthAccount) error {
	return db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		if profile != nil {
			profile.UserID = user.ID
			if err := tx.Create(profile).Error; err != nil {
				return err
			}
		}
		if oauthAcc != nil {
			oauthAcc.UserID = user.ID
			if err := tx.Create(oauthAcc).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
func (r *authRepository) GetSessionByIDAndUser(ctx context.Context, sessionID, userID string) (*models.UserSession, error) {
	var session models.UserSession
	if err := db.DB.WithContext(ctx).Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *authRepository) FindVerificationCodeByCodeAndType(ctx context.Context, code, codeType string) (*models.VerificationCode, error) {
	var vCode models.VerificationCode
	err := db.DB.WithContext(ctx).
		Where("code = ? AND type = ? AND is_used = ? AND expires_at > ?", code, codeType, false, time.Now()).
		First(&vCode).Error
	if err != nil {
		return nil, err
	}
	return &vCode, nil
}
func (r *authRepository) GetSessionByHashOrdered(ctx context.Context, hash string) (*models.UserSession, error) {
	var session models.UserSession
	if err := db.DB.WithContext(ctx).
		Where("refresh_token_hash = ?", hash).
		Order("updated_at DESC").
		First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *authRepository) GetActiveReplacementSession(ctx context.Context, userID string, since time.Time) (*models.UserSession, error) {
	var session models.UserSession
	if err := db.DB.WithContext(ctx).
		Where("user_id = ? AND is_active = ? AND created_at >= ?", userID, true, since).
		Order("created_at DESC").
		First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *authRepository) RotateSession(ctx context.Context, oldSessionID string, newSession *models.UserSession) error {
	return db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Revoke old session
		if err := tx.Model(&models.UserSession{}).
			Where("id = ?", oldSessionID).
			Updates(map[string]interface{}{
				"is_active":  false,
				"status":     "revoked",
				"revoked_at": tx.NowFunc(),
			}).Error; err != nil {
			return err
		}
		// Create new session
		if err := tx.Create(newSession).Error; err != nil {
			return err
		}
		return nil
	})
}
