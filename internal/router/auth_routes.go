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
		// Local auth sub-group - disabled in production
		localAuth := auth.Group("")
		localAuth.Use(middleware.DisableLocalAuthInProduction())
		{
			localAuth.POST("/register", middleware.AuthRateLimiter(), handlers.Register)
			localAuth.POST("/login", middleware.LoginRateLimiter(), handlers.Login)
			localAuth.POST("/verify-2fa", middleware.LoginRateLimiter(), handlers.Verify2FA)
			localAuth.POST("/magic-link", middleware.AuthRateLimiter(), handlers.RequestMagicLink)
			localAuth.GET("/magic-link/verify", middleware.AuthRateLimiter(), handlers.VerifyMagicLink)
			localAuth.POST("/forgot-password", middleware.AuthRateLimiter(), handlers.ForgotPassword)
			localAuth.POST("/reset-password", middleware.AuthRateLimiter(), handlers.ResetPassword)
			localAuth.GET("/verify-email", middleware.AuthRateLimiter(), handlers.VerifyEmail)
			localAuth.POST("/verify-email/resend", middleware.AuthRateLimiter(), handlers.ResendVerification)
			localAuth.POST("/refresh", handlers.RefreshToken)
			localAuth.POST("/logout", handlers.Logout)
			localAuth.GET("/guest", handlers.GetGuestUser)
		}

		// Protected auth routes (uses Clerk RS256 JWT tokens via middleware)
		protectedAuth := auth.Group("")
		protectedAuth.Use(middleware.Auth())
		{
			const sessionsPath = "/sessions"
			protectedAuth.GET("/me", handlers.GetProfile)
			protectedAuth.GET(sessionsPath, handlers.GetAuthSessions)
			protectedAuth.DELETE(sessionsPath, handlers.DeleteAuthSession)
			protectedAuth.PATCH(sessionsPath, handlers.UpdateAuthSession)
			protectedAuth.GET("/security-logs", handlers.GetSecurityLogs)
			protectedAuth.GET("/2fa/status", handlers.GetUser2FAStatus)
			protectedAuth.GET("/2fa/setup", handlers.InitiateUser2FASetup)
			protectedAuth.POST("/2fa/enable", handlers.EnableUser2FA)
			protectedAuth.POST("/2fa/disable", handlers.DisableUser2FA)
			protectedAuth.POST("/verify-phone/send", handlers.SendPhoneVerification)
			protectedAuth.POST("/verify-phone/verify", handlers.VerifyPhoneVerification)
			// Devices: list all active sessions with device info for the current user
			protectedAuth.GET("/devices", handlers.GetUserDevices)
		}
	}
}
