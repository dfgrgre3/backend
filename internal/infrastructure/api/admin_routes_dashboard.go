package api

import (
	admindelivery "thanawy-backend/internal/infrastructure/api/handlers/admin"
	handlers "thanawy-backend/internal/infrastructure/api/handlers/protected"

	"github.com/gin-gonic/gin"
)

// registerAdminDashboardRoutes registers the dashboard, analytics, AI, and
// impersonation (sensitive) endpoints.
func registerAdminDashboardRoutes(admin, sensitive *gin.RouterGroup) {
	// Dashboard
	// The aggregate endpoint stays as the compatibility surface for the
	// current admin UI; the routes below are the granular, per-widget API.
	admin.GET("/dashboard", admindelivery.GetAdminDashboard)
	admin.GET("/dashboard/filters", admindelivery.GetDashboardFiltersMetadata)
	admin.GET("/dashboard/summary", admindelivery.GetDashboardSummary)
	admin.GET("/dashboard/time-series", admindelivery.GetDashboardTimeSeries)
	admin.GET("/dashboard/pending-actions", admindelivery.GetDashboardPendingActions)
	admin.GET("/dashboard/alerts", admindelivery.GetDashboardAlerts)
	admin.POST("/dashboard/alerts/:alertId/acknowledge", admindelivery.AcknowledgeDashboardAlert)
	admin.GET("/dashboard/recent-activities", admindelivery.GetDashboardRecentActivities)
	admin.GET("/dashboard/top-courses", admindelivery.GetDashboardTopCourses)
	admin.GET("/dashboard/system-health", admindelivery.GetDashboardSystemHealth)
	admin.GET("/dashboard/system-health/:service/history", admindelivery.GetDashboardServiceHealthHistory)
	admin.POST("/dashboard/refresh", admindelivery.RefreshDashboardData)
	admin.POST("/dashboard/export", admindelivery.ExportDashboardReport)
	admin.GET("/dashboard/export/:exportJobId", admindelivery.GetDashboardExportStatus)
	admin.POST("/dashboard/saved-filters", admindelivery.SaveDashboardFilter)
	admin.DELETE("/dashboard/saved-filters/:filterId", admindelivery.DeleteDashboardSavedFilter)
	admin.GET("/live", admindelivery.GetAdminLive)
	admin.GET("/live-sessions", admindelivery.AdminListLiveSessions)
	admin.POST("/live-sessions", admindelivery.AdminCreateLiveSession)
	admin.GET("/analytics", handlers.GetAdminAnalytics)
	admin.GET("/health/detailed", handlers.GetDetailedAdminHealth)
	admin.GET("/health/export", handlers.ExportDetailedAdminHealth)
	// Analytics sub-resources used by the admin analytics workspace.
	admin.GET("/analytics/revenue", handlers.GetAdminRevenue)
	admin.GET("/analytics/journeys", handlers.GetUserJourneys)
	admin.GET("/analytics/metrics", handlers.GetActivityMetrics)
	// Journey/conversion tracking writes (used by the admin analytics
	// integration hook when a journey session ends or a goal converts).
	admin.POST("/analytics/journey", handlers.TrackUserJourney)
	admin.POST("/analytics/conversion", handlers.TrackConversionEvent)
	admin.GET("/infrastructure/stats", handlers.GetAdminInfrastructureStats)
	admin.GET(adminAnnouncementsRoute, handlers.GetAdminAnnouncements)
	admin.POST(adminAnnouncementsRoute, handlers.CreateAdminAnnouncement)
	admin.PATCH(adminAnnouncementsRoute, handlers.UpdateAdminAnnouncement)
	admin.DELETE(adminAnnouncementsRoute, handlers.DeleteAdminAnnouncement)
	admin.GET("/reports/overview", admindelivery.GetAdminReportsOverview)
	admin.GET("/reports/users", admindelivery.GetAdminReportsUsers)
	admin.GET("/reports/books", admindelivery.GetAdminReportsBooks)
	// Audit logs are read-only. Audit entries themselves are created by the
	// middleware above, never from a browser supplied payload.
	admin.GET("/audit-logs", handlers.AdminGetAuditLogs)

	// AI
	admin.GET("/ai", handlers.AdminAIGet)
	admin.POST("/ai", handlers.AdminAIPost)
	admin.GET("/ai/analyze", admindelivery.AdminAIAnalyze)
	admin.POST("/ai/analyze", admindelivery.AdminAIAnalyze)

	// Impersonation (admin-only)
	sensitive.POST("/reset-circuit-breaker", handlers.AdminResetCircuitBreaker)
	sensitive.POST("/impersonate", handlers.ImpersonateUser)
	sensitive.DELETE("/impersonate", handlers.DeleteImpersonation)
}
