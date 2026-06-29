package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"thanawy-backend/internal/config"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"
	"thanawy-backend/internal/services"

	"github.com/gin-gonic/gin"
)

// ─────────────────────────────────────────────
//  L1 in-memory role+permissions cache
// ─────────────────────────────────────────────

type rolePermsEntry struct {
	role        string
	permissions []string
	expiresAt   time.Time
}

var (
	rolePermsCache    sync.Map
	rolePermsCacheTTL = 5 * time.Minute
)

func getRolePermsFromCache(userID string) (string, []string, bool) {
	v, ok := rolePermsCache.Load(userID)
	if !ok {
		return "", nil, false
	}
	e := v.(*rolePermsEntry)
	if time.Now().After(e.expiresAt) {
		rolePermsCache.Delete(userID)
		return "", nil, false
	}
	return e.role, e.permissions, true
}

func setRolePermsCache(userID, role string, permissions []string) {
	rolePermsCache.Store(userID, &rolePermsEntry{
		role:        role,
		permissions: permissions,
		expiresAt:   time.Now().Add(rolePermsCacheTTL),
	})
}

// InvalidateRolePermsCache removes a user's cached role/permissions.
// Call this after any role or permission update.
func InvalidateRolePermsCache(userID string) {
	rolePermsCache.Delete(userID)
	// Also invalidate in Redis if available
	if db.Redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		db.Redis.Del(ctx, fmt.Sprintf("role_perms:%s", userID))
	}
}

// StartDistributedCacheInvalidator subscribes to Redis Pub/Sub for cross-node invalidation.
// If Redis is unavailable it becomes a no-op.
func StartDistributedCacheInvalidator() {
	if db.Redis == nil {
		return
	}
	go func() {
		sub := db.Redis.Subscribe(context.Background(), "cache:invalidate:role_perms")
		ch := sub.Channel()
		for msg := range ch {
			userID := msg.Payload
			if userID != "" {
				rolePermsCache.Delete(userID)
			}
		}
	}()
}

// ─────────────────────────────────────────────
//  Helpers
// ─────────────────────────────────────────────

// extractBearerToken extracts JWT from Authorization header or __session cookie
func extractBearerToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	if cookie, err := c.Cookie("access_token"); err == nil && cookie != "" {
		return cookie
	}
	// External auth provider sets __session for same-origin browser requests.
	if cookie, err := c.Cookie("__session"); err == nil && cookie != "" {
		return cookie
	}
	return ""
}

// resolveUserFromDB fetches user role/permissions from DB with caching
func resolveUserFromDB(userID string) (string, []string, error) {
	// L1 in-memory cache
	if role, perms, ok := getRolePermsFromCache(userID); ok {
		return role, perms, nil
	}

	// L2 Redis cache
	if db.Redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		cacheKey := fmt.Sprintf("role_perms:%s", userID)
		val, err := db.Redis.Get(ctx, cacheKey).Result()
		if err == nil && val != "" {
			parts := strings.SplitN(val, "|", 2)
			role := parts[0]
			var perms []string
			if len(parts) == 2 && parts[1] != "" {
				perms = strings.Split(parts[1], ",")
			}
			setRolePermsCache(userID, role, perms)
			return role, perms, nil
		}
	}

	// DB lookup
	if db.DB == nil {
		return string(models.RoleStudent), []string{}, nil
	}
	var user models.User
	if err := db.DB.Select("id, role, permissions, status").Where("id = ?", userID).First(&user).Error; err != nil {
		return "", nil, err
	}

	// Block suspended users
	if user.Status == models.StatusSuspended {
		return "", nil, fmt.Errorf("user account suspended")
	}

	role := string(user.Role)
	perms := user.GetEffectivePermissions()

	// Store in L1
	setRolePermsCache(userID, role, perms)

	// Store in L2 Redis (best-effort)
	if db.Redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		cacheKey := fmt.Sprintf("role_perms:%s", userID)
		cacheVal := role + "|" + strings.Join(perms, ",")
		db.Redis.Set(ctx, cacheKey, cacheVal, rolePermsCacheTTL)
	}

	return role, perms, nil
}

// ─────────────────────────────────────────────
//  Core Auth Middleware
// ─────────────────────────────────────────────

// Auth validates the external provider JWT, resolves user identity, and sets gin context keys.
// It ABORTS with 401 if the token is missing or invalid.
func Auth() gin.HandlerFunc {
	tokenSvc := services.NewAuthTokenService()
	return func(c *gin.Context) {
		tokenString := extractBearerToken(c)
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
				"code":  "missing_token",
			})
			return
		}

		claims, err := tokenSvc.ValidateAccessToken(tokenString)
		if err != nil {
			log.Printf("[Auth] Token validation failed: %v", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
				"code":  "invalid_token",
			})
			return
		}

		dbUserID := claims.UserID
		role, permissions, err := resolveUserFromDB(dbUserID)
		if err != nil {
			log.Printf("[Auth] Failed to resolve user %s: %v", dbUserID, err)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": err.Error(),
				"code":  "user_forbidden",
			})
			return
		}

		// Handle admin impersonation if impersonation cookie is present
		isAdmin := role == string(models.RoleAdmin) || role == string(models.RoleSuperAdmin)
		if isAdmin {
			if impersonateCookie, errCookie := c.Cookie("impersonate_user_id"); errCookie == nil && impersonateCookie != "" {
				targetUserID, adminID, impersonateRole, impersonatePerms, errImp := validateImpersonationToken(impersonateCookie, dbUserID)
				if errImp != nil {
					log.Printf("[Auth] Impersonation validation failed: %v", errImp)
					ClearImpersonationCookies(c)
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
						"error": "Impersonation session invalid or expired: " + errImp.Error(),
						"code":  "impersonation_expired",
					})
					return
				}

				// Validate impersonation CSRF for write methods
				if c.Request.Method != "GET" && c.Request.Method != "HEAD" && c.Request.Method != "OPTIONS" {
					csrfCookie, errCsrf := c.Cookie("impersonate_csrf_token")
					if errCsrf != nil || csrfCookie == "" {
						ClearImpersonationCookies(c)
						c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
							"error": "Impersonation CSRF token missing",
							"code":  "impersonation_csrf_missing",
						})
						return
					}
					expectedCsrf := SignImpersonationCSRFToken(adminID)
					if subtle.ConstantTimeCompare([]byte(csrfCookie), []byte(expectedCsrf)) != 1 {
						ClearImpersonationCookies(c)
						c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
							"error": "Impersonation CSRF token mismatch",
							"code":  "impersonation_csrf_mismatch",
						})
						return
					}
				}

				// Set target user details in context
				c.Set("userId", targetUserID)
				c.Set("user_id", targetUserID)
				c.Set("role", impersonateRole)
				c.Set("user_role", impersonateRole)
				c.Set("permissions", impersonatePerms)
				c.Set("is_admin", impersonateRole == string(models.RoleAdmin) || impersonateRole == string(models.RoleSuperAdmin))
				c.Set("jti", claims.ID)
				if claims.ExpiresAt != nil {
					c.Set("accessTokenExpiresAt", claims.ExpiresAt.Unix())
				}
				c.Set("isImpersonating", true)
				c.Set("adminUserId", adminID)

				log.Printf("[Auth] Impersonation active: Admin %s is impersonating User %s (%s)", adminID, targetUserID, impersonateRole)
				c.Next()
				return
			}
		}

		// Set context keys (both legacy and new names for compatibility)
		c.Set("userId", dbUserID)
		c.Set("user_id", dbUserID)

		c.Set("user_email", claims.Email)
		c.Set("role", role)
		c.Set("user_role", role)
		c.Set("permissions", permissions)
		c.Set("is_admin", isAdmin)
		c.Set("jti", claims.ID)
		if claims.ExpiresAt != nil {
			c.Set("accessTokenExpiresAt", claims.ExpiresAt.Unix())
		}

		c.Next()
	}
}

// OptionalAuth attempts to authenticate but does NOT abort on failure.
// Use this for endpoints that work for both guests and authenticated users.
func OptionalAuth() gin.HandlerFunc {
	tokenSvc := services.NewAuthTokenService()
	return func(c *gin.Context) {
		tokenString := extractBearerToken(c)
		if tokenString == "" {
			c.Next()
			return
		}

		claims, err := tokenSvc.ValidateAccessToken(tokenString)
		if err != nil {
			// Don't abort — treat as guest
			c.Next()
			return
		}

		dbUserID := claims.UserID

		role, permissions, err := resolveUserFromDB(dbUserID)
		if err == nil {
			isAdmin := role == string(models.RoleAdmin) || role == string(models.RoleSuperAdmin)
			if isAdmin {
				if impersonateCookie, errCookie := c.Cookie("impersonate_user_id"); errCookie == nil && impersonateCookie != "" {
					targetUserID, adminID, impersonateRole, impersonatePerms, errImp := validateImpersonationToken(impersonateCookie, dbUserID)
					if errImp == nil {
						c.Set("userId", targetUserID)
						c.Set("user_id", targetUserID)
						c.Set("role", impersonateRole)
						c.Set("user_role", impersonateRole)
						c.Set("permissions", impersonatePerms)
						c.Set("is_admin", impersonateRole == string(models.RoleAdmin) || impersonateRole == string(models.RoleSuperAdmin))
						c.Set("jti", claims.ID)
						if claims.ExpiresAt != nil {
							c.Set("accessTokenExpiresAt", claims.ExpiresAt.Unix())
						}
						c.Set("isImpersonating", true)
						c.Set("adminUserId", adminID)
						c.Next()
						return
					}
				}
			}

			c.Set("userId", dbUserID)
			c.Set("user_id", dbUserID)
			c.Set("user_email", claims.Email)
			c.Set("role", role)
			c.Set("user_role", role)
			c.Set("permissions", permissions)
			c.Set("is_admin", isAdmin)
			c.Set("jti", claims.ID)
			if claims.ExpiresAt != nil {
				c.Set("accessTokenExpiresAt", claims.ExpiresAt.Unix())
			}
		}

		c.Next()
	}
}

// ─────────────────────────────────────────────
//  Role Guards
// ─────────────────────────────────────────────

// AdminRequired ensures the user has ADMIN or SUPER_ADMIN role.
// AdminRequired ensures the user has ADMIN or SUPER_ADMIN role.
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("user_role")
		if !exists || role == nil {
			role, _ = c.Get("role")
		}
		roleStr, _ := role.(string)
		if roleStr != string(models.RoleAdmin) && roleStr != string(models.RoleSuperAdmin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Admin access required",
				"code":  "insufficient_role",
			})
			return
		}
		c.Next()
	}
}

// ModeratorRequired ensures the user has at least MODERATOR role.
func ModeratorRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("user_role")
		if !exists || role == nil {
			role, _ = c.Get("role")
		}
		roleStr, _ := role.(string)
		allowed := []string{
			string(models.RoleModerator),
			string(models.RoleAdmin),
			string(models.RoleSuperAdmin),
		}
		if !slices.Contains(allowed, roleStr) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Moderator or higher access required",
				"code":  "insufficient_role",
			})
			return
		}
		c.Next()
	}
}

// AdminOrModerator allows ADMIN, SUPER_ADMIN, MODERATOR, and SUPPORT.
func AdminOrModerator() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("user_role")
		if !exists || role == nil {
			role, _ = c.Get("role")
		}
		roleStr, _ := role.(string)
		allowed := []string{
			string(models.RoleModerator),
			string(models.RoleAdmin),
			string(models.RoleSuperAdmin),
			string(models.RoleSupport),
		}
		if !slices.Contains(allowed, roleStr) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Admin, moderator or support access required",
				"code":  "insufficient_role",
			})
			return
		}
		c.Next()
	}
}

// RoleRequired ensures the user has one of the given roles.
func RoleRequired(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("user_role")
		if !exists || role == nil {
			role, _ = c.Get("role")
		}
		roleStr, _ := role.(string)
		if !slices.Contains(roles, roleStr) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("One of roles %v required", roles),
				"code":  "insufficient_role",
			})
			return
		}
		c.Next()
	}
}

// PermissionRequired validates that the user holds a specific permission string.
// Uses wildcard matching defined in models.User.HasPermission.
func PermissionRequired(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		permsRaw, _ := c.Get("permissions")
		perms, _ := permsRaw.([]string)

		// Build a minimal User to leverage HasPermission logic
		u := &models.User{
			Permissions: models.JSONStringArray(perms),
		}
		roleRaw, exists := c.Get("user_role")
		if !exists || roleRaw == nil {
			roleRaw, _ = c.Get("role")
		}
		u.Role = models.UserRole(fmt.Sprintf("%v", roleRaw))

		if !u.HasPermission(permission) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("Permission '%s' required", permission),
				"code":  "insufficient_permission",
			})
			return
		}
		c.Next()
	}
}

// AnyAuthenticatedUser ensures that at least a valid user ID is set.
func AnyAuthenticatedUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists || userID == "" {
			userID, exists = c.Get("userId")
		}
		if !exists || userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
				"code":  "not_authenticated",
			})
			return
		}
		c.Next()
	}
}

// StrictRBAC is a Deny-by-Default guard that passes if Auth() already set a user.
// Pair this after AdminRequired/ModeratorRequired to ensure every route in a
// group explicitly has a role guard.
func StrictRBAC() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists || userID == "" {
			userID, exists = c.Get("userId")
		}
		if !exists || userID == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Access denied",
				"code":  "rbac_deny",
			})
			return
		}
		c.Next()
	}
}

// ─────────────────────────────────────────────
//  Impersonation Helpers
// ─────────────────────────────────────────────

// getImpersonationKey helper to decode hex-encoded impersonation secret
func getImpersonationKey(secret string) []byte {
	if decoded, err := hex.DecodeString(secret); err == nil {
		return decoded
	}
	return []byte(secret)
}

// SignImpersonationToken generates an HMAC-SHA256 signed impersonation token.
// IMPERSONATION_SECRET must be configured; no fallback secret is allowed.
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
	// Clear both cookie sets for backward compatibility
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

	// Format: adminID:userID:timestamp
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

// GuestOnly ensures that the request is made by a guest (unauthenticated user).
// If a valid access token is found, it aborts the request.
func GuestOnly() gin.HandlerFunc {
	tokenSvc := services.NewAuthTokenService()
	return func(c *gin.Context) {
		tokenString := extractBearerToken(c)
		if tokenString != "" {
			if _, err := tokenSvc.ValidateAccessToken(tokenString); err == nil {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "Already authenticated",
					"code":  "already_authenticated",
				})
				return
			}
		}
		c.Next()
	}
}

// TeacherRequired ensures the user has TEACHER, ADMIN, or SUPER_ADMIN role.
func TeacherRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("user_role")
		if !exists || role == nil {
			role, _ = c.Get("role")
		}
		roleStr, _ := role.(string)
		if roleStr != string(models.RoleTeacher) && roleStr != string(models.RoleAdmin) && roleStr != string(models.RoleSuperAdmin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Teacher access required",
				"code":  "insufficient_role",
			})
			return
		}
		c.Next()
	}
}

// StudentRequired ensures the user has STUDENT, ADMIN, or SUPER_ADMIN role.
func StudentRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("user_role")
		if !exists || role == nil {
			role, _ = c.Get("role")
		}
		roleStr, _ := role.(string)
		if roleStr != string(models.RoleStudent) && roleStr != string(models.RoleAdmin) && roleStr != string(models.RoleSuperAdmin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Student access required",
				"code":  "insufficient_role",
			})
			return
		}
		c.Next()
	}
}

// ─────────────────────────────────────────────
//  CORS Middleware
// ─────────────────────────────────────────────

// CORS handles Cross-Origin Resource Sharing headers.
// Deny-by-default: an origin is only allowed if it is explicitly whitelisted
// via CORS_ORIGINS. In development, localhost origins are permitted as a
// convenience when no whitelist is configured. Production NEVER reflects an
// arbitrary origin — an empty whitelist means no cross-origin access.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Load()

		// Build allowed origins from CORS_ORIGINS env var
		allowedOrigins := map[string]bool{}
		corsOrigins := os.Getenv("CORS_ORIGINS")
		for _, o := range strings.Split(corsOrigins, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				allowedOrigins[o] = true
			}
		}

		origin := c.Request.Header.Get("Origin")

		// Resolve the origin to allow. Only whitelisted origins are reflected.
		// In development with no whitelist, fall back to localhost origins so
		// local frontends still work without explicit configuration.
		allowOrigin := ""
		if origin != "" {
			if allowedOrigins[origin] {
				allowOrigin = origin
			} else if cfg.Environment == "development" && len(allowedOrigins) == 0 && isLocalhostOrigin(origin) {
				allowOrigin = origin
			}
		}

		// Only emit CORS headers when we are actually allowing an origin.
		// Setting Access-Control-Allow-Credentials without an allowed origin
		// is both useless and a footgun.
		if allowOrigin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			c.Writer.Header().Set("Vary", "Origin")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Headers",
				"Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-CSRF-Impersonation-Token")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
			c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// isLocalhostOrigin reports whether the given origin is a localhost URL,
// used only as a development convenience fallback for CORS.
func isLocalhostOrigin(origin string) bool {
	return strings.HasPrefix(origin, "http://localhost:") ||
		strings.HasPrefix(origin, "http://127.0.0.1:") ||
		strings.HasPrefix(origin, "https://localhost:") ||
		strings.HasPrefix(origin, "https://127.0.0.1:")
}
