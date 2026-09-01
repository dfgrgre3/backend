package api

import (
	admindelivery "thanawy-backend/internal/infrastructure/api/handlers/admin"
	handlers "thanawy-backend/internal/infrastructure/api/handlers/protected"

	"github.com/gin-gonic/gin"
)

// registerAdminSecurityRoutes registers Session & Security Management and
// IP Whitelist (sensitive) routes.
func registerAdminSecurityRoutes(admin, sensitive *gin.RouterGroup) {
	// Session & Security Management
	admin.GET("/security/sessions", handlers.GetActiveSessions)
	admin.GET("/security/sessions/stats", handlers.GetSessionStats)
	admin.POST("/security/sessions/:id/revoke", handlers.RevokeSession)
	admin.POST("/security/sessions/revoke-others", handlers.RevokeOtherSessions)
	admin.POST("/security/sessions/user/:userId/revoke-all", handlers.RevokeUserSessions)
	admin.POST("/security/sessions/:id/suspend", admindelivery.SuspendSession)
	admin.GET("/security/sessions/activity", admindelivery.GetSessionActivity)
	admin.GET("/security/logs", handlers.GetAdminSecurityLogs)
	admin.GET("/security/logs/users/:id", handlers.GetSecurityLogsForUser)
	admin.GET("/security/fingerprints", handlers.GetDeviceFingerprints)
	admin.POST("/security/fingerprints/block", handlers.BlockDeviceFingerprint)
	admin.POST("/security/fingerprints/:id/unblock", handlers.UnblockDeviceFingerprint)
	admin.GET("/security/roles", handlers.GetRolePermissions)
	admin.GET("/activity-log", handlers.AdminListActivityLog)
	admin.GET("/activity-log/options", handlers.AdminListActivityLogOptions)

	// IP Whitelist (admin-only)
	sensitive.GET("/security/ip-whitelist", handlers.GetIPWhitelist)
	sensitive.POST("/security/ip-whitelist", handlers.AddIPToWhitelist)
	sensitive.GET("/security/ip-whitelist/settings", handlers.GetIPWhitelistSettings)
	sensitive.POST("/security/ip-whitelist/settings", handlers.UpdateIPWhitelistSettings)
	sensitive.GET("/security/ip-whitelist/blocked", handlers.GetBlockedAttempts)
	sensitive.POST("/security/ip-whitelist/bulk", handlers.BulkAddIPToWhitelist)
	sensitive.GET("/security/ip-whitelist/check", handlers.CheckIPWhitelist)
	sensitive.PATCH("/security/ip-whitelist/:id", handlers.UpdateIPWhitelistEntry)
	sensitive.DELETE("/security/ip-whitelist/:id", handlers.RemoveIPFromWhitelist)
}
