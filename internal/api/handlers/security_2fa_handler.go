package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"net/http"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"

	"thanawy-backend/internal/db"
	"thanawy-backend/internal/middleware"
	"thanawy-backend/internal/models"
)

const (
	errInvalidVerificationCode = "Invalid verification code"
	err2FANotEnabled           = "2FA not enabled"
)

// GetTwoFactorStatus returns the 2FA status for the current user
// @Summary Get 2FA status
// @Description Get two-factor authentication status
// @Tags admin,security
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/security/2fa/status [get]
func GetTwoFactorStatus(c *gin.Context) {
	userID, _ := c.Get("userId")

	var settings models.TwoFactorSettings
	if err := db.DB.First(&settings, userIDQuery, userID).Error; err != nil {
		// No settings found, return disabled status
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"isEnabled":  false,
				"method":     nil,
				"isEnforced": false,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"isEnabled":       settings.IsEnabled,
			"method":          settings.Method,
			"lastUsedAt":      settings.LastUsedAt,
			"isEnforced":      settings.IsEnforced,
			"verifiedDevices": settings.VerifiedDevices,
		},
	})
}

// generateQRCodeBase64 generates a base64-encoded QR code PNG image from a string.
func generateQRCodeBase64(data string) (string, error) {
	qrCode, err := qr.Encode(data, qr.M, qr.Auto)
	if err != nil {
		return "", err
	}
	qrCode, err = barcode.Scale(qrCode, 200, 200)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, qrCode); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// InitiateTwoFactorSetup starts the 2FA setup process
// @Summary Initiate 2FA setup
// @Description Start setting up two-factor authentication
// @Tags admin,security
// @Accept json
// @Produce json
// @Param request body map[string]string true "Setup method"
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/security/2fa/setup [post]
func InitiateTwoFactorSetup(c *gin.Context) {
	var req struct {
		Method string `json:"method" binding:"required,oneof=authenticator sms email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("userId")
	userEmail, _ := c.Get("user_email")
	userPhone, _ := c.Get("user_phone")

	userIDStr, ok := userID.(string)
	if !ok || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var response gin.H

	switch req.Method {
	case "authenticator":
		// Generate TOTP secret
		secret := make([]byte, 20)
		rand.Read(secret)
		secretKey := base32.StdEncoding.EncodeToString(secret)

		emailStr := ""
		if userEmail != nil {
			if s, ok := userEmail.(string); ok {
				emailStr = s
			}
		}

		// Generate QR code URL
		qrURL := fmt.Sprintf("otpauth://totp/Thanawy Admin:%s?secret=%s&issuer=Thanawy Admin", emailStr, secretKey)
		qrCodeBase64, err := generateQRCodeBase64(qrURL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate QR code"})
			return
		}

		response = gin.H{
			"secret": secretKey,
			"qrCode": qrCodeBase64,
		}

		// Store pending setup
		var settings models.TwoFactorSettings
		err = db.DB.Where(userIDQuery, userIDStr).First(&settings).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				settings = models.TwoFactorSettings{
					ID:           uuid.NewString(),
					UserID:       userIDStr,
					Method:       "authenticator",
					Secret:       secretKey,
					IsEnabled:    false,
					PendingSetup: true,
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
				}
				db.DB.Create(&settings)
			}
		} else {
			settings.Secret = secretKey
			settings.Method = "authenticator"
			settings.PendingSetup = true
			settings.UpdatedAt = time.Now()
			db.DB.Save(&settings)
		}

	case "sms":
		if userPhone == nil || userPhone == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Phone number not available"})
			return
		}
		response = gin.H{
			"phoneNumber": userPhone,
			"message":     "Verification code sent via SMS",
		}

	case "email":
		response = gin.H{
			"email":   userEmail,
			"message": "Verification code sent via email",
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": response})
}

// VerifyTwoFactor verifies the 2FA code and activates it
// @Summary Verify and activate 2FA
// @Description Verify the 2FA code and activate two-factor authentication
// @Tags admin,security
// @Accept json
// @Produce json
// @Param request body map[string]string true "Verification code"
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/security/2fa/verify [post]
func VerifyTwoFactor(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("userId")

	var settings models.TwoFactorSettings
	if err := db.DB.First(&settings, userIDQuery, userID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No pending 2FA setup found"})
		return
	}

	// Verify TOTP code
	if settings.Method == "authenticator" {
		valid := totp.Validate(req.Code, settings.Secret)
		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": errInvalidVerificationCode})
			return
		}
	}

	// Generate backup codes
	backupCodes := generateBackupCodes(10)

	// Enable 2FA
	settings.IsEnabled = true
	settings.PendingSetup = false
	settings.BackupCodes = backupCodes
	now := time.Now()
	settings.ActivatedAt = &now
	db.DB.Save(&settings)

	// Log
	middleware.LogCriticalOperation(c, "2fa_activated", map[string]interface{}{
		"method": settings.Method,
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "2FA activated successfully",
		"data": gin.H{
			"backupCodes": backupCodes,
		},
	})
}

// DisableTwoFactor disables 2FA for the user
// @Summary Disable 2FA
// @Description Disable two-factor authentication
// @Tags admin,security
// @Accept json
// @Produce json
// @Param request body map[string]string true "Verification code"
// @Success 200 {object} map[string]string
// @Router /api/admin/security/2fa/disable [post]
func DisableTwoFactor(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("userId")

	var settings models.TwoFactorSettings
	if err := db.DB.First(&settings, userIDQuery, userID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err2FANotEnabled})
		return
	}

	// Verify code before disabling
	if settings.Method == "authenticator" {
		valid := totp.Validate(req.Code, settings.Secret)
		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": errInvalidVerificationCode})
			return
		}
	}

	settings.IsEnabled = false
	now := time.Now()
	settings.DeactivatedAt = &now
	db.DB.Save(&settings)

	middleware.LogCriticalOperation(c, "2fa_disabled", nil)

	c.JSON(http.StatusOK, gin.H{"message": "2FA disabled successfully"})
}

// RegenerateBackupCodes generates new backup codes
// @Summary Regenerate backup codes
// @Description Generate new backup codes for 2FA
// @Tags admin,security
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/security/2fa/backup-codes [post]
func RegenerateBackupCodes(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("userId")

	var settings models.TwoFactorSettings
	if err := db.DB.First(&settings, userIDQuery, userID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err2FANotEnabled})
		return
	}

	// Verify code
	if settings.Method == "authenticator" {
		valid := totp.Validate(req.Code, settings.Secret)
		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": errInvalidVerificationCode})
			return
		}
	}

	backupCodes := generateBackupCodes(10)
	settings.BackupCodes = backupCodes
	db.DB.Save(&settings)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"backupCodes": backupCodes,
		},
	})
}

// VerifyTwoFactorLogin validates a TOTP code + challengeId for admin-panel 2FA login.
// The challengeId must have been issued by the Login handler and stored in Redis;
// it is consumed on first use to prevent replay attacks.
func VerifyTwoFactorLogin(c *gin.Context) {
	var req struct {
		ChallengeID string `json:"challengeId" binding:"required"`
		Code        string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ip := c.ClientIP()
	var userID string

	if db.Redis == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Authentication service is temporarily unavailable (Redis connection required for 2FA)"})
		return
	}

	challengeKey := fmt.Sprintf("2fa_challenge:%s", req.ChallengeID)
	ctx := c.Request.Context()

	// Read and atomically delete the key (one-time use)
	val, err := db.Redis.GetDel(ctx, challengeKey).Result()
	if err != nil || val == "" {
		_ = LogSecurityEvent("", models.SecurityEvent2FAFailed, ip, c.Request.UserAgent(), nil, nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired challenge. Please sign in again."})
		return
	}
	userID = val

	var email string
	if userID != "" && db.DB != nil {
		_ = db.DB.Model(&models.User{}).Where("id = ?", userID).Pluck("email", &email).Error
	}
	if email != "" && isIPBlocked(c, email, ip) {
		_ = LogSecurityEvent(userID, models.SecurityEvent2FAFailed, ip, c.Request.UserAgent(), nil, nil)
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Account temporarily locked. Please try again after 15 minutes."})
		return
	}

	twoFAKey := fmt.Sprintf("2fa_attempts:%s:%s", userID, ip)
	if db.Redis != nil {
		attempts, err := db.Redis.Get(c.Request.Context(), twoFAKey).Int()
		if err == nil && attempts >= MaxLoginAttempts {
			_ = LogSecurityEvent(userID, models.SecurityEvent2FAFailed, ip, c.Request.UserAgent(), nil, nil)
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many failed attempts. Please try again after 15 minutes."})
			return
		}
	}

	var settings models.TwoFactorSettings
	if err := db.DB.First(&settings, userIDQuery, userID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "2FA not configured"})
		return
	}

	if !settings.IsEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "2FA is not enabled for this account"})
		return
	}

	codeValid := false
	if settings.Method == "authenticator" && settings.Secret != "" {
		codeValid = totp.Validate(req.Code, settings.Secret)
	}

	// Also allow backup codes
	if !codeValid && len(settings.BackupCodes) > 0 {
		for i, bc := range settings.BackupCodes {
			if bc == req.Code {
				// Consume the backup code
				settings.BackupCodes = append(settings.BackupCodes[:i], settings.BackupCodes[i+1:]...)
				db.DB.Model(&settings).Update("backup_codes", settings.BackupCodes)
				codeValid = true
				break
			}
		}
	}

	if !codeValid {
		if db.Redis != nil {
			db.Redis.Incr(c.Request.Context(), twoFAKey)
			db.Redis.Expire(c.Request.Context(), twoFAKey, LockoutDuration)
		}
		_ = LogSecurityEvent(userID, models.SecurityEvent2FAFailed, ip, c.Request.UserAgent(), nil, nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": errInvalidVerificationCode})
		return
	}

	// Clear brute-force counter on success
	if db.Redis != nil {
		db.Redis.Del(c.Request.Context(), twoFAKey)
	}

	// Update last used timestamp
	now := time.Now()
	db.DB.Model(&settings).Update("last_used_at", now)

	var user models.User
	if err := db.DB.Select("id, role, email").First(&user, idQuery, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User not found"})
		return
	}

	tokens, err := tokenService.GenerateTokenPair(user.ID, string(user.Role), user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": errFailedToGenerateTokens})
		return
	}

	userAgent := c.Request.UserAgent()
	session := &models.UserSession{
		ID:           tokens.JTI,
		UserID:       user.ID,
		RefreshToken: tokens.RefreshToken,
		UserAgent:    userAgent,
		IP:           ip,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		LastAccessed: time.Now(),
	}
	_ = db.DB.Create(session).Error

	middleware.LogCriticalOperation(c, "2fa_login_success", map[string]interface{}{
		"user_id": userID,
		"method":  settings.Method,
	})
	_ = LogSecurityEvent(userID, "2FA_LOGIN_SUCCESS", ip, userAgent, nil, nil)

	setAuthCookie(c, "access_token", tokens.AccessToken, 3600*24)
	setAuthCookie(c, "refresh_token", tokens.RefreshToken, 3600*24*30)

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"challengeId": req.ChallengeID,
	})
}

// GetUser2FAStatus returns the 2FA status for the authenticated user
func GetUser2FAStatus(c *gin.Context) {
	userID, _ := c.Get("userId")

	var settings models.TwoFactorSettings
	if err := db.DB.First(&settings, userIDQuery, userID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled": false,
			"data": gin.H{
				"isEnabled":  false,
				"method":     nil,
				"isEnforced": false,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled": settings.IsEnabled,
		"data": gin.H{
			"isEnabled":       settings.IsEnabled,
			"method":          settings.Method,
			"lastUsedAt":      settings.LastUsedAt,
			"isEnforced":      settings.IsEnforced,
			"verifiedDevices": settings.VerifiedDevices,
		},
	})
}

// InitiateUser2FASetup generates TOTP secret and QR code image URL for the authenticated user
func InitiateUser2FASetup(c *gin.Context) {
	userID, _ := c.Get("userId")

	userIDStr, ok := userID.(string)
	if !ok || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var user models.User
	if err := db.DB.First(&user, idQuery, userIDStr).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Generate TOTP secret
	secret := make([]byte, 20)
	rand.Read(secret)
	secretKey := base32.StdEncoding.EncodeToString(secret)

	// Generate QR code locally as base64
	otpAuthURL := fmt.Sprintf("otpauth://totp/Thanawy:%s?secret=%s&issuer=Thanawy", user.Email, secretKey)
	qrCodeBase64, err := generateQRCodeBase64(otpAuthURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate QR code"})
		return
	}

	// Store pending setup
	var settings models.TwoFactorSettings
	err = db.DB.Where(userIDQuery, userIDStr).First(&settings).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			settings = models.TwoFactorSettings{
				ID:           uuid.NewString(),
				UserID:       userIDStr,
				Method:       "authenticator",
				Secret:       secretKey,
				IsEnabled:    false,
				PendingSetup: true,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}
			if err := db.DB.Create(&settings).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store pending 2FA setup"})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
	} else {
		settings.Secret = secretKey
		settings.Method = "authenticator"
		settings.PendingSetup = true
		settings.UpdatedAt = time.Now()
		if err := db.DB.Save(&settings).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store pending 2FA setup"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"secret": secretKey,
		"qrCode": qrCodeBase64,
	})
}

// EnableUser2FA verifies the 2FA token/code and enables it for the authenticated user
func EnableUser2FA(c *gin.Context) {
	var req struct {
		Secret string `json:"secret" binding:"required"`
		Token  string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("userId")

	// Verify TOTP code
	valid := totp.Validate(req.Token, req.Secret)
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": errInvalidVerificationCode})
		return
	}

	// Generate backup codes
	backupCodes := generateBackupCodes(10)

	tx := db.DB.Begin()

	var settings models.TwoFactorSettings
	err := tx.First(&settings, userIDQuery, userID).Error
	if err != nil {
		settings = models.TwoFactorSettings{
			UserID: userID.(string),
		}
	}

	now := time.Now()
	settings.Method = "authenticator"
	settings.Secret = req.Secret
	settings.IsEnabled = true
	settings.PendingSetup = false
	settings.BackupCodes = backupCodes
	settings.ActivatedAt = &now
	settings.UpdatedAt = now

	if err := tx.Save(&settings).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save 2FA settings"})
		return
	}

	if err := tx.Model(&models.User{}).Where(idQuery, userID).Updates(map[string]interface{}{
		"two_factor_enabled": true,
		"two_factor_secret":  req.Secret,
		"updated_at":         now,
	}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable 2FA on user account"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	middleware.LogCriticalOperation(c, "2fa_activated", map[string]interface{}{
		"method": "authenticator",
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "2FA activated successfully",
		"data": gin.H{
			"backupCodes": backupCodes,
		},
	})
}

// DisableUser2FA disables 2FA for the authenticated user without code verification
func DisableUser2FA(c *gin.Context) {
	userID, _ := c.Get("userId")

	var settings models.TwoFactorSettings
	if err := db.DB.First(&settings, userIDQuery, userID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err2FANotEnabled})
		return
	}

	tx := db.DB.Begin()

	settings.IsEnabled = false
	now := time.Now()
	settings.DeactivatedAt = &now
	settings.UpdatedAt = now
	if err := tx.Save(&settings).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable 2FA settings"})
		return
	}

	if err := tx.Model(&models.User{}).Where(idQuery, userID).Updates(map[string]interface{}{
		"two_factor_enabled": false,
		"two_factor_secret":  nil,
		"updated_at":         now,
	}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user account"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	middleware.LogCriticalOperation(c, "2fa_disabled", nil)

	c.JSON(http.StatusOK, gin.H{"message": "2FA disabled successfully"})
}

func generateBackupCodes(count int) []string {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		// Generate 8-character alphanumeric code
		b := make([]byte, 4)
		rand.Read(b)
		codes[i] = base32.StdEncoding.EncodeToString(b)[:8]
	}
	return codes
}
