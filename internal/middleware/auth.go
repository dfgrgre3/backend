package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"thanawy-backend/internal/config"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"
	"thanawy-backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

var impersonationSignKey []byte
var impersonationKeyOnce sync.Once

func getImpersonationSignKey() []byte {
	impersonationKeyOnce.Do(func() {
		impersonationSignKey = make([]byte, 32)
		if _, err := rand.Read(impersonationSignKey); err != nil {
			panic("failed to generate secure random key for impersonation signing")
		}
	})
	return impersonationSignKey
}

func SignImpersonationToken(userID string) string {
	key := getImpersonationSignKey()
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(userID))
	signature := hex.EncodeToString(mac.Sum(nil))
	return userID + "." + signature
}

func VerifyImpersonationToken(token string) (string, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", false
	}
	userID := parts[0]
	signature := parts[1]

	key := getImpersonationSignKey()
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(userID))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return userID, true
	}
	return "", false
}

var (
	cachedConfig *config.Config
	configOnce   sync.Once

	localRolePermsTTL        = 5 * time.Minute
	rolePermsRedisTTL        = 5 * time.Minute
	localRolePermsMaxEntries = 10000
)

var (
	localRolePermsCache = cache.New(localRolePermsTTL, 10*time.Minute)
	userContextSF       singleflight.Group
)

type userAuthContext struct {
	Role        string
	Permissions []string
}

// InvalidateRolePermsCache evicts a user's cached role/permissions
func InvalidateRolePermsCache(userID string) {
	localRolePermsCache.Delete(userID)
}

// Context keys for storing user information in request context
type ContextKey string

const (
	UserContextKey  ContextKey = "user_id"
	RoleContextKey  ContextKey = "user_role"
	EmailContextKey ContextKey = "user_email"
)

func getConfig() *config.Config {
	configOnce.Do(func() {
		cachedConfig = config.Load()
	})
	return cachedConfig
}

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := extractToken(c)
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		tokenService := &services.TokenService{}
		claims, err := tokenService.ValidateToken(tokenString)
		if err != nil || claims.Subject == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		c.Set("userId", claims.Subject)
		c.Set("jti", claims.JTI)
		if claims.ExpiresAt != nil {
			c.Set("accessTokenExpiresAt", claims.ExpiresAt.Time.UnixMilli())
		}

		hydrateUserContext(c, claims.Subject, claims.Role)
		processImpersonation(c, claims.Subject)

		c.Next()
	}
}

// Helper to extract JWT token from Authorization header or access_token cookie
func extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		token := strings.TrimSpace(authHeader[len("Bearer "):])
		if token != "" {
			return token
		}
	}

	if cookieToken, err := c.Cookie("access_token"); err == nil {
		return strings.TrimSpace(cookieToken)
	}
	return ""
}

// Helper to set user permissions in context
func setContextPermissions(c *gin.Context, permissions models.JSONStringArray) {
	if permissions == nil {
		c.Set("permissions", []string{})
	} else {
		c.Set("permissions", []string(permissions))
	}
}

// fetchCachedRolePerms checks local in-memory cache for user role/permissions.
// Returns the cached context and true if found and not expired.
func fetchCachedRolePerms(userID string) (*userAuthContext, bool) {
	val, found := localRolePermsCache.Get(userID)
	if !found {
		return nil, false
	}
	return val.(*userAuthContext), true
}

// fetchRedisRolePerms attempts to retrieve user role/permissions from Redis cache.
// Returns the cached context and true if found and parsed successfully.
func fetchRedisRolePerms(cacheKey string) (*userAuthContext, bool) {
	if db.Redis == nil {
		return nil, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	cachedVal, err := db.Redis.Get(ctx, cacheKey).Result()
	if err != nil {
		return nil, false
	}

	parts := strings.SplitN(cachedVal, "|", 2)
	if len(parts) != 2 {
		return nil, false
	}

	permsVal := strings.Split(parts[1], ",")
	if len(permsVal) == 1 && permsVal[0] == "" {
		permsVal = []string{}
	}

	return &userAuthContext{Role: parts[0], Permissions: permsVal}, true
}

// storeInLocalCache populates the in-memory cache for the given user.
func storeInLocalCache(userID string, ctx *userAuthContext) {
	localRolePermsCache.Set(userID, ctx, cache.DefaultExpiration)
}

// fetchDatabaseRolePerms retrieves user role/permissions from the database.
// Caches the result in Redis (async) and local in-memory cache.
// Returns nil if the user is not found in the database.
func fetchDatabaseRolePerms(userID, cacheKey string) *userAuthContext {
	var user models.User
	if err := db.DB.Unscoped().
		Select("role", "permissions").
		Where("id = ?", userID).
		Take(&user).Error; err != nil {
		return nil
	}

	roleVal := string(user.Role)
	permsVal := []string(user.Permissions)
	if permsVal == nil {
		permsVal = []string{}
	}

	// Cache in Redis asynchronously
	if db.Redis != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			permsStr := strings.Join(permsVal, ",")
			db.Redis.Set(ctx, cacheKey, roleVal+"|"+permsStr, rolePermsRedisTTL)
		}()
	}

	authCtx := &userAuthContext{Role: roleVal, Permissions: permsVal}
	storeInLocalCache(userID, authCtx)
	return authCtx
}

// buildSingleFlightCallback creates the function used inside singleflight.Do
// to fetch user role/permissions from cache layers and finally database.
func buildSingleFlightCallback(userID, fallbackRole string) func() (interface{}, error) {
	return func() (interface{}, error) {
		// Re-check local cache inside singleflight (avoid duplicate work)
		if cached, ok := fetchCachedRolePerms(userID); ok {
			return cached, nil
		}

		cacheKey := fmt.Sprintf("user_role_perms:%s", userID)

		// Try Redis cache next
		if redisCtx, ok := fetchRedisRolePerms(cacheKey); ok {
			storeInLocalCache(userID, redisCtx)
			return redisCtx, nil
		}

		// Fallback to database
		if dbCtx := fetchDatabaseRolePerms(userID, cacheKey); dbCtx != nil {
			return dbCtx, nil
		}

		return &userAuthContext{Role: strings.ToUpper(fallbackRole), Permissions: []string{}}, nil
	}
}

// Helper to fetch and set user role/permissions in context from database or fallback
func hydrateUserContext(c *gin.Context, userID, fallbackRole string) {
	if db.DB == nil {
		log.Printf("WARN: Database connection is nil in hydrateUserContext for user %s", userID)
		c.Set("role", strings.ToUpper(fallbackRole))
		c.Set("permissions", []string{})
		return
	}

	// 1. Try local in-memory cache first to bypass Redis cloud network latency
	if cached, ok := fetchCachedRolePerms(userID); ok {
		c.Set("role", cached.Role)
		c.Set("permissions", cached.Permissions)
		return
	}

	// 2. Use singleflight to collapse concurrent calls for the same user
	res, err, _ := userContextSF.Do(userID, buildSingleFlightCallback(userID, fallbackRole))

	if err == nil {
		authCtx := res.(*userAuthContext)
		c.Set("role", authCtx.Role)
		c.Set("permissions", authCtx.Permissions)
	} else {
		c.Set("role", strings.ToUpper(fallbackRole))
		c.Set("permissions", []string{})
	}
}

// Helper to handle admin impersonation logic if applicable
func processImpersonation(c *gin.Context, adminID string) {
	currentRole, _ := c.Get("role")
	currentRoleStr, _ := currentRole.(string)
	if currentRoleStr == "" {
		currentRoleStr = "ADMIN" // Default fallback
	}

	if currentRoleStr != "ADMIN" && currentRoleStr != "SUPER_ADMIN" {
		return
	}

	impersonatedCookie, err := c.Cookie("impersonate_user_id")
	if err != nil || impersonatedCookie == "" {
		return
	}

	impersonatedID, ok := VerifyImpersonationToken(impersonatedCookie)
	if !ok {
		log.Printf("Security Warning: Admin %s attempted to impersonate with an invalid or tampered token: %s", adminID, impersonatedCookie)
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Invalid or tampered impersonation token"})
		return
	}

	if db.DB == nil {
		log.Printf("WARN: Database connection is nil in processImpersonation")
		return
	}

	var authCtx *userAuthContext
	// Try local in-memory cache first to bypass Redis cloud network latency
	if cached, ok := fetchCachedRolePerms(impersonatedID); ok {
		authCtx = cached
	} else {
		// Use singleflight to collapse concurrent calls for the same user
		res, err, _ := userContextSF.Do(impersonatedID, buildSingleFlightCallback(impersonatedID, ""))
		if err == nil {
			authCtx = res.(*userAuthContext)
		}
	}

	if authCtx != nil && authCtx.Role != "" {
		// Prevent privilege escalation: An ADMIN/SUPER_ADMIN cannot impersonate another user of equal or higher rank
		roleHierarchy := map[string]int{
			"STUDENT":     1,
			"TEACHER":     2,
			"MODERATOR":   3,
			"ADMIN":       4,
			"SUPER_ADMIN": 5,
		}

		if roleHierarchy[currentRoleStr] <= roleHierarchy[authCtx.Role] {
			log.Printf("Security Warning: Admin %s with role %s attempted to impersonate user %s with role %s", adminID, currentRoleStr, impersonatedID, authCtx.Role)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Impersonating users of equal or higher administrative rank is not allowed"})
			return
		}

		c.Set("originalAdminId", adminID)
		c.Set("userId", impersonatedID)
		c.Set("role", authCtx.Role)
		c.Set("isImpersonating", true)
		c.Set("permissions", authCtx.Permissions)
	}
}

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		rs, _ := role.(string)
		if rs != "ADMIN" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}
		MarkRBACAuthorized(c)
		c.Next()
	}
}

func ModeratorRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		rs, _ := role.(string)
		if rs != "ADMIN" && rs != "MODERATOR" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Moderator access required"})
			return
		}
		MarkRBACAuthorized(c)
		c.Next()
	}
}

func AdminOrModerator() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		rs, _ := role.(string)
		if rs != "ADMIN" && rs != "MODERATOR" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin or Moderator access required"})
			return
		}
		MarkRBACAuthorized(c)
		c.Next()
	}
}

func RoleRequired(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentRole, _ := c.Get("role")
		for _, role := range roles {
			if currentRole == role {
				MarkRBACAuthorized(c)
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
	}
}

func PermissionRequired(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if user is authenticated
		role, roleExists := c.Get("role")
		if !roleExists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		// Check specific permission
		perms, _ := c.Get("permissions")
		var userPermissions []string
		if perms != nil {
			userPermissions = perms.([]string)
		}

		// Create a temporary user object to use the HasPermission logic
		user := &models.User{
			Role:        models.UserRole(role.(string)),
			Permissions: models.JSONStringArray(userPermissions),
		}

		if user.HasPermission(permission) {
			MarkRBACAuthorized(c)
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Missing required permission: " + permission})
	}
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		cfg := getConfig()
		isDev := cfg.Environment == "development" || cfg.Environment == ""

		allowedOrigins := []string{
			"http://localhost:3000",
			"http://localhost:3001",
			"https://thanawy.net",
			"https://www.thanawy.net",
		}

		isAllowed := isOriginAllowed(origin, isDev, allowedOrigins)

		// Only log CORS details in development to avoid production log noise
		if isDev {
			log.Printf("[CORS] Origin: '%s', IsAllowed: %v, Method: %s, Path: %s",
				origin, isAllowed, c.Request.Method, c.Request.URL.Path)
		}

		setCorsHeaders(c, origin, isAllowed)

		if c.Request.Method == "OPTIONS" {
			handleOptions(c, origin, isAllowed, isDev)
			return
		}

		c.Next()
	}
}

// Helper to check if the request origin is allowed
func isOriginAllowed(origin string, isDev bool, allowedOrigins []string) bool {
	if origin == "" {
		// In development, allow requests with no origin (e.g. from mobile apps, Postman, Electron)
		if isDev {
			return true
		}
		return false
	}

	// In development, allow localhost and LAN IPs
	if isDev && isLocalhostOrLAN(origin) {
		return true
	}

	// Check against explicit allowed origins
	for _, o := range allowedOrigins {
		if origin == o {
			return true
		}
	}

	return false
}

// Helper to check if origin is localhost or a LAN IP
func isLocalhostOrLAN(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}

	host := u.Hostname()
	if host == "" {
		host = u.Host
	}

	if strings.Contains(host, ":") {
		h, _, err := net.SplitHostPort(host)
		if err == nil {
			host = h
		}
	}

	lowerHost := strings.ToLower(host)
	if lowerHost == "localhost" || lowerHost == "127.0.0.1" || lowerHost == "::1" {
		return true
	}

	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() {
			return true
		}
	}

	return false
}

// Helper to set CORS response headers
func setCorsHeaders(c *gin.Context, origin string, isAllowed bool) {
	if isAllowed {
		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			// For requests with no origin (e.g. curl, mobile apps), allow all
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		}
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Dev-Admin-Bypass, accept, origin, Cache-Control, X-Requested-With, Connect-Protocol-Version, Connect-Timeout-Ms, Connect-Content-Encoding, X-Grpc-Web, X-User-Agent, Idempotency-Key")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
	c.Writer.Header().Set("Access-Control-Expose-Headers", "Grpc-Status, Grpc-Message, Grpc-Status-Details-Bin, Connect-Protocol-Version, Connect-Content-Encoding")
}

// Helper to handle CORS preflight OPTIONS requests
func handleOptions(c *gin.Context, origin string, isAllowed bool, isDev bool) {
	if isAllowed || (origin == "" && isDev) {
		c.AbortWithStatus(http.StatusNoContent)
	} else {
		c.AbortWithStatus(http.StatusForbidden)
	}
}
