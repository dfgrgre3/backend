package api

import (
	admindelivery "thanawy-backend/internal/infrastructure/api/handlers/admin"
	handlers "thanawy-backend/internal/infrastructure/api/handlers/protected"

	"github.com/gin-gonic/gin"
)

// registerAdminSupportBackupRoutes registers Support Ticket and Backup routes.
func registerAdminSupportBackupRoutes(admin, sensitive *gin.RouterGroup) {
	// Support Tickets
	admin.GET("/tickets", handlers.GetSupportTickets)
	admin.POST("/tickets", handlers.CreateSupportTicket)
	admin.GET("/tickets/stats", handlers.GetTicketStats)
	admin.GET("/tickets/:id", handlers.GetSupportTicket)
	admin.POST("/tickets/:id/messages", handlers.SendTicketMessage)
	admin.PATCH("/tickets/:id/status", handlers.UpdateTicketStatus)
	admin.PATCH("/tickets/:id/priority", handlers.UpdateTicketPriority)
	admin.POST("/tickets/:id/assign", handlers.AssignTicket)
	admin.POST("/tickets/:id/close", handlers.CloseTicket)
	admin.PATCH("/tickets/:id/tags", admindelivery.UpdateTicketTags)

	// Backups (admin-only)
	sensitive.GET("/backups", handlers.GetBackups)
	sensitive.POST("/backups", handlers.CreateBackup)
	sensitive.GET("/backups/stats", handlers.GetBackupStats)
	sensitive.GET("/backups/tables", handlers.GetDatabaseTables)
	sensitive.POST(adminBackupsScheduleRoute, handlers.ScheduleBackup)
	sensitive.PUT(adminBackupsScheduleRoute, handlers.ScheduleBackup)
	sensitive.PUT(adminBackupsScheduleIDRoute, handlers.ScheduleBackup)
	sensitive.DELETE(adminBackupsScheduleRoute, admindelivery.DeleteBackupSchedule)
	sensitive.DELETE(adminBackupsScheduleIDRoute, admindelivery.DeleteBackupSchedule)
	sensitive.DELETE("/backups/:id", handlers.DeleteBackup)
	sensitive.GET("/backups/:id/download", handlers.DownloadBackup)
	sensitive.POST("/backups/:id/restore", handlers.RestoreBackup)
	sensitive.POST("/backups/:id/verify", handlers.VerifyBackup)
	sensitive.GET("/backups/:id/progress", handlers.GetBackupProgress)
}
