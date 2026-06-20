package middleware

import (
	"net/http"
	"thanawy-backend/internal/config"

	"github.com/gin-gonic/gin"
)

// DisableLocalAuthInProduction blocks local auth endpoints in production.
func DisableLocalAuthInProduction() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Load()
		if cfg.Environment == "production" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "Local authentication is disabled in production",
				"message": "عذراً، نظام تسجيل الدخول المحلي غير نشط في البيئة الإنتاجية. يرجى استخدام بوابة Clerk.",
			})
			return
		}
		c.Next()
	}
}
