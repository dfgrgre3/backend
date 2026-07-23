package middleware

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"thanawy-backend/internal/config"

	"github.com/gin-gonic/gin"
)

// getImpersonationKey helper to decode hex-encoded impersonation secret
func getImpersonationKey(secret string) []byte {
	if decoded, err := hex.DecodeString(secret); err == nil {
		return decoded
	}
	return []byte(secret)
}

// SignImpersonationToken generates an HMAC-SHA256 signed impersonation token.
func SignImpersonationToken(userID string, adminID string) string {
	cfg := config.Load()
	secret := cfg.ImpersonationSecret
	if secret == "" {
		return ""
	}

	payload := fmt.Sprintf("%s:%s:%d", adminID, userID, time.Now().Unix())
	mac := hmac.New(sha256.New, getImpersonationKey(secret))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%s", payload, sig)
}

// SignImpersonationCSRFToken generates a CSRF token for impersonation sessions.
func SignImpersonationCSRFToken(adminID string) string {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(adminID))
	mac.Write(raw)
	return hex.EncodeToString(mac.Sum(nil))
}

// ClearImpersonationCookies removes impersonation state cookies.
func ClearImpersonationCookies(c *gin.Context) {
	cfg := config.Load()
	secure := cfg.CookieSecure
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("impersonate_user_id", "", -1, "/", cfg.CookieDomain, secure, true)
	c.SetCookie("impersonate_csrf_token", "", -1, "/", cfg.CookieDomain, secure, false)
	c.SetCookie("impersonation_token", "", -1, "/", cfg.CookieDomain, secure, true)
	c.SetCookie("impersonation_csrf", "", -1, "/", cfg.CookieDomain, secure, false)
}

// validateImpersonationToken verifies the signed impersonation token and returns target user details
func validateImpersonationToken(tokenStr string, currentAdminID string) (string, string, string, []string, error) {
	cfg := config.Load()
	secret := cfg.ImpersonationSecret
	if secret == "" {
		return "", "", "", nil, errors.New("impersonation secret is not configured")
	}

	parts := strings.Split(tokenStr, ".")
	if len(parts) != 2 {
		return "", "", "", nil, errors.New("invalid token format")
	}

	payload := parts[0]
	sigHex := parts[1]

	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return "", "", "", nil, errors.New("invalid signature format")
	}

	mac := hmac.New(sha256.New, getImpersonationKey(secret))
	mac.Write([]byte(payload))
	expectedSig := mac.Sum(nil)

	if subtle.ConstantTimeCompare(sig, expectedSig) != 1 {
		return "", "", "", nil, errors.New("signature mismatch")
	}

	payloadParts := strings.Split(payload, ":")
	if len(payloadParts) != 3 {
		return "", "", "", nil, errors.New("invalid payload format")
	}

	adminID := payloadParts[0]
	targetUserID := payloadParts[1]
	timestampStr := payloadParts[2]

	if adminID != currentAdminID {
		return "", "", "", nil, errors.New("admin user mismatch")
	}

	timestampUnix, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return "", "", "", nil, errors.New("invalid timestamp format")
	}

	tokenTime := time.Unix(timestampUnix, 0)
	if time.Since(tokenTime) > 1*time.Hour {
		return "", "", "", nil, errors.New("token expired")
	}

	role, perms, err := resolveUserFromDB(targetUserID)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("failed to resolve target user: %w", err)
	}

	return targetUserID, adminID, role, perms, nil
}
