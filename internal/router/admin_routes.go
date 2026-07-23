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

// SetupAdminRoutes configures administrative API endpoints.
// Access is layered:
//   - General admin group: ADMIN, SUPER_ADMIN, MODERATOR (AdminOrModerator)
//   - Sensitive sub-group: ADMIN and SUPER_ADMIN only (AdminRequired)
func SetupAdminRoutes(router *gin.Engine) {
	admin := router.Group("/api/admin")
	admin.Use(middleware.Auth())
	admin.Use(middleware.AdminOrModerator())
	admin.Use(middleware.StrictRBAC())
	// Keep an immutable, server-authoritative audit trail for every admin
	// operation. This is deliberately registered after authentication so the
	// logger can associate each event with the authenticated administrator.
	admin.Use(middleware.NewAdminAuditLogger(middleware.DefaultAuditLoggerConfig()).LogAdminOperations())
	{
		// Dashboard
		admin.GET("/dashboard", handlers.GetAdminDashboard)
		admin.GET("/live", handlers.GetAdminLive)
		admin.GET("/live-sessions", handlers.AdminListLiveSessions)
		admin.POST("/live-sessions", handlers.AdminCreateLiveSession)
		admin.GET("/analytics", handlers.GetAdminAnalytics)
		// Analytics sub-resources used by the admin analytics workspace.
		admin.GET("/analytics/revenue", handlers.GetAdminRevenue)
		admin.GET("/analytics/journeys", handlers.GetUserJourneys)
		admin.GET("/analytics/metrics", handlers.GetActivityMetrics)
		admin.GET("/infrastructure/stats", handlers.GetAdminInfrastructureStats)
		admin.GET(adminAnnouncementsRoute, handlers.GetAdminAnnouncements)
		admin.POST(adminAnnouncementsRoute, handlers.CreateAdminAnnouncement)
		admin.PATCH(adminAnnouncementsRoute, handlers.UpdateAdminAnnouncement)
		admin.DELETE(adminAnnouncementsRoute, handlers.DeleteAdminAnnouncement)
		admin.GET("/reports/overview", handlers.GetAdminReportsOverview)
		admin.GET("/reports/users", handlers.GetAdminReportsUsers)
		admin.GET("/reports/books", handlers.GetAdminReportsBooks)
		// Audit logs are read-only. Audit entries themselves are created by the
		// middleware above, never from a browser supplied payload.
		admin.GET("/audit-logs", handlers.AdminGetAuditLogs)

		// AI
		admin.GET("/ai", handlers.AdminAIGet)
		admin.POST("/ai", handlers.AdminAIPost)

		// ---------------------------------------------------------------
		// Sensitive operations: ADMIN and SUPER_ADMIN only.
		// Created as a sub-group of `admin` so it inherits Auth() +
		// AdminOrModerator() + StrictRBAC() automatically; we then layer
		// AdminRequired() on top to ALSO block MODERATOR.
		// ---------------------------------------------------------------
		sensitive := admin.Group("")
		sensitive.Use(middleware.AdminRequired()) // additional check: blocks MODERATOR

		// Impersonation (admin-only)
		sensitive.POST("/reset-circuit-breaker", handlers.AdminResetCircuitBreaker)
		sensitive.POST("/impersonate", handlers.ImpersonateUser)
		sensitive.DELETE("/impersonate", handlers.DeleteImpersonation)

		// Teachers
		admin.GET(adminTeachersRoute, handlers.GetTeachersForAdmin)
		admin.POST(adminTeachersRoute, handlers.CreateTeacher)
		admin.PATCH(adminTeachersRoute, handlers.UpdateTeacher)
		admin.DELETE(adminTeachersRoute, handlers.DeleteTeacher)
		admin.GET(adminTeachersRoute+"/applications", handlers.GetTeacherApplications)
		admin.DELETE(adminTeachersRoute+"/applications", handlers.ReviewTeacherApplication)

		// Instructor management
		admin.GET("/instructors", handlers.GetInstructors)
		admin.POST("/instructors", handlers.CreateInstructor)
		admin.GET("/instructors/statistics", handlers.GetInstructorStatistics)
		admin.GET("/instructors/export", handlers.ExportInstructors)
		admin.POST("/instructors/bulk-delete", handlers.BulkDeleteInstructors)
		admin.POST("/instructors/bulk-notifications", handlers.BulkSendInstructorNotifications)
		admin.GET("/instructors/:id", handlers.GetInstructor)
		admin.PATCH("/instructors/:id", handlers.UpdateInstructor)
		admin.DELETE("/instructors/:id", handlers.DeleteInstructor)
		admin.POST("/instructors/:id/approve", handlers.ApproveInstructor)
		admin.POST("/instructors/:id/reject", handlers.RejectInstructor)
		admin.POST("/instructors/:id/suspend", handlers.SuspendInstructor)
		admin.GET("/instructors/:id/documents", handlers.GetInstructorDocuments)
		admin.POST("/instructors/:id/documents/:documentId/review", handlers.ReviewInstructorDocument)
		admin.GET("/instructors/:id/contracts", handlers.GetInstructorContracts)
		admin.POST("/instructors/:id/contracts", handlers.CreateInstructorContract)
		admin.GET("/instructors/:id/payouts", handlers.GetInstructorPayouts)
		admin.GET("/instructors/:id/performance", handlers.GetInstructorPerformance)
		admin.GET("/instructors/:id/violations", handlers.GetInstructorViolations)
		admin.POST("/instructors/:id/violations", handlers.CreateInstructorViolation)
		admin.POST("/instructors/:id/violations/:violationId/resolve", handlers.ResolveInstructorViolation)
		admin.POST("/instructors/:id/notifications", handlers.SendInstructorNotification)

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

		// Backups (admin-only)
		sensitive.GET("/backups", handlers.GetBackups)
		sensitive.POST("/backups", handlers.CreateBackup)
		sensitive.GET("/backups/stats", handlers.GetBackupStats)
		sensitive.GET("/backups/tables", handlers.GetDatabaseTables)
		sensitive.POST(adminBackupsScheduleRoute, handlers.ScheduleBackup)
		sensitive.PUT(adminBackupsScheduleRoute, handlers.UpdateBackupSchedule)
		sensitive.PUT(adminBackupsScheduleIDRoute, handlers.UpdateBackupSchedule)
		sensitive.DELETE(adminBackupsScheduleRoute, handlers.DeleteBackupSchedule)
		sensitive.DELETE(adminBackupsScheduleIDRoute, handlers.DeleteBackupSchedule)
		sensitive.DELETE("/backups/:id", handlers.DeleteBackup)
		sensitive.GET("/backups/:id/download", handlers.DownloadBackup)
		sensitive.POST("/backups/:id/restore", handlers.RestoreBackup)
		sensitive.POST("/backups/:id/verify", handlers.VerifyBackup)
		sensitive.GET("/backups/:id/progress", handlers.GetBackupProgress)

		// Session Management
		admin.GET("/security/sessions", handlers.GetActiveSessions)
		admin.GET("/security/sessions/stats", handlers.GetSessionStats)
		admin.POST("/security/sessions/:id/revoke", handlers.RevokeSession)
		admin.POST("/security/sessions/revoke-others", handlers.RevokeOtherSessions)
		admin.POST("/security/sessions/user/:userId/revoke-all", handlers.RevokeUserSessions)
		admin.POST("/security/sessions/:id/suspend", handlers.SuspendSession)
		admin.GET("/security/sessions/activity", handlers.GetSessionActivity)
		admin.GET("/security/logs/users/:id", handlers.GetSecurityLogsForUser)

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
		admin.POST("/ab-testing/:id/track", handlers.AdminTrackABEvent)

		// Forum Categories
		admin.GET("/forum", handlers.AdminGetForum)
		admin.GET("/forum-categories", handlers.AdminGetForumCategories)
		admin.POST("/forum-categories", handlers.AdminCreateForumCategory)

		// Books
		admin.GET("/books", handlers.AdminGetBooks)

		// Affiliates
		admin.GET("/affiliates", handlers.AdminGetAffiliates)
		admin.POST("/affiliates", handlers.AdminCreateAffiliate)
		admin.GET("/affiliates/:id", handlers.AdminGetAffiliate)
		admin.PATCH("/affiliates/:id", handlers.AdminUpdateAffiliate)
		admin.DELETE("/affiliates/:id", handlers.AdminDeleteAffiliate)
		admin.GET("/affiliates/:id/referrals", handlers.AdminGetAffiliateReferrals)
		admin.POST("/affiliates/:id/pay", handlers.AdminPayAffiliate)
		admin.POST("/books", handlers.AdminCreateBook)
		admin.PATCH("/books/:id", handlers.AdminUpdateBook)
		admin.DELETE("/books/:id", handlers.AdminDeleteBook)
		admin.GET("/books/views", handlers.AdminBookReviews)
		admin.GET("/books/reviews", handlers.AdminBookReviews)
		admin.DELETE("/books/reviews", handlers.AdminBookReviews)

		// User/Subject Admin Operations
		// User read/update: moderators can access; delete is admin-only
		admin.GET("/users", handlers.GetUsers)
		admin.POST("/users", handlers.CreateUser)
		admin.GET(adminUserIDRoute, handlers.GetUserByID)
		admin.PATCH(adminUserIDRoute, handlers.UpdateUser)
		sensitive.DELETE(adminUserIDRoute, handlers.DeleteUser) // ADMIN-only
		admin.GET("/users/:id/enrollments", handlers.GetUserEnrollments)
		admin.POST("/users/:id/enrollments", handlers.AdminEnrollUser)
		admin.GET("/users/:id/login-attempts", handlers.GetUserLoginAttempts)
		admin.GET("/users/:id/video-engagement", handlers.GetUserVideoEngagement)
		admin.GET("/search/users", handlers.SearchUsers)
		admin.POST("/users/search", handlers.SearchUsers)

		// Subject
		admin.GET(adminSubjectsRoute, handlers.GetSubjects)
		admin.POST(adminSubjectsRoute, handlers.CreateSubject)
		admin.PATCH(adminSubjectsRoute, handlers.UpdateSubject)
		admin.DELETE(adminSubjectsRoute, handlers.DeleteSubject)

		// Course aliases for Admin panel compatibility
		admin.GET("/courses", handlers.GetSubjects)
		admin.POST("/courses", handlers.CreateSubject)
		admin.PATCH("/courses", handlers.UpdateSubject)
		admin.DELETE("/courses", handlers.DeleteSubject)
		admin.GET("/courses/:id/curriculum", handlers.GetSubjectCurriculum)
		admin.PUT("/courses/:id/curriculum", handlers.UpdateCourseCurriculum)
		admin.PATCH("/courses/:id/curriculum", handlers.UpdateCourseCurriculum)
		admin.POST("/courses/duplicate", handlers.DuplicateCourse)
		admin.POST("/courses/batch", handlers.BatchCourseAction)

		// Course lifecycle workflow (migration 0064) - TODO: implement missing handlers
		// admin.POST("/courses/:id/submit-review", handlers.SubmitCourseForReview)
		admin.POST("/courses/:id/reject", handlers.RejectCourse)
		admin.POST("/courses/:id/archive", handlers.ArchiveCourse)
		// admin.POST("/courses/:id/restore", handlers.RestoreCourse)
		// admin.GET("/courses/review-queue", handlers.GetReviewQueue)
		admin.GET("/courses/:id/changelog", handlers.GetCourseChangelog)

		// Course tags CRUD
		admin.GET("/course-tags", handlers.GetCourseTags)
		admin.POST("/course-tags", handlers.CreateCourseTag)
		admin.PATCH("/course-tags/:id", handlers.UpdateCourseTag)
		admin.DELETE("/course-tags/:id", handlers.DeleteCourseTag)
		admin.PUT("/courses/:id/tags", handlers.AssignTagsToCourse)

		// Related / prerequisite courses
		admin.GET("/courses/:id/related", handlers.GetRelatedCourses)
		admin.POST("/courses/:id/related", handlers.AddRelatedCourse)
		admin.DELETE("/courses/:id/related/:relatedId", handlers.RemoveRelatedCourse)

		// Review workflow comments
		admin.GET("/courses/:id/review-comments", handlers.GetReviewComments)
		admin.POST("/courses/:id/review-comments", handlers.AddReviewComment)
		admin.PATCH("/courses/:id/review-comments/:commentId", handlers.UpdateReviewComment)

		// Curriculum
		admin.PATCH("/subjects/:id/curriculum", handlers.UpdateCourseCurriculum)
		admin.GET("/subjects/:id/curriculum", handlers.GetSubjectCurriculum)

		// Course Students (view list of enrolled students)
		admin.GET("/courses/:id/students", handlers.GetCourseStudents)

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

		// Partitions (admin-only - system-level operation)
		sensitive.GET("/database-partitions", handlers.DatabasePartitions)
		// Marketing & Contests
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

		// Settings (write = admin-only, read = open to moderators)
		admin.GET(adminSettingsRoute, handlers.AdminSettings)
		sensitive.PATCH(adminSettingsRoute, handlers.AdminSettings)
		sensitive.PUT(adminSettingsRoute, handlers.AdminSettings)

		// Report content
		admin.GET("/reports/content", handlers.AdminReportsContent)
		admin.PATCH("/reports/content", handlers.AdminReportsContent)

		// Missing notifications
		admin.GET("/notifications", handlers.AdminListNotifications)
		admin.POST("/notifications/:id/read", handlers.AdminMarkNotificationRead)
		admin.POST("/notifications/read-all", handlers.AdminMarkAllNotificationsRead)
		admin.DELETE("/notifications/:id", handlers.AdminDeleteNotification)

		// Interactive Questions for Video Lessons - TODO: implement missing handlers
		// admin.GET("/courses/lessons/:id/interactive-questions", handlers.GetInteractiveQuestions)
		// admin.POST("/courses/lessons/:id/interactive-questions", handlers.CreateInteractiveQuestion)
		// admin.GET("/interactive-questions/:id", handlers.GetInteractiveQuestion)
		// admin.PATCH("/interactive-questions", handlers.UpdateInteractiveQuestion)
		// admin.DELETE("/interactive-questions/:id", handlers.DeleteInteractiveQuestion)

		// Phase 1: Course Pricing, Bundles & Workflow
		// Course Pricing
		admin.GET("/courses/:id/pricing", handlers.GetCoursePricing)
		admin.PUT("/courses/:id/pricing", handlers.UpdateCoursePricing)

		// Course Bundles
		admin.GET("/bundles", handlers.ListBundles)
		admin.POST("/bundles", handlers.CreateBundle)
		admin.GET("/bundles/:id", handlers.GetBundle)
		admin.PATCH("/bundles/:id", handlers.UpdateBundle)
		admin.DELETE("/bundles/:id", handlers.DeleteBundle)
		admin.POST("/bundles/:id/courses", handlers.AddCoursesToBundle)
		admin.DELETE("/bundles/:id/courses", handlers.RemoveCoursesFromBundle)
		admin.GET("/bundles/:id/enrollments", handlers.GetBundleEnrollments)

		// Course Versioning
		admin.GET("/courses/:id/versions", handlers.GetCourseVersions)
		admin.POST("/courses/:id/versions", handlers.CreateCourseVersion)
		admin.POST("/courses/:id/versions/:versionId/restore", handlers.RestoreCourseVersion)

		// Course Status Workflow
		admin.POST("/courses/:id/submit-review", handlers.SubmitForReview)
		admin.POST("/courses/:id/unarchive", handlers.UnarchiveCourse)
		admin.GET("/courses/pending-review", handlers.GetCoursesPendingReview)
		admin.POST("/courses/bulk-status", handlers.BulkStatusChange)
	}
}
