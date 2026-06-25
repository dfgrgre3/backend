package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
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

func init() {
	services.InvalidateCacheCallback = InvalidateRolePermsCache
}

func getImpersonationSignKey() ([]byte, error) {
	cfg := getConfig()
	if cfg.ImpersonationSecret == "" {
		return nil, fmt.Errorf("impersonation secret is unconfigured")
	}
	decodedKey, err := hex.DecodeString(cfg.ImpersonationSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to decode impersonation secret: %w", err)
	}
	if len(decodedKey) < 32 {
		return nil, fmt.Errorf("impersonation secret must be at least 32 bytes of valid hex")
	}
	return decodedKey, nil
}

func SignImpersonationToken(userID string, adminID string) string {
	expiresAt := time.Now().Add(15 * time.Minute).Unix()
	payload := fmt.Sprintf("%s:%d:%s", userID, expiresAt, adminID)

	key, err := getImpersonationSignKey()
	if err != nil {
		log.Printf("[Security CRITICAL] SignImpersonationToken failed to retrieve key: %v", err)
		return ""
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%d.%s.%s", userID, expiresAt, adminID, signature)
}

func VerifyImpersonationToken(token string, currentAdminID string) (string, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 4 {
		return "", false
	}
	userID := parts[0]
	expiresStr := parts[1]
	adminID := parts[2]
	signature := parts[3]

	expiresAt, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil || time.Now().Unix() > expiresAt {
		return "", false
	}

	if adminID != currentAdminID {
		return "", false
	}

	payload := fmt.Sprintf("%s:%s:%s", userID, expiresStr, adminID)
	key, err := getImpersonationSignKey()
	if err != nil {
		log.Printf("[Security CRITICAL] VerifyImpersonationToken failed to retrieve key: %v", err)
		return "", false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return userID, true
	}
	return "", false
}

func SignImpersonationCSRFToken(adminID string) string {
	expiresAt := time.Now().Add(15 * time.Minute).Unix()
	payload := fmt.Sprintf("csrf:%d:%s", expiresAt, adminID)

	key, err := getImpersonationSignKey()
	if err != nil {
		log.Printf("[Security CRITICAL] SignImpersonationCSRFToken failed to retrieve key: %v", err)
		return ""
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("csrf.%d.%s.%s", expiresAt, adminID, signature)
}

func VerifyImpersonationCSRFToken(token, currentAdminID string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 4 || parts[0] != "csrf" {
		return false
	}
	expiresStr := parts[1]
	adminID := parts[2]
	signature := parts[3]

	expiresAt, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil || time.Now().Unix() > expiresAt {
		return false
	}

	if adminID != currentAdminID {
		return false
	}

	payload := fmt.Sprintf("csrf:%s:%s", expiresStr, adminID)
	key, err := getImpersonationSignKey()
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

func ClearImpersonationCookies(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("impersonate_user_id", "", -1, "/", "", true, true)
	c.SetCookie("impersonate_csrf_token", "", -1, "/", "", true, false)
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
	clerkToDBIDCache    = cache.New(localRolePermsTTL, 10*time.Minute)
	userContextSF       singleflight.Group
)

type userAuthContext struct {
	Role        string
	Permissions []string
	Status      string
}

const rolePermsInvalidateChannel = "cache:invalidate:user_role_perms"

// InvalidateRolePermsCache evicts a user's cached role/permissions locally and broadcasts it globally via Redis.
func InvalidateRolePermsCache(userID string) {
	localRolePermsCache.Delete(userID)

	if db.Redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := db.Redis.Publish(ctx, rolePermsInvalidateChannel, userID).Err(); err != nil {
			log.Printf("[Cache Incoherence] Failed to publish invalidation for user %s: %v", userID, err)
		}
	}
}

// StartDistributedCacheInvalidator subscribes to Redis Pub/Sub to listen for role/permissions invalidation events.
func StartDistributedCacheInvalidator() {
	if db.Redis == nil {
		log.Println("[Cache Incoherence] Redis not available, distributed cache invalidator listener skipped")
		return
	}

	go func() {
		pubsub := db.Redis.Subscribe(context.Background(), rolePermsInvalidateChannel)
		defer pubsub.Close()

		ch := pubsub.Channel()
		log.Printf("[Cache Incoherence] Subscribed to distributed cache invalidation channel: %s", rolePermsInvalidateChannel)

		for msg := range ch {
			userID := msg.Payload
			if userID != "" {
				localRolePermsCache.Delete(userID)
			}
		}
	}()
}

type ContextKey string

const (
	UserContextKey    ContextKey = "user_id"
	RoleContextKey    ContextKey = "user_role"
	EmailContextKey   ContextKey = "user_email"
	ClerkIDContextKey ContextKey = "clerk_id"
)

func getConfig() *config.Config {
	configOnce.Do(func() {
		cachedConfig = config.Load()
	})
	return cachedConfig
}

func authMiddleware(optional bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := extractToken(c)
		if tokenString == "" {
			if optional {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		tokenService := &services.TokenService{}
		claims, err := tokenService.ValidateToken(tokenString)
		if err != nil || claims.Subject == "" {
			if optional {
				c.Next()
				return
			}
			log.Printf("[Auth Middleware] Token validation failed: %v", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		if claims.JTI != "" && tokenService.IsJTIBlacklisted(claims.JTI) {
			log.Printf("[Auth Middleware] Token JTI %s is blacklisted", claims.JTI)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Session has been revoked or logged out"})
			return
		}

		clerkID := claims.Subject
		dbUserID := resolveUserIDFromClerkID(clerkID)

		// Set both "userId" and "user_id" keys to avoid breaking existing handlers.
		c.Set("userId", dbUserID)
		c.Set("user_id", dbUserID)
		c.Set("clerkUserId", clerkID)
		c.Set("jti", claims.JTI)
		if claims.ExpiresAt != nil {
			c.Set("accessTokenExpiresAt", claims.ExpiresAt.Time.UnixMilli())
		}
		if claims.Email != "" {
			c.Set("user_email", claims.Email)
		}

		hydrateUserContext(c, dbUserID, claims.Role)
		if c.IsAborted() {
			return
		}
		if resolvedUserID := resolveUserIDFromClerkID(clerkID); resolvedUserID != "" && resolvedUserID != dbUserID {
			dbUserID = resolvedUserID
			c.Set("userId", dbUserID)
			c.Set("user_id", dbUserID)
		}
		processImpersonation(c, dbUserID)
		if c.IsAborted() {
			return
		}

		// Propagate to standard context for gRPC/Connect RPC handlers
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, UserContextKey, dbUserID)
		if finalRole, exists := c.Get("role"); exists {
			ctx = context.WithValue(ctx, RoleContextKey, finalRole)
		}
		if claims.Email != "" {
			ctx = context.WithValue(ctx, EmailContextKey, claims.Email)
		}
		if clerkID != "" {
			ctx = context.WithValue(ctx, ClerkIDContextKey, clerkID)
		}
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

func Auth() gin.HandlerFunc {
	return authMiddleware(false)
}

func OptionalAuth() gin.HandlerFunc {
	return authMiddleware(true)
}

// Helper to extract JWT token from Authorization header or auth cookies.
func extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		token := strings.TrimSpace(authHeader[len("Bearer "):])
		if token != "" {
			return token
		}
	}

	// Try cookies as fallback for SSR/API proxy support.
	// Clerk stores the active session JWT in "__session".
	if cookieToken, err := c.Cookie("__session"); err == nil {
		if token := strings.TrimSpace(cookieToken); token != "" {
			return token
		}
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

// resolveUserIDFromClerkID checks local in-memory cache and then DB for clerkID mapping.
// Returns resolved dbUserID. Fallback is deterministically generated from clerkID.
func resolveUserIDFromClerkID(clerkID string) string {
	if clerkID == "" {
		return ""
	}

	// 1. Check local mapping cache
	if val, found := clerkToDBIDCache.Get(clerkID); found {
		return val.(string)
	}

	if db.DB == nil {
		return services.ClerkIDToUUID(clerkID)
	}

	// 2. Query DB
	var user models.User
	if err := db.DB.Select("id").Where("clerk_id = ?", clerkID).Take(&user).Error; err == nil && user.ID != "" {
		clerkToDBIDCache.Set(clerkID, user.ID, cache.DefaultExpiration)
		return user.ID
	}

	// 3. Fallback to deterministic UUID v5
	return services.ClerkIDToUUID(clerkID)
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

	parts := strings.SplitN(cachedVal, "|", 3)
	if len(parts) < 2 {
		return nil, false
	}

	permsVal := strings.Split(parts[1], ",")
	if len(permsVal) == 1 && permsVal[0] == "" {
		permsVal = []string{}
	}

	statusVal := "ACTIVE"
	if len(parts) == 3 {
		statusVal = parts[2]
	}

	return &userAuthContext{Role: parts[0], Permissions: permsVal, Status: statusVal}, true
}

// storeInLocalCache populates the in-memory cache for the given user.
func storeInLocalCache(userID string, ctx *userAuthContext) {
	localRolePermsCache.Set(userID, ctx, cache.DefaultExpiration)
}

// fetchDatabaseRolePerms retrieves user role/permissions from the database.
// Caches the result in Redis (async) and local in-memory cache.
// Returns nil if the user is not found in the database and cannot be provisioned.
func fetchDatabaseRolePerms(userID, clerkID, cacheKey string) *userAuthContext {
	var user models.User
	if err := db.DB.
		Select("id", "role", "permissions", "status").
		Where("id = ?", userID).
		Take(&user).Error; err != nil {

		if clerkID != "" && strings.HasPrefix(clerkID, "user_") {
			// ProvisionUserFromClerk uses singleflight.Group internally,
			// so concurrent requests for the same clerkID share a single HTTP call.
			provisionedUser, errProvision := services.ProvisionUserFromClerk(clerkID)
			if errProvision != nil {
				log.Printf("[Auth Middleware] Provisioning failed for Clerk ID %s: %v", clerkID, errProvision)
				return nil
			}
			if provisionedUser != nil && provisionedUser.ID != "" && provisionedUser.ID != userID {
				userID = provisionedUser.ID
				cacheKey = fmt.Sprintf("user_role_perms:%s", userID)
				clerkToDBIDCache.Set(clerkID, userID, cache.DefaultExpiration)
			}

			// Retry up to 3 times with a short delay to handle any eventual-consistency delay
			// (e.g. if a goroutine in a different request just created the user).
			for i := 0; i < 3; i++ {
				if errRetry := db.DB.
					Select("id", "role", "permissions", "status").
					Where("id = ?", userID).
					Take(&user).Error; errRetry == nil {
					break // user found, exit retry loop
				}
				time.Sleep(50 * time.Millisecond)
			}
			if user.ID == "" {
				log.Printf("[Auth Middleware] User still missing after provisioning + retries (user_id=%s, clerk_id=%s)", userID, clerkID)
				return nil
			}
		} else {
			return nil
		}
	}

	roleVal := string(user.Role)
	permsVal := []string(user.Permissions)
	if permsVal == nil {
		permsVal = []string{}
	}
	statusVal := string(user.Status)
	if statusVal == "" {
		statusVal = "ACTIVE"
	}

	// Cache in Redis asynchronously
	if db.Redis != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			permsStr := strings.Join(permsVal, ",")
			db.Redis.Set(ctx, cacheKey, roleVal+"|"+permsStr+"|"+statusVal, rolePermsRedisTTL)
		}()
	}

	authCtx := &userAuthContext{Role: roleVal, Permissions: permsVal, Status: statusVal}
	storeInLocalCache(userID, authCtx)
	return authCtx
}

// buildSingleFlightCallback creates the function used inside singleflight.Do
// to fetch user role/permissions from cache layers and finally database.
func buildSingleFlightCallback(userID, clerkID, fallbackRole string) func() (interface{}, error) {
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
		if dbCtx := fetchDatabaseRolePerms(userID, clerkID, cacheKey); dbCtx != nil {
			return dbCtx, nil
		}

		return &userAuthContext{Role: strings.ToUpper(fallbackRole), Permissions: []string{}, Status: "ACTIVE"}, nil
	}
}

// Helper to fetch and set user role/permissions in context from database or fallback
func hydrateUserContext(c *gin.Context, userID, fallbackRole string) {
	if db.DB == nil {
		log.Printf("WARN: Database connection is nil in hydrateUserContext for user %s", userID)
		fallbackUpper := strings.ToUpper(fallbackRole)
		c.Set("role", fallbackUpper)
		c.Set("user_role", fallbackUpper)
		c.Set("permissions", []string{})
		c.Set("is_admin", fallbackUpper == "ADMIN" || fallbackUpper == "SUPER_ADMIN")
		return
	}

	// 1. Try local in-memory cache first to bypass Redis cloud network latency
	if cached, ok := fetchCachedRolePerms(userID); ok {
		if cached.Status == string(models.StatusSuspended) || cached.Status == string(models.StatusInactive) {
			log.Printf("[Auth Middleware] User %s is suspended or inactive (status: %s)", userID, cached.Status)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Account is inactive or suspended"})
			return
		}
		c.Set("role", cached.Role)
		c.Set("user_role", cached.Role)
		c.Set("permissions", cached.Permissions)
		c.Set("is_admin", cached.Role == "ADMIN" || cached.Role == "SUPER_ADMIN")
		return
	}

	clerkIDVal, _ := c.Get("clerkUserId")
	clerkID, _ := clerkIDVal.(string)

	// 2. Use singleflight to collapse concurrent calls for the same user
	res, err, _ := userContextSF.Do(userID, buildSingleFlightCallback(userID, clerkID, fallbackRole))

	if err == nil {
		authCtx := res.(*userAuthContext)
		if authCtx.Status == string(models.StatusSuspended) || authCtx.Status == string(models.StatusInactive) {
			log.Printf("[Auth Middleware] User %s is suspended or inactive (status: %s)", userID, authCtx.Status)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Account is inactive or suspended"})
			return
		}
		c.Set("role", authCtx.Role)
		c.Set("user_role", authCtx.Role)
		c.Set("permissions", authCtx.Permissions)
		c.Set("is_admin", authCtx.Role == "ADMIN" || authCtx.Role == "SUPER_ADMIN")
	} else {
		fallbackUpper := strings.ToUpper(fallbackRole)
		c.Set("role", fallbackUpper)
		c.Set("user_role", fallbackUpper)
		c.Set("permissions", []string{})
		c.Set("is_admin", fallbackUpper == "ADMIN" || fallbackUpper == "SUPER_ADMIN")
	}
}

// Helper to handle admin impersonation logic if applicable
func processImpersonation(c *gin.Context, adminID string) {
	currentRole, _ := c.Get("role")
	currentRoleStr, _ := currentRole.(string)

	if currentRoleStr != "ADMIN" && currentRoleStr != "SUPER_ADMIN" {
		return
	}

	// Only enforce impersonation CSRF when actually impersonating (cookie is set).
	// Normal admin API requests should not be blocked by this check.
	impersonatedCookie, err := c.Cookie("impersonate_user_id")
	if err != nil || impersonatedCookie == "" {
		return
	}

	csrfToken := c.GetHeader("X-CSRF-Impersonation-Token")
	if csrfToken == "" {
		// Clear cookies to tear down invalid state and allow recovery
		ClearImpersonationCookies(c)
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Missing impersonation CSRF token"})
		return
	}

	if !VerifyImpersonationCSRFToken(csrfToken, adminID) {
		log.Printf("[Security Alert] Admin %s provided invalid impersonation CSRF token", adminID)
		// Clear cookies to tear down invalid state and allow recovery
		ClearImpersonationCookies(c)
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Invalid impersonation CSRF token"})
		return
	}

	impersonatedID, ok := VerifyImpersonationToken(impersonatedCookie, adminID)
	if !ok {
		log.Printf("Security Warning: Admin %s attempted to impersonate with an invalid or tampered token: %s", adminID, impersonatedCookie)
		// Clear cookies to tear down invalid state and allow recovery
		ClearImpersonationCookies(c)
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
		res, err, _ := userContextSF.Do(impersonatedID, buildSingleFlightCallback(impersonatedID, "", ""))
		if err == nil {
			authCtx = res.(*userAuthContext)
		}
	}

	if authCtx != nil && authCtx.Role != "" {
		// Prevent privilege escalation: An ADMIN/SUPER_ADMIN cannot impersonate another user of equal or higher rank
		roleHierarchy := map[string]int{
			"STUDENT":     1,
			"PREMIUM":     2, // Premium sits between Student and Teacher
			"TEACHER":     3,
			"MODERATOR":   4,
			"ADMIN":       5,
			"SUPER_ADMIN": 6,
		}

		currRank, validParent := roleHierarchy[currentRoleStr]
		targetRank, validChild := roleHierarchy[authCtx.Role]

		if !validParent || !validChild {
			log.Printf("[Security Alert] Context conversion evaluation failed for rank assessment: %s to %s", currentRoleStr, authCtx.Role)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Malformed role structure encountered during evaluation"})
			return
		}

		if currRank <= targetRank {
			log.Printf("[Security Infraction] Privilege escalation block: Operator (%s) attempted control over (%s)", adminID, impersonatedID)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Impersonating users of equal or higher administrative rank is explicitly prohibited"})
			return
		}

		c.Set("originalAdminId", adminID)
		// Set both "userId" and "user_id" keys to avoid breaking existing handlers.
		// Note: "userId" is widely used throughout legacy handlers (>100 references),
		// while "user_id" is used in standard middlewares and newer handlers.
		c.Set("userId", impersonatedID)
		c.Set("user_id", impersonatedID)
		c.Set("role", authCtx.Role)
		c.Set("user_role", authCtx.Role)
		c.Set("isImpersonating", true)
		c.Set("is_admin", authCtx.Role == "ADMIN" || authCtx.Role == "SUPER_ADMIN")
		c.Set("permissions", authCtx.Permissions)
	}
}

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		rs, _ := role.(string)
		if rs != "ADMIN" && rs != "SUPER_ADMIN" {
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
		if rs != "ADMIN" && rs != "SUPER_ADMIN" && rs != "MODERATOR" {
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
		if rs != "ADMIN" && rs != "SUPER_ADMIN" && rs != "MODERATOR" {
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
			"https://tolo.app",
			"https://www.tolo.app",
			"https://tolo-blond.vercel.app",
			"https://admin-lime-omega-38.vercel.app",
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

	// Security: Do NOT use a wildcard for *.vercel.app — any project on Vercel
	// could then make authenticated requests to this API.
	// Add specific Vercel preview URLs to the allowedOrigins slice above instead.

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
		if ip.IsLoopback() {
			return true
		}
	}

	return false
}

// Helper to set CORS response headers
func setCorsHeaders(c *gin.Context, origin string, isAllowed bool) {
	if isAllowed && origin != "" {
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, Connect-Protocol-Version, Connect-Timeout-Ms, Connect-Content-Encoding, X-Grpc-Web, X-User-Agent, Idempotency-Key")
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
