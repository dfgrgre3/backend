package authservice

import (
	"context"
	"errors"
	"fmt"
	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"

	"gorm.io/gorm"
)

// LinkOAuthAccountToUser links an OAuth account to an existing user
func LinkOAuthAccountToUser(ctx context.Context, userID string, oauthInfo *OAuthUserInfo) error {
	oauthAcc := models.OAuthAccount{
		UserID:         userID,
		ProviderUserID: oauthInfo.ProviderUserID,
		Email:          &oauthInfo.Email,
	}

	result := db.DB.WithContext(ctx).Create(&oauthAcc)
	if result.Error != nil {
		if result.Error.Error() == "pq: duplicate key value violates unique constraint" {
			return errors.New("this OAuth account is already linked to another user")
		}
		return fmt.Errorf("failed to link OAuth account: %w", result.Error)
	}

	return nil
}

// CreateUserFromOAuth creates a new user from OAuth information
func CreateUserFromOAuth(ctx context.Context, oauthInfo *OAuthUserInfo) (*models.User, *models.OAuthAccount, error) {
	// Check if OAuth account already exists
	var existingOAuth models.OAuthAccount
	if err := db.DB.WithContext(ctx).
		Where("provider = ? AND provider_user_id = ?", oauthInfo.Provider, oauthInfo.ProviderUserID).
		First(&existingOAuth).Error; err == nil {
		// Account exists, fetch the associated user
		var user models.User
		if err := db.DB.WithContext(ctx).
			Where("id = ? AND deleted_at IS NULL", existingOAuth.UserID).
			First(&user).Error; err != nil {
			return nil, &existingOAuth, fmt.Errorf("associated user not found: %w", err)
		}
		return &user, &existingOAuth, nil
	} else if err != gorm.ErrRecordNotFound {
		return nil, nil, fmt.Errorf("database error: %w", err)
	}

	// Check if email already exists
	var existingUser models.User
	emailExists := db.DB.WithContext(ctx).
		Where("email = ? AND deleted_at IS NULL", oauthInfo.Email).
		First(&existingUser).Error == nil

	if emailExists {
		// Email exists but OAuth account doesn't - link them
		oauthAcc := models.OAuthAccount{
			UserID:         existingUser.ID,
			Provider:       oauthInfo.Provider,
			ProviderUserID: oauthInfo.ProviderUserID,
			Email:          &oauthInfo.Email,
		}
		if err := db.DB.WithContext(ctx).Create(&oauthAcc).Error; err != nil {
			return nil, nil, fmt.Errorf("failed to link existing user to OAuth: %w", err)
		}
		return &existingUser, &oauthAcc, nil
	}

	// Create new user
	user := models.User{
		Email:         oauthInfo.Email,
		Role:          models.RoleStudent,
		Status:        models.StatusActive,
		Name:          &oauthInfo.Name,
		EmailVerified: true, // OAuth providers verify emails
	}

	if err := db.DB.WithContext(ctx).Create(&user).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to create user from OAuth: %w", err)
	}

	// Create user profile
	profile := models.Profile{
		UserID:    user.ID,
		Name:      oauthInfo.Name,
		AvatarURL: &oauthInfo.Picture,
	}
	db.DB.WithContext(ctx).Create(&profile)

	// Create OAuth account record
	oauthAcc := models.OAuthAccount{
		UserID:         user.ID,
		Provider:       oauthInfo.Provider,
		ProviderUserID: oauthInfo.ProviderUserID,
		Email:          &oauthInfo.Email,
	}

	if err := db.DB.WithContext(ctx).Create(&oauthAcc).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to create OAuth account: %w", err)
	}

	return &user, &oauthAcc, nil
}
