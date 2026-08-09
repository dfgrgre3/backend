package middleware

import (
	"os"
	"strings"
	"sync"

	"thanawy-backend/internal/infrastructure/config"

	"github.com/gin-gonic/gin"
)

var (
	corsAllowedOrigins     map[string]bool
	corsAllowedOriginsOnce sync.Once
	corsEnvironment        string
)

// initCORSConfig parses CORS_ORIGINS once and caches the result.
func initCORSConfig() {
	corsAllowedOriginsOnce.Do(func() {
		if config.GlobalConfig != nil {
			corsEnvironment = config.GlobalConfig.Environment
		} else {
			corsEnvironment = getEnvFallback("NODE_ENV", "development")
		}
		corsAllowedOrigins = make(map[string]bool)
		corsOrigins := os.Getenv("CORS_ORIGINS")
		for _, o := range strings.Split(corsOrigins, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				corsAllowedOrigins[o] = true
			}
		}
	})
}

func getEnvFallback(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// CORS handles Cross-Origin Resource Sharing headers.
func CORS() gin.HandlerFunc {
	initCORSConfig()
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		allowOrigin := ""
		if origin != "" {
			if corsAllowedOrigins[origin] {
				allowOrigin = origin
			} else if corsEnvironment == "development" && len(corsAllowedOrigins) == 0 && isLocalhostOrigin(origin) {
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
