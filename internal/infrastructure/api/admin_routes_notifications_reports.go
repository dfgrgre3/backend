package api

import (
	handlers "thanawy-backend/internal/infrastructure/api/handlers/protected"

	"github.com/gin-gonic/gin"
)

// registerAdminNotificationReportRoutes registers notification broadcast,
// custom reports, scheduler and search routes.
func registerAdminNotificationReportRoutes(admin *gin.RouterGroup) {
	// Notifications Broadcast
	admin.POST("/notifications/broadcast", handlers.SendNotificationBroadcast)
	admin.POST("/notifications/schedule", handlers.ScheduleNotificationBroadcast)
	admin.POST("/notifications/broadcast/:id/cancel", handlers.CancelScheduledBroadcast)
	admin.POST("/notifications/broadcast/:id/retry", handlers.RetryFailedNotifications)
	admin.GET("/broadcasts", handlers.GetBroadcasts)
	admin.GET("/notifications/stats", handlers.GetNotificationStats)
	admin.POST("/notifications/push", handlers.SendPushNotification)

	// Reports
	admin.GET("/reports", handlers.GetCustomReports)
	admin.POST("/reports", handlers.CreateCustomReport)
	admin.GET(adminReportsIDRoute, handlers.GetCustomReport)
	admin.PATCH(adminReportsIDRoute, handlers.UpdateCustomReport)
	admin.DELETE(adminReportsIDRoute, handlers.DeleteCustomReport)
	admin.POST("/reports/:id/execute", handlers.ExecuteCustomReport)
	admin.GET("/reports/:id/export", handlers.ExportCustomReport)
	admin.POST("/reports/:id/schedule", handlers.ScheduleCustomReport)

	// Scheduler
	admin.GET("/scheduler", handlers.GetScheduledItems)
	admin.POST("/scheduler", handlers.CreateScheduledItem)
	admin.POST("/scheduler/:id/cancel", handlers.CancelScheduledItem)
	admin.POST("/scheduler/:id/retry", handlers.RetryScheduledItem)
	admin.POST("/scheduler/:id/execute", handlers.ExecuteScheduledItemNow)
	admin.DELETE("/scheduler/:id", handlers.DeleteScheduledItem)
	admin.GET("/scheduler/stats", handlers.GetSchedulerStats)

	// Search
	admin.GET("/search/content", handlers.SearchContent)
}
