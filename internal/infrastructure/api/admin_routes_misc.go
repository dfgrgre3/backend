package api

import (
	admindelivery "thanawy-backend/internal/infrastructure/api/handlers/admin"
	handlers "thanawy-backend/internal/infrastructure/api/handlers/protected"

	"github.com/gin-gonic/gin"
)

// registerAdminMiscRoutes registers database partitions (sensitive),
// marketing/contests, course action, settings, report content, generic
// notifications, interactive questions and lesson transcript routes.
func registerAdminMiscRoutes(admin, sensitive *gin.RouterGroup) {
	// Partitions (admin-only - system-level operation)
	sensitive.GET("/database-partitions", admindelivery.DatabasePartitions)
	// Marketing & Contests
	admin.GET("/marketing", admindelivery.Marketing)
	admin.POST("/marketing", admindelivery.Marketing)
	admin.GET("/contests", admindelivery.Contests)
	admin.POST("/contests", admindelivery.Contests)
	admin.PATCH("/contests/:id", admindelivery.Contests)
	admin.DELETE("/contests/:id", admindelivery.Contests)

	// Course action
	admin.GET(adminCoursesActionRoute, admindelivery.AdminCourseAction)
	admin.POST(adminCoursesActionRoute, admindelivery.AdminCourseAction)
	admin.PATCH(adminCoursesActionRoute, admindelivery.AdminCourseAction)
	admin.PUT(adminCoursesActionRoute, admindelivery.AdminCourseAction)
	admin.GET("/courses/export", handlers.ExportSubjectsCSV)
	admin.GET("/courses/stats", handlers.GetCourseStats)

	// Settings (write = admin-only, read = open to moderators)
	admin.GET(adminSettingsRoute, admindelivery.AdminSettings)
	sensitive.PATCH(adminSettingsRoute, admindelivery.AdminSettings)
	sensitive.PUT(adminSettingsRoute, admindelivery.AdminSettings)

	// Report content
	admin.GET("/reports/content", admindelivery.AdminReportsContent)
	admin.PATCH("/reports/content", admindelivery.AdminReportsContent)

	// Missing notifications
	admin.GET("/notifications", admindelivery.AdminListNotifications)
	admin.POST("/notifications/:id/read", admindelivery.AdminMarkNotificationRead)
	admin.POST("/notifications/read-all", admindelivery.AdminMarkAllNotificationsRead)
	admin.DELETE("/notifications/:id", admindelivery.AdminDeleteNotification)

	// Interactive Questions for Video Lessons
	admin.GET("/courses/lessons/:id/interactive-questions", handlers.GetInteractiveQuestions)
	admin.POST("/courses/lessons/:id/interactive-questions", handlers.CreateInteractiveQuestion)
	admin.GET("/interactive-questions/:id", handlers.GetInteractiveQuestion)
	admin.PATCH("/interactive-questions", handlers.UpdateInteractiveQuestion)
	admin.DELETE("/interactive-questions/:id", handlers.DeleteInteractiveQuestion)

	// Lesson Transcripts (SRT/VTT upload for the video player's transcript panel)
	admin.POST("/courses/lessons/:id/transcript", handlers.UpsertLessonTranscript)
	admin.DELETE("/courses/lessons/:id/transcript", handlers.DeleteLessonTranscript)

	// Course Pricing, Versioning and Bundles are registered in
	// hexagonal_routes.go via CourseRESTHandler.
}
