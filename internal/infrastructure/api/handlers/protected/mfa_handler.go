package protected

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	authdto "thanawy-backend/internal/application/dto"
	authservice "thanawy-backend/internal/domain/auth/service"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"
	"thanawy-backend/internal/infrastructure/config"
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

// backup_codes is JSONB in the database. Keep the wire format as a JSON array
// and accept the old comma-separated representation so existing credentials
// remain usable after the storage fix.
func encodeBackupCodes(codes []string) string {
	encoded, err := json.Marshal(codes)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func decodeBackupCodes(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var codes []string
	if json.Unmarshal([]byte(raw), &codes) == nil {
		return codes
	}
	// Backward compatibility for rows written by the previous implementation.
	legacy := strings.Split(raw, ",")
	filtered := make([]string, 0, len(legacy))
	for _, code := range legacy {
		if strings.TrimSpace(code) != "" {
			filtered = append(filtered, code)
		}
	}
	return filtered
}

func NewMFAHandler(mfaService authservice.MFAService, tokenSvc authservice.AuthTokenService, authService authservice.AuthService) *MFAHandler {
	return &MFAHandler{
		mfaService: mfaService,
		tokenSvc:   tokenSvc,
		authRepo:   authService,
	}
}

func (h *MFAHandler) SetupMFA(c *gin.Context) {
	userIDVal, exists := c.Get("userId")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	userID, ok := userIDVal.(string)
	if !ok || userID == "" {
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
		UserID:      userID,
		Secret:      secret,
		BackupCodes: "[]",
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
	twoFactorCredential.BackupCodes = encodeBackupCodes(hashedBackupCodes)

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
		codes := decodeBackupCodes(twoFactorCredential.BackupCodes)
		for i, c := range codes {
			if c == hashedCode {
				valid = true
				// Remove used backup code
				codes = append(codes[:i], codes[i+1:]...)
				twoFactorCredential.BackupCodes = encodeBackupCodes(codes)
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
	twoFactorCredential.BackupCodes = "[]"

	if err := db.DB.WithContext(c.Request.Context()).Save(&twoFactorCredential).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to disable MFA")
		return
	}

	response.Success(c, gin.H{"message": "MFA disabled successfully"})
}

// @Summary Verify MFA challenge
// @Description Verify a one-time MFA code and establish a session.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body authdto.VerifyMFARequest true "MFA verification"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/v1/auth/mfa/verify [post]
func (h *MFAHandler) VerifyMFA(c *gin.Context) {
	var req authdto.VerifyMFARequest
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
	ticketKey := fmt.Sprintf("mfa_ticket:%s", req.ChallengeID)
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
		codes := decodeBackupCodes(twoFactorCredential.BackupCodes)
		for i, codeVal := range codes {
			if codeVal == hashedCode {
				valid = true
				// Remove used backup code
				codes = append(codes[:i], codes[i+1:]...)
				twoFactorCredential.BackupCodes = encodeBackupCodes(codes)
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
		ID:           tokenPair.JTI,
		UserID:       user.ID,
		RefreshToken: tokenPair.RefreshToken,
		UserAgent:    userAgent,
		IP:           ip,
		Browser:      browser,
		OS:           osVal,
		DeviceType:   "web",
		Status:       "active",
		IsActive:     true,
		RememberMe:   req.RememberMe,
		LastAccessed: time.Now(),
		ExpiresAt: time.Now().Add(func() time.Duration {
			if req.RememberMe {
				return 90 * 24 * time.Hour
			}
			return 30 * 24 * time.Hour
		}()),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.DB.WithContext(ctx).Create(userSession).Error; err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create session")
		return
	}

	// Set cookies
	cfg := config.Load()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("access_token", tokenPair.AccessToken, 15*60, "/", cfg.CookieDomain, secureCookie(c), true)
	refreshMaxAge := 30 * 24 * 60 * 60
	if req.RememberMe {
		refreshMaxAge = 90 * 24 * 60 * 60
	}
	c.SetCookie("refresh_token", tokenPair.RefreshToken, refreshMaxAge, "/", cfg.CookieDomain, secureCookie(c), true)

	// SECURITY: refresh token stays cookie-only (HttpOnly) — not echoed in
	// the body. See the SECURITY note in auth_handler.go Login for why.
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
