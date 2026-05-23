package router

import (
	"thanawy-backend/internal/api/handlers"
	"thanawy-backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

const (
	adminAnnouncementsRoute     = "/announcements"
	adminTeachersRoute          = "/teachers"
	adminCourseCategoriesRoute  = "/course-categories"
	adminBackupsScheduleRoute   = "/backups/schedule"
	adminBackupsScheduleIDRoute = adminBackupsScheduleRoute + "/:id"
	adminUserIDRoute            = "/users/:id"
	adminSubjectsRoute          = "/subjects"
	adminReportsIDRoute         = "/reports/:id"
	adminCoursesActionRoute     = "/courses/action"
	adminSettingsRoute          = "/settings"
)

// SetupAdminRoutes configures administrative API endpoints
func SetupAdminRoutes(router *gin.Engine) {
	admin := router.Group("/api/admin")
	admin.Use(middleware.Auth())
	admin.Use(middleware.AdminRequired())
	admin.Use(middleware.StrictRBAC())
	{
		// Dashboard
		admin.GET("/dashboard", handlers.GetAdminDashboard)
		admin.GET("/live", handlers.GetAdminLive)
		admin.GET("/analytics", handlers.GetAdminAnalytics)
		admin.GET("/infrastructure/stats", handlers.GetAdminInfrastructureStats)
		admin.GET(adminAnnouncementsRoute, handlers.GetAdminAnnouncements)
		admin.POST(adminAnnouncementsRoute, handlers.CreateAdminAnnouncement)
		admin.PATCH(adminAnnouncementsRoute, handlers.UpdateAdminAnnouncement)
		admin.DELETE(adminAnnouncementsRoute, handlers.DeleteAdminAnnouncement)
		admin.GET("/reports/overview", handlers.GetAdminReportsOverview)
		admin.GET("/reports/users", handlers.GetAdminReportsUsers)
		admin.GET("/reports/books", handlers.GetAdminReportsBooks)

		// AI / Impersonation
		admin.GET("/ai", handlers.AdminAIGet)
		admin.POST("/ai", handlers.AdminAIPost)
		admin.POST("/reset-circuit-breaker", handlers.AdminResetCircuitBreaker)
		admin.POST("/impersonate", handlers.ImpersonateUser)
		admin.DELETE("/impersonate", handlers.DeleteImpersonation)

		// Teachers
		admin.GET(adminTeachersRoute, handlers.GetTeachersForAdmin)
		admin.POST(adminTeachersRoute, handlers.CreateTeacher)
		admin.PATCH(adminTeachersRoute, handlers.UpdateTeacher)
		admin.DELETE(adminTeachersRoute, handlers.DeleteTeacher)

		// Categories
		admin.GET(adminCourseCategoriesRoute, handlers.GetCategoriesForAdmin)
		admin.POST(adminCourseCategoriesRoute, handlers.CreateCategory)
		admin.PATCH(adminCourseCategoriesRoute, handlers.UpdateCategory)
		admin.DELETE(adminCourseCategoriesRoute, handlers.DeleteCategory)

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
		admin.PATCH("/tickets/:id/tags", handlers.UpdateTicketTags)

		// Backups
		admin.GET("/backups", handlers.GetBackups)
		admin.POST("/backups", handlers.CreateBackup)
		admin.GET("/backups/stats", handlers.GetBackupStats)
		admin.GET("/backups/tables", handlers.GetDatabaseTables)
		admin.POST(adminBackupsScheduleRoute, handlers.ScheduleBackup)
		admin.PUT(adminBackupsScheduleRoute, handlers.UpdateBackupSchedule)
		admin.PUT(adminBackupsScheduleIDRoute, handlers.UpdateBackupSchedule)
		admin.DELETE(adminBackupsScheduleRoute, handlers.DeleteBackupSchedule)
		admin.DELETE(adminBackupsScheduleIDRoute, handlers.DeleteBackupSchedule)
		admin.DELETE("/backups/:id", handlers.DeleteBackup)
		admin.GET("/backups/:id/download", handlers.DownloadBackup)
		admin.POST("/backups/:id/restore", handlers.RestoreBackup)
		admin.POST("/backups/:id/verify", handlers.VerifyBackup)
		admin.GET("/backups/:id/progress", handlers.GetBackupProgress)

		// 2FA Management (for Admins)
		admin.GET("/security/2fa/status", handlers.GetTwoFactorStatus)
		admin.POST("/security/2fa/setup", handlers.InitiateTwoFactorSetup)
		admin.POST("/security/2fa/verify", handlers.VerifyTwoFactor)
		admin.POST("/security/2fa/disable", handlers.DisableTwoFactor)
		admin.POST("/security/2fa/backup-codes", handlers.RegenerateBackupCodes)
		admin.POST("/security/2fa/verify-login", handlers.VerifyTwoFactorLogin)
		admin.POST("/users/:id/2fa/enforce", handlers.AdminEnforceUserTwoFactor)
		admin.POST("/users/:id/2fa/reset", handlers.AdminResetUserTwoFactor)

		// Session Management
		admin.GET("/security/sessions", handlers.GetActiveSessions)
		admin.GET("/security/sessions/stats", handlers.GetSessionStats)
		admin.POST("/security/sessions/:id/revoke", handlers.RevokeSession)
		admin.POST("/security/sessions/revoke-others", handlers.RevokeOtherSessions)
		admin.POST("/security/sessions/user/:userId/revoke-all", handlers.RevokeUserSessions)
		admin.POST("/security/sessions/:id/suspend", handlers.SuspendSession)
		admin.GET("/security/sessions/activity", handlers.GetSessionActivity)

		// IP Whitelist
		admin.GET("/security/ip-whitelist", handlers.GetIPWhitelist)
		admin.POST("/security/ip-whitelist", handlers.AddIPToWhitelist)
		admin.GET("/security/ip-whitelist/settings", handlers.GetIPWhitelistSettings)
		admin.POST("/security/ip-whitelist/settings", handlers.UpdateIPWhitelistSettings)
		admin.GET("/security/ip-whitelist/blocked", handlers.GetBlockedAttempts)
		admin.POST("/security/ip-whitelist/bulk", handlers.BulkAddIPToWhitelist)
		admin.GET("/security/ip-whitelist/check", handlers.CheckIPWhitelist)
		admin.PATCH("/security/ip-whitelist/:id", handlers.UpdateIPWhitelistEntry)
		admin.DELETE("/security/ip-whitelist/:id", handlers.RemoveIPFromWhitelist)

		// General CRUD / Gamification
		// Achievements
		admin.GET("/achievements", handlers.AdminGetAchievements)
		admin.POST("/achievements", handlers.AdminCreateAchievement)
		admin.PATCH("/achievements/:id", handlers.AdminUpdateAchievement)
		admin.DELETE("/achievements/:id", handlers.AdminDeleteAchievement)

		// Rewards
		admin.GET("/rewards", handlers.AdminGetRewards)
		admin.POST("/rewards", handlers.AdminCreateReward)
		admin.PATCH("/rewards/:id", handlers.AdminUpdateReward)
		admin.DELETE("/rewards/:id", handlers.AdminDeleteReward)

		// Seasons
		admin.GET("/seasons", handlers.AdminGetSeasons)
		admin.POST("/seasons", handlers.AdminCreateSeason)
		admin.PATCH("/seasons/:id", handlers.AdminUpdateSeason)
		admin.DELETE("/seasons/:id", handlers.AdminDeleteSeason)

		// Coupons
		admin.GET("/coupons", handlers.AdminGetCoupons)
		admin.POST("/coupons", handlers.AdminCreateCoupon)
		admin.PATCH("/coupons/:id", handlers.AdminUpdateCoupon)
		admin.DELETE("/coupons/:id", handlers.AdminDeleteCoupon)

		// Challenges
		admin.GET("/challenges", handlers.AdminGetChallenges)
		admin.POST("/challenges", handlers.AdminCreateChallenge)
		admin.PATCH("/challenges/:id", handlers.AdminUpdateChallenge)
		admin.DELETE("/challenges/:id", handlers.AdminDeleteChallenge)

		// Blog
		admin.GET("/blog", handlers.AdminGetBlog)
		admin.POST("/blog", handlers.AdminCreateBlogPost)
		admin.PATCH("/blog/:id", handlers.AdminUpdateBlogPost)
		admin.DELETE("/blog/:id", handlers.AdminDeleteBlogPost)

		// Automations
		admin.GET("/automations", handlers.AdminGetAutomations)
		admin.POST("/automations", handlers.AdminCreateAutomation)
		admin.PATCH("/automations/:id", handlers.AdminUpdateAutomation)
		admin.DELETE("/automations/:id", handlers.AdminDeleteAutomation)

		// Campaigns
		admin.GET("/marketing/campaigns", handlers.AdminGetCampaigns)
		admin.POST("/marketing/campaigns", handlers.AdminCreateCampaign)
		admin.PATCH("/marketing/campaigns/:id", handlers.AdminUpdateCampaign)
		admin.DELETE("/marketing/campaigns/:id", handlers.AdminDeleteCampaign)

		// AB Testing
		admin.GET("/ab-testing", handlers.AdminGetABTests)
		admin.POST("/ab-testing", handlers.AdminCreateABTest)
		admin.PATCH("/ab-testing/:id", handlers.AdminUpdateABTest)
		admin.DELETE("/ab-testing/:id", handlers.AdminDeleteABTest)

		// Forum Categories
		admin.GET("/forum", handlers.AdminGetForum)
		admin.GET("/forum-categories", handlers.AdminGetForumCategories)
		admin.POST("/forum-categories", handlers.AdminCreateForumCategory)

		// Books
		admin.GET("/books", handlers.AdminGetBooks)
		admin.POST("/books", handlers.AdminCreateBook)
		admin.PATCH("/books/:id", handlers.AdminUpdateBook)
		admin.DELETE("/books/:id", handlers.AdminDeleteBook)
		admin.GET("/books/views", handlers.AdminBookReviews)
		admin.GET("/books/reviews", handlers.AdminBookReviews)
		admin.DELETE("/books/reviews", handlers.AdminBookReviews)

		// User/Subject Admin Operations
		// User
		admin.GET("/users", handlers.GetUsers)
		admin.POST("/users", handlers.CreateUser)
		admin.GET(adminUserIDRoute, handlers.GetUserByID)
		admin.PATCH(adminUserIDRoute, handlers.UpdateUser)
		admin.DELETE(adminUserIDRoute, handlers.DeleteUser)
		admin.GET("/search/users", handlers.SearchUsers)
		admin.POST("/users/search", handlers.SearchUsers)

		// Subject
		admin.GET(adminSubjectsRoute, handlers.GetSubjects)
		admin.POST(adminSubjectsRoute, handlers.CreateSubject)
		admin.PATCH(adminSubjectsRoute, handlers.UpdateSubject)
		admin.DELETE(adminSubjectsRoute, handlers.DeleteSubject)

		// Curriculum
		admin.PATCH("/subjects/:id/curriculum", handlers.UpdateCourseCurriculum)
		admin.GET("/subjects/:id/curriculum", handlers.GetSubjectCurriculum)

		// Manual Enroll
		admin.GET("/courses/enrollments", handlers.GetCourseEnrollments)
		admin.POST("/courses/enroll", handlers.ManualEnroll)
		admin.POST("/courses/unenroll", handlers.UnenrollUser)
		admin.POST("/courses/lessons/attachments", handlers.AddLessonAttachment)

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

		// Partitions, Marketing & Contests (moved from protected_routes.go)
		admin.GET("/database-partitions", handlers.DatabasePartitions)
		admin.GET("/marketing", handlers.Marketing)
		admin.POST("/marketing", handlers.Marketing)
		admin.GET("/contests", handlers.Contests)
		admin.POST("/contests", handlers.Contests)
		admin.PATCH("/contests/:id", handlers.Contests)
		admin.DELETE("/contests/:id", handlers.Contests)

		// Course action
		admin.GET(adminCoursesActionRoute, handlers.AdminCourseAction)
		admin.POST(adminCoursesActionRoute, handlers.AdminCourseAction)
		admin.PATCH(adminCoursesActionRoute, handlers.AdminCourseAction)
		admin.PUT(adminCoursesActionRoute, handlers.AdminCourseAction)
		admin.GET("/courses/export", handlers.AdminCourseAction)

		// Setting
		admin.GET(adminSettingsRoute, handlers.AdminSettings)
		admin.PATCH(adminSettingsRoute, handlers.AdminSettings)
		admin.PUT(adminSettingsRoute, handlers.AdminSettings)

		// Report content
		admin.GET("/reports/content", handlers.AdminReportsContent)
		admin.PATCH("/reports/content", handlers.AdminReportsContent)

		// Missing notifications
		admin.GET("/notifications", handlers.AdminListNotifications)
		admin.POST("/notifications/:id/read", handlers.AdminMarkNotificationRead)
		admin.POST("/notifications/read-all", handlers.AdminMarkAllNotificationsRead)
		admin.DELETE("/notifications/:id", handlers.AdminDeleteNotification)
	}
}
