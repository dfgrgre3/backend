package middleware

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

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
	return e.role, slices.Clone(e.permissions), true
}

func setRolePermsCache(userID, role string, permissions []string) {
	rolePermsCache.Store(userID, &rolePermsEntry{
		role:        role,
		permissions: slices.Clone(permissions),
		expiresAt:   time.Now().Add(rolePermsCacheTTL),
	})
}

// InvalidateRolePermsCache removes a user's cached role/permissions.
// Call this after any role or permission update.
func InvalidateRolePermsCache(userID string) {
	rolePermsCache.Delete(userID)
	if db.Redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		db.Redis.Del(ctx, fmt.Sprintf("role_perms:%s", userID))
		// Notify every application instance so a permission change is effective
		// on the very next authenticated request, not after the local TTL.
		db.Redis.Publish(ctx, "cache:invalidate:role_perms", userID)
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

// extractBearerToken extracts JWT from Authorization header or __session cookie
func extractBearerToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	// WebSocket clients cannot set custom Authorization headers. The browser
	// client therefore sends the access token as a query parameter when opening
	// /api/ws. Keep headers/cookies as the preferred sources for normal HTTP
	// requests, and only use the query parameter as a fallback.
	if token := c.Query("access_token"); token != "" {
		return token
	}
	if cookie, err := c.Cookie("access_token"); err == nil && cookie != "" {
		return cookie
	}
	if cookie, err := c.Cookie("__session"); err == nil && cookie != "" {
		return cookie
	}
	return ""
}

// resolveUserFromDB fetches user role/permissions from DB with caching
func resolveUserFromDB(userID string) (string, []string, error) {
	if role, perms, ok := getRolePermsFromCache(userID); ok {
		return role, perms, nil
	}

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

	if db.DB == nil {
		return string(models.RoleStudent), []string{}, nil
	}
	var user models.User
	if err := db.DB.Select("id, role, permissions, status").Where("id = ?", userID).First(&user).Error; err != nil {
		return "", nil, err
	}

	if user.Status == models.StatusSuspended {
		return "", nil, fmt.Errorf("user account suspended")
	}

	role := string(user.Role)
	perms := user.GetEffectivePermissions()

	setRolePermsCache(userID, role, perms)

	if db.Redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		cacheKey := fmt.Sprintf("role_perms:%s", userID)
		cacheVal := role + "|" + strings.Join(perms, ",")
		db.Redis.Set(ctx, cacheKey, cacheVal, rolePermsCacheTTL)
	}

	return role, perms, nil
}

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

// GuestOnly ensures that the request is made by a guest (unauthenticated user).
// It blocks authenticated users from accessing auth handshake routes (login, register, etc.)
// by returning a 409 Conflict — indicating the request conflicts with the current auth state.
func GuestOnly() gin.HandlerFunc {
	tokenSvc := services.NewAuthTokenService()
	return func(c *gin.Context) {
		tokenString := extractBearerToken(c)
		if tokenString != "" {
			if _, err := tokenSvc.ValidateAccessToken(tokenString); err == nil {
				c.AbortWithStatusJSON(http.StatusConflict, gin.H{
					"error": "User is already authenticated. Please log out before attempting to log in again.",
					"code":  "already_authenticated",
				})
				return
			}
		}
		c.Next()
	}
}
