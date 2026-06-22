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
		}
	}
}
