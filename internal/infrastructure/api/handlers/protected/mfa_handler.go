package protected

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	authservice "thanawy-backend/internal/domain/auth/service"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"
	"time"

	"thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

type MFAHandler struct {
	mfaService authservice.MFAService
	tokenSvc   authservice.AuthTokenService
	authRepo   authservice.AuthService // using to create sessions
}

func NewMFAHandler(mfaService authservice.MFAService, tokenSvc authservice.AuthTokenService, authService authservice.AuthService) *MFAHandler {
	return &MFAHandler{
		mfaService: mfaService,
		tokenSvc:   tokenSvc,
		authRepo:   authService,
	}
}

func (h *MFAHandler) SetupMFA(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var user models.User
	if err := db.DB.WithContext(c.Request.Context()).Where("id = ?", userID).First(&user).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "User not found")
		return
	}

	if user.TwoFactorEnabled {
		response.Error(c, http.StatusBadRequest, "MFA is already enabled")
		return
	}

	secret, qrCodeURL, err := h.mfaService.GenerateTOTPSecret(user.Email)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to generate TOTP secret")
		return
	}

	// Store secret in TwoFactorCredential table
	twoFactorCredential := models.TwoFactorCredential{
		UserID: userID.(string),
		Secret: secret,
	}
	if err := db.DB.WithContext(c.Request.Context()).Save(&twoFactorCredential).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to update security settings")
		return
	}

	response.Success(c, gin.H{
		"secret":    secret,
		"qrCodeUrl": qrCodeURL,
	})
}

func (h *MFAHandler) EnableMFA(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	var twoFactorCredential models.TwoFactorCredential
	if err := db.DB.WithContext(c.Request.Context()).Where("user_id = ?", userID).First(&twoFactorCredential).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "MFA setup has not been initiated")
		return
	}

	if !h.mfaService.ValidateTOTP(twoFactorCredential.Secret, req.Code) {
		response.Error(c, http.StatusBadRequest, "Invalid verification code")
		return
	}

	// Generate backup codes
	rawBackupCodes := h.mfaService.GenerateBackupCodes()
	hashedBackupCodes := make([]string, len(rawBackupCodes))
	for i, bc := range rawBackupCodes {
		hash := sha256.Sum256([]byte(bc))
		hashedBackupCodes[i] = hex.EncodeToString(hash[:])
	}

	twoFactorCredential.Enabled = true
	twoFactorCredential.BackupCodes = strings.Join(hashedBackupCodes, ",")

	if err := db.DB.WithContext(c.Request.Context()).Save(&twoFactorCredential).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to enable MFA")
		return
	}

	response.Success(c, gin.H{
		"message":     "MFA enabled successfully",
		"backupCodes": rawBackupCodes, // Show once to user
	})
}

func (h *MFAHandler) DisableMFA(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	var twoFactorCredential models.TwoFactorCredential
	if err := db.DB.WithContext(c.Request.Context()).Where("user_id = ?", userID).First(&twoFactorCredential).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "MFA is not enabled")
		return
	}

	if !twoFactorCredential.Enabled {
		response.Error(c, http.StatusBadRequest, "MFA is not enabled")
		return
	}

	// Validate TOTP or Backup Code
	valid := false
	if h.mfaService.ValidateTOTP(twoFactorCredential.Secret, req.Code) {
		valid = true
	} else {
		// Check backup codes
		hash := sha256.Sum256([]byte(req.Code))
		hashedCode := hex.EncodeToString(hash[:])
		codes := strings.Split(twoFactorCredential.BackupCodes, ",")
		for i, c := range codes {
			if c == hashedCode {
				valid = true
				// Remove used backup code
				codes = append(codes[:i], codes[i+1:]...)
				twoFactorCredential.BackupCodes = strings.Join(codes, ",")
				break
			}
		}
	}

	if !valid {
		response.Error(c, http.StatusBadRequest, "Invalid code or backup code")
		return
	}

	twoFactorCredential.Enabled = false
	twoFactorCredential.Secret = ""
	twoFactorCredential.BackupCodes = ""

	if err := db.DB.WithContext(c.Request.Context()).Save(&twoFactorCredential).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to disable MFA")
		return
	}

	response.Success(c, gin.H{"message": "MFA disabled successfully"})
}

func (h *MFAHandler) VerifyMFA(c *gin.Context) {
	var req struct {
		Ticket string `json:"ticket" binding:"required"`
		Code   string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if cache.Redis == nil {
		response.Error(c, http.StatusInternalServerError, "Redis is required for MFA verification tickets")
		return
	}

	// Retrieve user ID from Redis ticket
	ctx := c.Request.Context()
	ticketKey := fmt.Sprintf("mfa_ticket:%s", req.Ticket)
	userID, err := cache.Redis.Get(ctx, ticketKey).Result()
	if err != nil || userID == "" {
		response.Error(c, http.StatusUnauthorized, "Invalid or expired verification ticket")
		return
	}

	// Delete ticket immediately to prevent replay
	cache.Redis.Del(ctx, ticketKey)

	var twoFactorCredential models.TwoFactorCredential
	if err := db.DB.WithContext(ctx).Where("user_id = ?", userID).First(&twoFactorCredential).Error; err != nil {
		response.Error(c, http.StatusUnauthorized, "User not found")
		return
	}

	// Verify TOTP or Backup Code
	valid := false
	if h.mfaService.ValidateTOTP(twoFactorCredential.Secret, req.Code) {
		valid = true
	} else {
		// Check backup codes
		hash := sha256.Sum256([]byte(req.Code))
		hashedCode := hex.EncodeToString(hash[:])
		codes := strings.Split(twoFactorCredential.BackupCodes, ",")
		for i, codeVal := range codes {
			if codeVal == hashedCode {
				valid = true
				// Remove used backup code
				codes = append(codes[:i], codes[i+1:]...)
				twoFactorCredential.BackupCodes = strings.Join(codes, ",")
				db.DB.WithContext(ctx).Save(&twoFactorCredential)
				break
			}
		}
	}

	if !valid {
		response.Error(c, http.StatusUnauthorized, "Invalid MFA code")
		return
	}

	// Get user for token generation
	var user models.User
	if err := db.DB.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		response.Error(c, http.StatusUnauthorized, "User not found")
		return
	}

	// Validated! Generate token pair and set cookies
	tokenPair, err := h.tokenSvc.GenerateTokenPair(&user)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to generate session tokens")
		return
	}

	// Save session
	userAgent := c.Request.UserAgent()
	ip := c.ClientIP()

	// Basic parsing
	uaLower := strings.ToLower(userAgent)
	osVal := "Unknown OS"
	if strings.Contains(uaLower, "windows") {
		osVal = "Windows"
	} else if strings.Contains(uaLower, "mac") {
		osVal = "MacOS"
	} else if strings.Contains(uaLower, "linux") {
		osVal = "Linux"
	}
	browser := "Unknown Browser"
	if strings.Contains(uaLower, "chrome") {
		browser = "Chrome"
	} else if strings.Contains(uaLower, "firefox") {
		browser = "Firefox"
	}

	// Create session via DB directly
	userSession := &models.UserSession{
		UserID:       user.ID,
		RefreshToken: tokenPair.RefreshToken,
		UserAgent:    userAgent,
		IP:           ip,
		Browser:      browser,
		OS:           osVal,
		DeviceType:   "web",
		Status:       "active",
		IsActive:     true,
		LastAccessed: time.Now(),
		ExpiresAt:    time.Now().Add(30 * 24 * time.Hour),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	db.DB.WithContext(ctx).Create(userSession)

	// Set cookies
	secureCookie := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("access_token", tokenPair.AccessToken, 15*60, "/", "", secureCookie, true)
	c.SetCookie("refresh_token", tokenPair.RefreshToken, 30*24*60*60, "/", "", secureCookie, true)

	response.Success(c, gin.H{
		"accessToken": tokenPair.AccessToken,
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.GetName(),
			"role":  string(user.Role),
		},
	})
}
