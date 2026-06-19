package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"
	"thanawy-backend/internal/repository"
	"thanawy-backend/internal/services"
)

var authService *services.AuthService
var tokenService = &services.TokenService{}

func InitAuthService(repo *repository.UserRepository) {
	authService = services.NewAuthService(repo)
}

// setAuthCookie writes an HttpOnly, SameSite=Lax auth cookie.
// Using SameSite=Lax prevents CSRF on state-changing requests while still
// allowing top-level GET navigations (e.g. OAuth redirects).
func setAuthCookie(c *gin.Context, name, value string, maxAgeSec int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, value, maxAgeSec, "/", "", isProduction(), true)
}

func getLoginAttemptsKey(email, ip string) string {
	return fmt.Sprintf("login_attempts:%s:%s", email, ip)
}

func isIPBlocked(c *gin.Context, email, ip string) bool {
	if db.Redis == nil {
		return false
	}
	key := getLoginAttemptsKey(email, ip)
	attempts, err := db.Redis.Get(c.Request.Context(), key).Int()
	if err != nil && err != redis.Nil {
		return false
	}
	return attempts >= MaxLoginAttempts
}

func recordLoginAttempt(c *gin.Context, email, ip string, success bool) {
	if db.Redis == nil {
		return
	}
	key := getLoginAttemptsKey(email, ip)
	if success {
		db.Redis.Del(c.Request.Context(), key)
		return
	}

	db.Redis.Incr(c.Request.Context(), key)
	db.Redis.Expire(c.Request.Context(), key, LockoutDuration)
}

type LoginRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required,min=6"`
	RememberMe bool   `json:"rememberMe"`
}

// Login handles user authentication
// @Summary User login
// @Description Authenticate user with email and password
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} map[string]interface{} "Login successful"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Router /auth/login [post]
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "بيانات الدخول غير صالحة: " + err.Error()})
		return
	}

	ip := c.ClientIP()
	userAgent := c.Request.UserAgent()
	email := strings.ToLower(strings.TrimSpace(req.Email))

	if isIPBlocked(c, email, ip) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "تم حظر محاولات الدخول مؤقتاً بسبب محاولات فاشلة متكررة. يرجى المحاولة بعد 15 دقيقة."})
		return
	}

	user, err := authService.Login(email, req.Password, ip, userAgent)
	if err != nil {
		recordLoginAttempt(c, email, ip, false)
		services.GetAuditService().LogAsync("", services.AuditEventLoginFailed, "auth", email, map[string]interface{}{"error": err.Error()}, ip, userAgent)
		_ = LogSecurityEvent("", models.SecurityEventLoginFailed, ip, userAgent, nil, nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "البريد الإلكتروني أو كلمة المرور غير صحيحة"})
		return
	}

	recordLoginAttempt(c, email, ip, true)

	if user.TwoFactorEnabled {
		challengeID := uuid.New().String()
		if db.Redis != nil {
			challengeKey := fmt.Sprintf("2fa_challenge:%s", challengeID)
			db.Redis.Set(c.Request.Context(), challengeKey, user.ID, 5*time.Minute)
		}

		c.JSON(http.StatusOK, gin.H{
			"success":     true,
			"requires2FA": true,
			"challengeId": challengeID,
			"user": gin.H{
				"id":    user.ID,
				"email": user.Email,
			},
		})
		return
	}

	tokens, err := tokenService.GenerateTokenPair(user.ID, string(user.Role), user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": errFailedToGenerateTokens, "details": err.Error()})
		return
	}

	location := getMockLocation(ip)
	expiryDuration := 24 * time.Hour
	if req.RememberMe {
		expiryDuration = 30 * 24 * time.Hour
	}

	session := &models.UserSession{
		ID:           tokens.JTI,
		UserID:       user.ID,
		RefreshToken: tokens.RefreshToken,
		UserAgent:    userAgent,
		IP:           ip,
		Location:     location,
		ExpiresAt:    time.Now().Add(expiryDuration),
		LastAccessed: time.Now(),
	}

	activeSessions, _ := getSessionRepo().GetActiveSessions(user.ID)
	if len(activeSessions) >= 2 {
		oldestIdx := 0
		for i, s := range activeSessions {
			if s.LastAccessed.Before(activeSessions[oldestIdx].LastAccessed) {
				oldestIdx = i
			}
		}
		_ = LogSecurityEvent(user.ID, "DEVICE_LIMIT_REACHED", ip, userAgent, location, nil)
		_ = getSessionRepo().RevokeSessionByJTI(activeSessions[oldestIdx].ID, user.ID)
	}

	if err := getSessionRepo().Create(session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session", "details": err.Error()})
		return
	}

	_ = LogSecurityEvent(user.ID, models.SecurityEventLoginSuccess, ip, userAgent, location, nil)
	services.GetAuditService().LogAsync(user.ID, services.AuditEventLogin, "auth", user.ID, map[string]interface{}{"ip": ip}, ip, userAgent)

	setAuthCookie(c, "access_token", tokens.AccessToken, 3600*24)
	refreshExpiry := int(expiryDuration.Seconds())
	setAuthCookie(c, "refresh_token", tokens.RefreshToken, refreshExpiry)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    user,
		"metadata": gin.H{
			"lastLogin": user.UpdatedAt,
			"ip":        ip,
			"device":    userAgent,
			"location":  location,
		},
	})
}

func Verify2FA(c *gin.Context) {
	var req struct {
		UserID     string `json:"userId"`
		Token      string `json:"token"`
		RememberMe bool   `json:"rememberMe"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	ip := c.ClientIP()

	twoFAKey := fmt.Sprintf("2fa_attempts:%s:%s", req.UserID, ip)
	if db.Redis != nil {
		attempts, err := db.Redis.Get(c.Request.Context(), twoFAKey).Int()
		if err == nil && attempts >= MaxLoginAttempts {
			_ = LogSecurityEvent(req.UserID, models.SecurityEvent2FAFailed, ip, c.Request.UserAgent(), nil, nil)
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many failed attempts. Please try again after 15 minutes."})
			return
		}
	}

	user, err := getUserRepo().FindByID(req.UserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": errUserNotFound})
		return
	}

	tokenValid := false

	if user.TwoFactorEnabled && user.TwoFactorSecret != nil && *user.TwoFactorSecret != "" {
		if validateTOTP(*user.TwoFactorSecret, req.Token) {
			tokenValid = true
		}
	}

	if !tokenValid && user.VerificationToken != nil && *user.VerificationToken == req.Token {
		if user.VerificationExpires != nil && user.VerificationExpires.After(time.Now()) {
			tokenValid = true
			db.DB.Model(&user).Updates(map[string]interface{}{
				"verification_token":   nil,
				"verification_expires": nil,
			})
		}
	}

	if !tokenValid {
		if db.Redis != nil {
			db.Redis.Incr(c.Request.Context(), twoFAKey)
			db.Redis.Expire(c.Request.Context(), twoFAKey, LockoutDuration)
		}
		_ = LogSecurityEvent(user.ID, models.SecurityEvent2FAFailed, ip, c.Request.UserAgent(), nil, nil)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired verification code"})
		return
	}

	if db.Redis != nil {
		db.Redis.Del(c.Request.Context(), twoFAKey)
	}

	tokens, err := tokenService.GenerateTokenPair(user.ID, string(user.Role), user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": errFailedToGenerateTokens})
		return
	}

	expiryDuration := 24 * time.Hour
	if req.RememberMe {
		expiryDuration = 30 * 24 * time.Hour
	}

	session := &models.UserSession{
		ID:           tokens.JTI,
		UserID:       user.ID,
		RefreshToken: tokens.RefreshToken,
		UserAgent:    c.Request.UserAgent(),
		IP:           ip,
		ExpiresAt:    time.Now().Add(expiryDuration),
		LastAccessed: time.Now(),
	}
	_ = getSessionRepo().Create(session)

	_ = LogSecurityEvent(user.ID, "2FA_SUCCESS", ip, c.Request.UserAgent(), nil, nil)

	setAuthCookie(c, "access_token", tokens.AccessToken, 3600*24)
	setAuthCookie(c, "refresh_token", tokens.RefreshToken, int(expiryDuration.Seconds()))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    user,
	})
}

func validateTOTP(secretBase32, token string) bool {
	secret := strings.ToUpper(secretBase32)
	secretBytes, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		return false
	}

	timeStep := time.Now().Unix() / 30

	for _, offset := range []int64{-1, 0, 1} {
		expectedToken := generateTOTP(secretBytes, timeStep+offset)
		if expectedToken == token {
			return true
		}
	}

	return false
}

func generateTOTP(secret []byte, timeStep int64) string {
	msg := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		msg[i] = byte(timeStep & 0xff)
		timeStep >>= 8
	}

	mac := hmac.New(sha1.New, secret)
	mac.Write(msg)
	hash := mac.Sum(nil)

	offset := hash[len(hash)-1] & 0x0f
	truncated := ((int(hash[offset]) & 0x7f) << 24) |
		((int(hash[offset+1]) & 0xff) << 16) |
		((int(hash[offset+2]) & 0xff) << 8) |
		(int(hash[offset+3]) & 0xff)

	code := truncated % 1000000
	return fmt.Sprintf("%06d", code)
}

func RequestMagicLink(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errInvalidEmail})
		return
	}

	token, err := authService.RequestMagicLink(req.Email)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "If an account exists, a link has been sent."})
		return
	}

	_ = LogSecurityEvent("", models.SecurityEventMagicLinkRequested, c.ClientIP(), c.Request.UserAgent(), nil, nil)

	response := gin.H{
		"success": true,
		"message": "Magic link sent successfully",
	}
	if !isProduction() {
		log.Printf("[DEBUG] Magic link token generated: %s", token)
	}
	c.JSON(http.StatusOK, response)
}

func VerifyMagicLink(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token is required"})
		return
	}

	user, err := authService.VerifyMagicLink(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	tokens, err := tokenService.GenerateTokenPair(user.ID, string(user.Role), user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": errFailedToGenerateTokens})
		return
	}

	session := &models.UserSession{
		ID:           tokens.JTI,
		UserID:       user.ID,
		RefreshToken: tokens.RefreshToken,
		UserAgent:    c.Request.UserAgent(),
		IP:           c.ClientIP(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		LastAccessed: time.Now(),
	}
	_ = getSessionRepo().Create(session)

	_ = LogSecurityEvent(user.ID, models.SecurityEventMagicLinkLogin, c.ClientIP(), c.Request.UserAgent(), nil, nil)

	setAuthCookie(c, "access_token", tokens.AccessToken, 3600*24)
	setAuthCookie(c, "refresh_token", tokens.RefreshToken, 3600*24)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    user,
	})
}

func ForgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errInvalidEmail})
		return
	}

	token, err := authService.RequestPasswordReset(req.Email)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "If an account exists, a reset link has been sent."})
		return
	}

	_ = LogSecurityEvent("", models.SecurityEventPasswordResetReq, c.ClientIP(), c.Request.UserAgent(), nil, nil)

	response := gin.H{
		"success": true,
		"message": "Password reset link sent",
	}
	if !isProduction() {
		log.Printf("[DEBUG] Password reset token generated: %s", token)
	}
	c.JSON(http.StatusOK, response)
}

func ResetPassword(c *gin.Context) {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := authService.ResetPassword(req.Token, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Password reset successful"})
}

func VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token is required"})
		return
	}

	if err := authService.VerifyEmail(token); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Email verified successfully"})
}

func ResendVerification(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errInvalidEmail})
		return
	}

	token, err := authService.RequestEmailVerification(req.Email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to generate verification link"})
		return
	}

	response := gin.H{
		"success": true,
		"message": "Verification email sent",
	}
	if !isProduction() {
		log.Printf("[DEBUG] Verification token generated: %s", token)
	}
	c.JSON(http.StatusOK, response)
}

type refreshMetaEntry struct {
	data      []byte
	expiresAt time.Time
}

var (
	refreshMetaL1       sync.Map
	refreshMetaL1TTL    = 60 * time.Second
	refreshMetaRedisTTL = 5 * time.Minute
)

const refreshMetaRedisKeyFmt = "refresh_meta:%s"

func refreshMetaKey(tokenHash string) string { return fmt.Sprintf(refreshMetaRedisKeyFmt, tokenHash) }

func loadRefreshMeta(tokenHash string) (string, string, string, error) {
	if val, ok := refreshMetaL1.Load(tokenHash); ok {
		entry := val.(*refreshMetaEntry)
		if time.Now().Before(entry.expiresAt) {
			var m struct {
				UserID    string `json:"u"`
				Role      string `json:"r"`
				SessionID string `json:"s"`
			}
			if json.Unmarshal(entry.data, &m) == nil {
				return m.UserID, m.Role, m.SessionID, nil
			}
		}
		refreshMetaL1.Delete(tokenHash)
	}

	if db.Redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		cached, err := db.Redis.Get(ctx, refreshMetaKey(tokenHash)).Bytes()
		cancel()
		if err == nil && len(cached) > 0 {
			refreshMetaL1.Store(tokenHash, &refreshMetaEntry{
				data:      cached,
				expiresAt: time.Now().Add(refreshMetaL1TTL),
			})
			var m struct {
				UserID    string `json:"u"`
				Role      string `json:"r"`
				SessionID string `json:"s"`
			}
			if json.Unmarshal(cached, &m) == nil {
				return m.UserID, m.Role, m.SessionID, nil
			}
		}
	}

	type meta struct {
		UserID    string
		Role      string
		SessionID string
	}
	var m meta
	var res struct {
		ID        string
		UserID    string
		ExpiresAt time.Time
		IsActive  bool
	}
	err := db.DB.Model(&models.UserSession{}).
		Select("id", "user_id", "expires_at", "is_active").
		Where("refresh_token_hash = ?", tokenHash).
		Take(&res).Error
	if err != nil {
		return "", "", "", err
	}
	m.SessionID = res.ID
	m.UserID = res.UserID
	if u, err := getUserRepo().FindByID(m.UserID); err == nil {
		m.Role = string(u.Role)
	}
	payload, _ := json.Marshal(struct {
		UserID    string `json:"u"`
		Role      string `json:"r"`
		SessionID string `json:"s"`
	}{m.UserID, m.Role, m.SessionID})
	refreshMetaL1.Store(tokenHash, &refreshMetaEntry{
		data:      payload,
		expiresAt: time.Now().Add(refreshMetaL1TTL),
	})
	if db.Redis != nil {
		go func(key string, data []byte) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			db.Redis.Set(ctx, key, data, refreshMetaRedisTTL)
		}(refreshMetaKey(tokenHash), payload)
	}
	return m.UserID, m.Role, m.SessionID, nil
}

func RefreshToken(c *gin.Context) {
	var refreshToken string
	var err error

	authHeader := c.GetHeader("Authorization")
	if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
		refreshToken = strings.TrimPrefix(authHeader, "Bearer ")
	} else {
		refreshToken, err = c.Cookie("refresh_token")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Refresh token missing"})
			return
		}
	}

	claims, err := tokenService.ValidateRefreshToken(refreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
		return
	}

	tokenHash := models.ComputeRefreshTokenHash(refreshToken)

	userID, _, sessionID, err := loadRefreshMeta(tokenHash)
	if err != nil || sessionID == "" {
		session, sErr := getSessionRepo().FindByRefreshToken(refreshToken)
		if sErr != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid session"})
			return
		}
		if session.IsExpired() {
			_ = getSessionRepo().RevokeSessionByJTI(session.ID, session.UserID)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired"})
			return
		}
		userID = session.UserID
		sessionID = session.ID
	}

	if claims.Subject != userID {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token owner mismatch"})
		return
	}

	user, err := getUserRepo().FindByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": errUserNotFound})
		return
	}

	tokens, err := tokenService.GenerateTokenPair(user.ID, string(user.Role), user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh token"})
		return
	}

	newExpiry := time.Now().Add(30 * 24 * time.Hour)
	updatedSession, err := getSessionRepo().RotateToken(sessionID, refreshToken, tokens.RefreshToken, newExpiry)
	if err == nil {
		refreshMetaL1.Delete(tokenHash)
		if db.Redis != nil {
			go func(key string) {
				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				defer cancel()
				db.Redis.Del(ctx, key)
			}(refreshMetaKey(tokenHash))
		}

		newTokenHash := models.ComputeRefreshTokenHash(tokens.RefreshToken)
		payload, _ := json.Marshal(struct {
			UserID    string `json:"u"`
			Role      string `json:"r"`
			SessionID string `json:"s"`
		}{user.ID, string(user.Role), updatedSession.ID})
		refreshMetaL1.Store(newTokenHash, &refreshMetaEntry{
			data:      payload,
			expiresAt: time.Now().Add(refreshMetaL1TTL),
		})
		if db.Redis != nil {
			go func(key string, data []byte) {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()
				db.Redis.Set(ctx, key, data, refreshMetaRedisTTL)
			}(refreshMetaKey(newTokenHash), payload)
		}
	} else {
		_ = getSessionRepo().RevokeSessionByJTI(sessionID, userID)
		_ = getSessionRepo().Create(&models.UserSession{
			ID:           tokens.JTI,
			UserID:       user.ID,
			RefreshToken: tokens.RefreshToken,
			UserAgent:    c.Request.UserAgent(),
			IP:           c.ClientIP(),
			ExpiresAt:    newExpiry,
			LastAccessed: time.Now(),
		})
		refreshMetaL1.Delete(tokenHash)
		if db.Redis != nil {
			go func(key string) {
				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				defer cancel()
				db.Redis.Del(ctx, key)
			}(refreshMetaKey(tokenHash))
		}
	}

	setAuthCookie(c, "access_token", tokens.AccessToken, 3600*24)
	setAuthCookie(c, "refresh_token", tokens.RefreshToken, 3600*24*30)

	c.JSON(http.StatusOK, gin.H{
		"success":              true,
		"accessTokenExpiresAt": time.Now().Add(15 * time.Minute).UnixMilli(),
	})
}

func Logout(c *gin.Context) {
	if token, err := c.Cookie("access_token"); err == nil {
		if claims, err := tokenService.ValidateToken(token); err == nil {
			_ = getSessionRepo().RevokeSessionByJTI(claims.JTI, claims.Subject)
			services.GetAuditService().LogAsync(claims.Subject, services.AuditEventLogout, "auth", claims.Subject, nil, c.ClientIP(), c.Request.UserAgent())
		}
	}

	if refreshToken, err := c.Cookie("refresh_token"); err == nil && refreshToken != "" {
		tokenHash := models.ComputeRefreshTokenHash(refreshToken)
		refreshMetaL1.Delete(tokenHash)
		if db.Redis != nil {
			go func(key string) {
				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				defer cancel()
				db.Redis.Del(ctx, key)
			}(refreshMetaKey(tokenHash))
		}
	}

	setAuthCookie(c, "access_token", "", -1)
	setAuthCookie(c, "refresh_token", "", -1)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Logged out successfully"})
}

func GetAuthSessions(c *gin.Context) {
	userID, _ := c.Get("userId")
	var sessions []models.UserSession
	if err := db.DB.Where("user_id = ? AND "+isActiveQuery, userID, true).Find(&sessions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch sessions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sessions": sessions, "success": true})
}

func DeleteAuthSession(c *gin.Context) {
	sessionID := c.Query("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId is required"})
		return
	}

	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var session models.UserSession
	if err := db.DB.Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found or access denied"})
		return
	}

	if err := getSessionRepo().RevokeSessionByJTI(sessionID, userID.(string)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UpdateAuthSession(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetUserDevices(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var sessions []models.UserSession
	if err := db.DB.
		Where("user_id = ? AND is_active = ?", userID, true).
		Order("last_accessed DESC").
		Find(&sessions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch devices"})
		return
	}

	type DeviceInfo struct {
		ID           string  `json:"id"`
		UserAgent    string  `json:"userAgent"`
		IP           string  `json:"ip"`
		Location     *string `json:"location"`
		LastAccessed string  `json:"lastAccessed"`
		ExpiresAt    string  `json:"expiresAt"`
		IsCurrent    bool    `json:"isCurrent"`
	}

	currentJTI := ""
	if token, err := c.Cookie("access_token"); err == nil {
		if claims, err := tokenService.ValidateToken(token); err == nil {
			currentJTI = claims.JTI
		}
	}

	devices := make([]DeviceInfo, 0, len(sessions))
	for _, s := range sessions {
		devices = append(devices, DeviceInfo{
			ID:           s.ID,
			UserAgent:    s.UserAgent,
			IP:           s.IP,
			Location:     s.Location,
			LastAccessed: s.LastAccessed.Format(time.RFC3339),
			ExpiresAt:    s.ExpiresAt.Format(time.RFC3339),
			IsCurrent:    s.ID == currentJTI,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"devices": devices,
			"count":   len(devices),
		},
	})
}

type RegisterRequest struct {
	Email         string `json:"email" binding:"required,email"`
	Password      string `json:"password" binding:"required,min=8"`
	Username      string `json:"username" binding:"required"`
	Phone         string `json:"phone"`
	GradeLevel    string `json:"gradeLevel"`
	EducationType string `json:"educationType"`
	Section       string `json:"section"`
}

func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := services.RegisterInput{
		Email:         req.Email,
		Username:      req.Username,
		Password:      req.Password,
		Role:          models.RoleStudent,
		Phone:         req.Phone,
		GradeLevel:    req.GradeLevel,
		EducationType: req.EducationType,
		Section:       req.Section,
		IP:            c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	}

	user, err := authService.Register(input)
	if err != nil {
		if err.Error() == "user already exists" {
			c.JSON(http.StatusConflict, gin.H{"error": "البريد الإلكتروني مسجل بالفعل"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	token, tokenErr := authService.RequestEmailVerification(user.Email)
	if tokenErr != nil {
		log.Printf("[Register] Failed to generate verification token for user %s: %v", user.Email, tokenErr)
	}

	services.GetAuditService().LogAsync(user.ID, "user.register", "user", user.ID, nil, c.ClientIP(), c.Request.UserAgent())
	GlobalNotifyAdmins("مستخدم جديد", fmt.Sprintf("انضم %s إلى المنصة", user.Email), "success")

	response := gin.H{
		"success": true,
		"user":    user,
		"message": "Registration successful. Please verify your email.",
	}
	if !isProduction() {
		log.Printf("[DEBUG] Registration verification token generated: %s", token)
	}
	c.JSON(http.StatusCreated, response)
}
