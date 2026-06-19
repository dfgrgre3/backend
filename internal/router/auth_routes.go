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
		// Public auth endpoints
		auth.POST("/register", handlers.Register)
		auth.POST("/login", handlers.Login)
		auth.POST("/verify-2fa", handlers.Verify2FA)
		auth.POST("/magic-link", handlers.RequestMagicLink)
		auth.GET("/magic-link/verify", handlers.VerifyMagicLink)
		auth.POST("/forgot-password", handlers.ForgotPassword)
		auth.POST("/reset-password", handlers.ResetPassword)
		auth.GET("/verify-email", handlers.VerifyEmail)
		auth.POST("/verify-email/resend", handlers.ResendVerification)
		auth.POST("/refresh", handlers.RefreshToken)
		auth.POST("/logout", handlers.Logout)

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
			// Devices: list all active sessions with device info for the current user
			auth.GET("/devices", handlers.GetUserDevices)
		}
	}
}
