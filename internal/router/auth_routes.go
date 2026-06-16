package router

import (
	"github.com/gin-gonic/gin"
	"thanawy-backend/internal/api/handlers"
	"thanawy-backend/internal/middleware"
)

// SetupAuthRoutes configures authentication endpoints
func SetupAuthRoutes(router *gin.Engine) {
	auth := router.Group("/api/auth")
	{
		// Guest user creation (no auth required)
		auth.GET("/guest", handlers.GetGuestUser)

		// Protected auth routes (uses Clerk RS256 JWT tokens via middleware)
		auth.Use(middleware.Auth())
		{
			const sessionsPath = "/sessions"
			auth.GET("/me", handlers.GetProfile)
			auth.GET(sessionsPath, handlers.GetAuthSessions)
			auth.DELETE(sessionsPath, handlers.DeleteAuthSession)
			auth.PATCH(sessionsPath, handlers.UpdateAuthSession)
			auth.GET("/security-logs", handlers.GetSecurityLogs)
			auth.GET("/2fa/status", handlers.GetUser2FAStatus)
			auth.GET("/2fa/setup", handlers.InitiateUser2FASetup)
			auth.POST("/2fa/enable", handlers.EnableUser2FA)
			auth.POST("/2fa/disable", handlers.DisableUser2FA)
			auth.POST("/verify-phone/send", handlers.SendPhoneVerification)
			auth.POST("/verify-phone/verify", handlers.VerifyPhoneVerification)
		}
	}
}
