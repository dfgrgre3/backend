package middleware

import (
	"os"
	"strings"

	"thanawy-backend/internal/config"

	"github.com/gin-gonic/gin"
)

// CORS handles Cross-Origin Resource Sharing headers.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Load()

		allowedOrigins := map[string]bool{}
		corsOrigins := os.Getenv("CORS_ORIGINS")
		for _, o := range strings.Split(corsOrigins, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				allowedOrigins[o] = true
			}
		}

		origin := c.Request.Header.Get("Origin")
		allowOrigin := ""
		if origin != "" {
			if allowedOrigins[origin] {
				allowOrigin = origin
			} else if cfg.Environment == "development" && len(allowedOrigins) == 0 && isLocalhostOrigin(origin) {
				allowOrigin = origin
			}
		}

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

// isLocalhostOrigin reports whether the given origin is a localhost URL.
func isLocalhostOrigin(origin string) bool {
	return strings.HasPrefix(origin, "http://localhost:") ||
		strings.HasPrefix(origin, "http://127.0.0.1:") ||
		strings.HasPrefix(origin, "https://localhost:") ||
		strings.HasPrefix(origin, "https://127.0.0.1:")
}
