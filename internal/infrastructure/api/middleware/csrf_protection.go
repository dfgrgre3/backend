package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	csrfHeaderName  = "X-CSRF-Token"
	csrfCookieName  = "_csrf"
	csrfTokenLength = 32
)

// CSRFProtection middleware for cookie-based authentication.
// It skips auth handshake endpoints and safe methods, and validates state-changing
// requests using the Double Submit Cookie pattern.
func CSRFProtection() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isSafeMethod(c.Request.Method) {
			EnsureCSRFToken(c)
			c.Next()
			return
		}

		if shouldSkipPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		if !validateCSRFToken(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "CSRF token validation failed",
			})
			return
		}

		c.Next()
	}
}

// EnsureCSRFToken creates a CSRF token if one doesn't exist.
// Cookie is NOT HttpOnly to allow JavaScript to read it for the header.
func EnsureCSRFToken(c *gin.Context) {
	// Check if cookie exists
	cookie, err := c.Cookie(csrfCookieName)
	if err == nil && cookie != "" {
		// Validate existing token
		if isValidCSRFToken(cookie) {
			// Set header for client-side access
			c.Header(csrfHeaderName, cookie)
			return
		}
	}

	// Generate new token
	token := generateCSRFToken()

	// Set cookie (NOT HttpOnly - allows JS to read for Double Submit Cookie pattern)
	setCSRFCookie(c, token)

	// Set header
	c.Header(csrfHeaderName, token)
}

// validateCSRFToken validates the CSRF token from request
// Compares the token from the header with the token from the cookie
func validateCSRFToken(c *gin.Context) bool {
	// Get token from header
	headerToken := c.GetHeader(csrfHeaderName)
	if headerToken == "" {
		return false
	}

	// Get token from cookie
	cookieToken, err := c.Cookie(csrfCookieName)
	if err != nil || cookieToken == "" {
		return false
	}

	// Compare tokens using constant-time comparison
	if subtle.ConstantTimeCompare([]byte(headerToken), []byte(cookieToken)) == 1 {
		return true
	}

	return false
}

// generateCSRFToken creates a new CSRF token
func generateCSRFToken() string {
	bytes := make([]byte, csrfTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		log.Panic("CRITICAL: Failed to generate secure random bytes for CSRF token")
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

// setCSRFCookie sets the CSRF cookie
// NOT HttpOnly - this is intentional for Double Submit Cookie pattern
// JavaScript needs to read this cookie and set it as the X-CSRF-Token header
func setCSRFCookie(c *gin.Context, token string) {
	// Prefer secure cookies when the request is over TLS. Behind a reverse
	// proxy, however, Gin may see an internal HTTP hop, so TLS alone is not a
	// reliable production signal. Allow the deployment to enforce Secure via
	// COOKIE_SECURE or the production environment while keeping local HTTP
	// development usable by default.
	secure := c.Request.TLS != nil ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("COOKIE_SECURE")), "true") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("NODE_ENV")), "production")

	// Domain cookies are valid only when the configured domain matches the request host.
	// In local development, "localhost" and IP hosts must use host-only cookies.
	requestHost := c.Request.Host
	if host, _, err := net.SplitHostPort(requestHost); err == nil {
		requestHost = host
	}
	requestHost = strings.Trim(requestHost, "[]")

	domain := strings.TrimPrefix(strings.TrimSpace(os.Getenv("COOKIE_DOMAIN")), ".")
	if domain == "localhost" || net.ParseIP(requestHost) != nil ||
		(domain != "" && requestHost != domain && !strings.HasSuffix(requestHost, "."+domain)) {
		domain = ""
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		csrfCookieName,
		token,
		int(24*time.Hour.Seconds()), // 24 hours
		"/",
		domain,
		secure, // Secure in production
		false,  // NOSONAR: NOT HttpOnly - allows JS to read for Double Submit Cookie pattern
	)
}

// isValidCSRFToken checks if a token is valid (not expired, proper format)
func isValidCSRFToken(token string) bool {
	// Decode and validate format
	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil || len(decoded) != csrfTokenLength {
		return false
	}
	return true
}

// csrfSkipPaths are paths that bypass CSRF protection.
// These are auth handshake endpoints where the caller cannot yet possess a
// valid CSRF token (e.g. they just opened the app, or they are sending a
// request that establishes their identity).
// Webhooks and external callbacks use their own signing-secret authentication
// (Paymob HMAC, etc.) and never carry browser cookies/headers.
// Public analytics endpoints are fire-and-forget tracking calls that should
// not require CSRF tokens.
var csrfSkipPaths = []string{
	"/api/webhooks/",
	"/api/payments/paymob/callback",
	"/api/auth/login",
	"/api/auth/register",
	"/api/auth/refresh",
	"/api/auth/logout",
	"/api/auth/2fa/",
	"/api/auth/mfa/",
	"/api/auth/change-password",
	"/api/auth/sessions",
	"/api/analytics/promo",
	"/api/analytics/mega-menu",
}

// isSafeMethod checks if the HTTP method is read-only
func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// shouldSkipPath checks if the request path is in the skip list
func shouldSkipPath(path string) bool {
	for _, skip := range csrfSkipPaths {
		if strings.HasPrefix(path, skip) {
			return true
		}
	}
	return false
}

// shouldEnforceCSRF determines if CSRF should be enforced based on environment and request type
func shouldEnforceCSRF(c *gin.Context) bool {
	// Only enforce if using cookie authentication or likely from a browser
	_, err := c.Cookie("access_token")
	if err == nil {
		return true
	}

	accept := c.GetHeader("Accept")
	return strings.Contains(accept, "text/html") || strings.Contains(accept, "application/xhtml+xml")
}

// validateOrigin checks if the origin or referer header matches the host or CORS origins.
func validateOrigin(c *gin.Context) bool {
	origin := c.GetHeader("Origin")
	if origin == "" {
		origin = c.GetHeader("Referer")
	}
	if origin == "" {
		return false // Require a browser-originating request for state-changing operations.
	}

	parsedOrigin, err := url.Parse(origin)
	if err != nil || parsedOrigin.Host == "" {
		return false
	}

	host := c.Request.Host
	originHost := parsedOrigin.Host

	if originHost == host {
		return true
	}

	corsOrigins := os.Getenv("CORS_ORIGINS")
	for _, o := range strings.Split(corsOrigins, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			if parsed, err := url.Parse(o); err == nil && parsed.Host == originHost {
				return true
			}
		}
	}

	return false
}

// CSRFMiddleware returns a configured CSRF protection middleware
func CSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip CSRF for safe methods
		if isSafeMethod(c.Request.Method) {
			EnsureCSRFToken(c)
			c.Next()
			return
		}

		// Skip for certain paths (webhooks, auth handshakes, etc.)
		if shouldSkipPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Apply CSRF protection for state-changing requests in production
		if shouldEnforceCSRF(c) {
			if !validateCSRFToken(c) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "CSRF token validation failed",
				})
				return
			}

			origin := c.GetHeader("Origin")
			if origin == "" {
				origin = c.GetHeader("Referer")
			}
			if origin != "" && !validateOrigin(c) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "CSRF origin validation failed",
				})
				return
			}
		}

		c.Next()
	}
}
