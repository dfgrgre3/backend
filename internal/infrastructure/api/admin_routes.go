package api

import (
	models "thanawy-backend/internal/domain/common"

	admindelivery "thanawy-backend/internal/infrastructure/api/handlers/admin"
	dashboarddelivery "thanawy-backend/internal/infrastructure/api/handlers/admin"

	aidelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	analyticsdelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	authdelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	contentdelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	coursedelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	gamificationdelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	handlers "thanawy-backend/internal/infrastructure/api/handlers/protected"
	marketingdelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	notificationdelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	paymentdelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	searchdelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	systemdelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"
	userdelivery "thanawy-backend/internal/infrastructure/api/handlers/protected"

	"thanawy-backend/internal/infrastructure/api/middleware"

	"time"

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
	admin.Use(middleware.AdminAPIPermissionRequired())
	// Bound all administrative traffic with the Redis-backed limiter.
	// GlobalRateLimiter resolves the current Redis client per request and fails closed if unavailable.
	// 300/min is sized for data-heavy admin pages (tables, charts, live dashboards)
	// that issue many parallel fetches on load.
	admin.Use(middleware.GlobalRateLimiter(300, time.Minute))
	// Keep an immutable, server-authoritative audit trail for every admin
	// operation. This is deliberately registered after authentication so the
	// logger can associate each event with the authenticated administrator.
	admin.Use(middleware.NewAdminAuditLogger(middleware.DefaultAuditLoggerConfig()).LogAdminOperations())
	{
		// Dashboard
		// The aggregate endpoint stays as the compatibility surface for the
		// current admin UI; the routes below are the granular, per-widget API.
		admin.GET("/dashboard", dashboarddelivery.GetAdminDashboard)
		admin.GET("/dashboard/filters", admindelivery.GetDashboardFiltersMetadata)
		admin.GET("/dashboard/summary", admindelivery.GetDashboardSummary)
		admin.GET("/dashboard/time-series", admindelivery.GetDashboardTimeSeries)
		admin.GET("/dashboard/pending-actions", admindelivery.GetDashboardPendingActions)
		admin.GET("/dashboard/alerts", admindelivery.GetDashboardAlerts)
		admin.POST("/dashboard/alerts/:alertId/acknowledge", admindelivery.AcknowledgeDashboardAlert)
		admin.GET("/dashboard/recent-activities", admindelivery.GetDashboardRecentActivities)
		admin.GET("/dashboard/top-courses", admindelivery.GetDashboardTopCourses)
		admin.GET("/dashboard/system-health", admindelivery.GetDashboardSystemHealth)
		admin.POST("/dashboard/refresh", admindelivery.RefreshDashboardData)
		admin.POST("/dashboard/export", admindelivery.ExportDashboardReport)
		admin.GET("/dashboard/export/:exportJobId", admindelivery.GetDashboardExportStatus)
		admin.POST("/dashboard/saved-filters", admindelivery.SaveDashboardFilter)
		admin.DELETE("/dashboard/saved-filters/:filterId", admindelivery.DeleteDashboardSavedFilter)
		admin.GET("/live", admindelivery.GetAdminLive)
		admin.GET("/live-sessions", admindelivery.AdminListLiveSessions)
		admin.POST("/live-sessions", admindelivery.AdminCreateLiveSession)
		admin.GET("/analytics", systemdelivery.GetAdminAnalytics)
		admin.GET("/health/detailed", systemdelivery.GetDetailedAdminHealth)
		admin.GET("/health/export", systemdelivery.ExportDetailedAdminHealth)
		// Analytics sub-resources used by the admin analytics workspace.
		admin.GET("/analytics/revenue", paymentdelivery.GetAdminRevenue)
		admin.GET("/analytics/journeys", analyticsdelivery.GetUserJourneys)
		admin.GET("/analytics/metrics", analyticsdelivery.GetActivityMetrics)
		admin.GET("/infrastructure/stats", systemdelivery.GetAdminInfrastructureStats)
		admin.GET(adminAnnouncementsRoute, analyticsdelivery.GetAdminAnnouncements)
		admin.POST(adminAnnouncementsRoute, analyticsdelivery.CreateAdminAnnouncement)
		admin.PATCH(adminAnnouncementsRoute, analyticsdelivery.UpdateAdminAnnouncement)
		admin.DELETE(adminAnnouncementsRoute, analyticsdelivery.DeleteAdminAnnouncement)
		admin.GET("/reports/overview", dashboarddelivery.GetAdminReportsOverview)
		admin.GET("/reports/users", dashboarddelivery.GetAdminReportsUsers)
		admin.GET("/reports/books", dashboarddelivery.GetAdminReportsBooks)
		// Audit logs are read-only. Audit entries themselves are created by the
		// middleware above, never from a browser supplied payload.
		admin.GET("/audit-logs", analyticsdelivery.AdminGetAuditLogs)

		// AI
		admin.GET("/ai", aidelivery.AdminAIGet)
		admin.POST("/ai", aidelivery.AdminAIPost)
		admin.GET("/ai/analyze", admindelivery.AdminAIAnalyze)
		admin.POST("/ai/analyze", admindelivery.AdminAIAnalyze)

		// ---------------------------------------------------------------
		// Sensitive operations: ADMIN and SUPER_ADMIN only.
		// Created as a sub-group of `admin` so it inherits Auth() +
		// AdminOrModerator() + StrictRBAC() automatically; we then layer
		// AdminRequired() on top to ALSO block MODERATOR.
		// ---------------------------------------------------------------
		sensitive := admin.Group("")
		sensitive.Use(middleware.AdminRequired()) // additional check: blocks MODERATOR

		// Impersonation (admin-only)
		sensitive.POST("/reset-circuit-breaker", aidelivery.AdminResetCircuitBreaker)
		sensitive.POST("/impersonate", systemdelivery.ImpersonateUser)
		sensitive.DELETE("/impersonate", systemdelivery.DeleteImpersonation)

		// Teachers
		admin.GET(adminTeachersRoute, userdelivery.GetTeachersForAdmin)
		admin.POST(adminTeachersRoute, userdelivery.CreateTeacher)
		admin.PATCH(adminTeachersRoute, userdelivery.UpdateTeacher)
		admin.DELETE(adminTeachersRoute, userdelivery.DeleteTeacher)
		admin.GET(adminTeachersRoute+"/applications", userdelivery.GetTeacherApplications)
		admin.DELETE(adminTeachersRoute+"/applications", userdelivery.ReviewTeacherApplication)

		// Instructor management
		admin.GET("/instructors", userdelivery.GetInstructors)
		admin.POST("/instructors", userdelivery.CreateInstructor)
		admin.GET("/instructors/statistics", userdelivery.GetInstructorStatistics)
		admin.GET("/instructors/export", userdelivery.ExportInstructors)
		admin.POST("/instructors/bulk-delete", userdelivery.BulkDeleteInstructors)
		admin.POST("/instructors/bulk-notifications", userdelivery.BulkSendInstructorNotifications)
		admin.GET("/instructors/:id", userdelivery.GetInstructor)
		admin.PATCH("/instructors/:id", userdelivery.UpdateInstructor)
		admin.DELETE("/instructors/:id", userdelivery.DeleteInstructor)
		admin.POST("/instructors/:id/approve", userdelivery.ApproveInstructor)
		admin.POST("/instructors/:id/reject", userdelivery.RejectInstructor)
		admin.POST("/instructors/:id/suspend", userdelivery.SuspendInstructor)
		admin.GET("/instructors/:id/documents", userdelivery.GetInstructorDocuments)
		admin.POST("/instructors/:id/documents/:documentId/review", userdelivery.ReviewInstructorDocument)
		admin.GET("/instructors/:id/contracts", userdelivery.GetInstructorContracts)
		admin.POST("/instructors/:id/contracts", userdelivery.CreateInstructorContract)
		admin.GET("/instructors/:id/payouts", userdelivery.GetInstructorPayouts)
		admin.GET("/instructors/:id/performance", userdelivery.GetInstructorPerformance)
		admin.GET("/instructors/:id/violations", userdelivery.GetInstructorViolations)
		admin.POST("/instructors/:id/violations", userdelivery.CreateInstructorViolation)
		admin.POST("/instructors/:id/violations/:violationId/resolve", userdelivery.ResolveInstructorViolation)
		admin.POST("/instructors/:id/notifications", userdelivery.SendInstructorNotification)

		// Categories
		admin.GET(adminCourseCategoriesRoute, contentdelivery.GetCategoriesForAdmin)
		admin.POST(adminCourseCategoriesRoute, contentdelivery.CreateCategory)
		admin.PATCH(adminCourseCategoriesRoute, contentdelivery.UpdateCategory)
		admin.DELETE(adminCourseCategoriesRoute, contentdelivery.DeleteCategory)

		// Support Tickets
		admin.GET("/tickets", systemdelivery.GetSupportTickets)
		admin.POST("/tickets", systemdelivery.CreateSupportTicket)
		admin.GET("/tickets/stats", systemdelivery.GetTicketStats)
		admin.GET("/tickets/:id", systemdelivery.GetSupportTicket)
		admin.POST("/tickets/:id/messages", systemdelivery.SendTicketMessage)
		admin.PATCH("/tickets/:id/status", systemdelivery.UpdateTicketStatus)
		admin.PATCH("/tickets/:id/priority", systemdelivery.UpdateTicketPriority)
		admin.POST("/tickets/:id/assign", systemdelivery.AssignTicket)
		admin.POST("/tickets/:id/close", systemdelivery.CloseTicket)
		admin.PATCH("/tickets/:id/tags", admindelivery.UpdateTicketTags)

		// Backups (admin-only)
		sensitive.GET("/backups", systemdelivery.GetBackups)
		sensitive.POST("/backups", systemdelivery.CreateBackup)
		sensitive.GET("/backups/stats", systemdelivery.GetBackupStats)
		sensitive.GET("/backups/tables", systemdelivery.GetDatabaseTables)
		sensitive.POST(adminBackupsScheduleRoute, systemdelivery.ScheduleBackup)
		sensitive.PUT(adminBackupsScheduleRoute, systemdelivery.ScheduleBackup)
		sensitive.PUT(adminBackupsScheduleIDRoute, systemdelivery.ScheduleBackup)
		sensitive.DELETE(adminBackupsScheduleRoute, admindelivery.DeleteBackupSchedule)
		sensitive.DELETE(adminBackupsScheduleIDRoute, admindelivery.DeleteBackupSchedule)
		sensitive.DELETE("/backups/:id", systemdelivery.DeleteBackup)
		sensitive.GET("/backups/:id/download", systemdelivery.DownloadBackup)
		sensitive.POST("/backups/:id/restore", systemdelivery.RestoreBackup)
		sensitive.POST("/backups/:id/verify", systemdelivery.VerifyBackup)
		sensitive.GET("/backups/:id/progress", systemdelivery.GetBackupProgress)

		// Session & Security Management
		admin.GET("/security/sessions", authdelivery.GetActiveSessions)
		admin.GET("/security/sessions/stats", authdelivery.GetSessionStats)
		admin.POST("/security/sessions/:id/revoke", authdelivery.RevokeSession)
		admin.POST("/security/sessions/revoke-others", authdelivery.RevokeOtherSessions)
		admin.POST("/security/sessions/user/:userId/revoke-all", authdelivery.RevokeUserSessions)
		admin.POST("/security/sessions/:id/suspend", admindelivery.SuspendSession)
		admin.GET("/security/sessions/activity", admindelivery.GetSessionActivity)
		admin.GET("/security/logs", authdelivery.GetAdminSecurityLogs)
		admin.GET("/security/logs/users/:id", authdelivery.GetSecurityLogsForUser)
		admin.GET("/security/fingerprints", authdelivery.GetDeviceFingerprints)
		admin.POST("/security/fingerprints/block", authdelivery.BlockDeviceFingerprint)
		admin.POST("/security/fingerprints/:id/unblock", authdelivery.UnblockDeviceFingerprint)
		admin.GET("/security/roles", authdelivery.GetRolePermissions)

		// IP Whitelist (admin-only)
		sensitive.GET("/security/ip-whitelist", authdelivery.GetIPWhitelist)
		sensitive.POST("/security/ip-whitelist", authdelivery.AddIPToWhitelist)
		sensitive.GET("/security/ip-whitelist/settings", authdelivery.GetIPWhitelistSettings)
		sensitive.POST("/security/ip-whitelist/settings", authdelivery.UpdateIPWhitelistSettings)
		sensitive.GET("/security/ip-whitelist/blocked", authdelivery.GetBlockedAttempts)
		sensitive.POST("/security/ip-whitelist/bulk", authdelivery.BulkAddIPToWhitelist)
		sensitive.GET("/security/ip-whitelist/check", authdelivery.CheckIPWhitelist)
		sensitive.PATCH("/security/ip-whitelist/:id", authdelivery.UpdateIPWhitelistEntry)
		sensitive.DELETE("/security/ip-whitelist/:id", authdelivery.RemoveIPFromWhitelist)

		// General CRUD / Gamification
		// Achievements
		admin.GET("/achievements", gamificationdelivery.AdminGetAchievements)
		admin.POST("/achievements", gamificationdelivery.AdminCreateAchievement)
		admin.PATCH("/achievements/:id", gamificationdelivery.AdminUpdateAchievement)
		admin.DELETE("/achievements/:id", gamificationdelivery.AdminDeleteAchievement)

		// Rewards
		admin.GET("/rewards", gamificationdelivery.AdminGetRewards)
		admin.POST("/rewards", gamificationdelivery.AdminCreateReward)
		admin.PATCH("/rewards/:id", gamificationdelivery.AdminUpdateReward)
		admin.DELETE("/rewards/:id", gamificationdelivery.AdminDeleteReward)

		// Seasons
		admin.GET("/seasons", gamificationdelivery.AdminGetSeasons)
		admin.POST("/seasons", gamificationdelivery.AdminCreateSeason)
		admin.PATCH("/seasons/:id", gamificationdelivery.AdminUpdateSeason)
		admin.DELETE("/seasons/:id", gamificationdelivery.AdminDeleteSeason)

		// Coupons
		admin.GET("/coupons", marketingdelivery.AdminGetCoupons)
		admin.POST("/coupons", marketingdelivery.AdminCreateCoupon)
		admin.PATCH("/coupons/:id", marketingdelivery.AdminUpdateCoupon)
		admin.DELETE("/coupons/:id", marketingdelivery.AdminDeleteCoupon)

		// Challenges
		admin.GET("/challenges", gamificationdelivery.AdminGetChallenges)
		admin.POST("/challenges", gamificationdelivery.AdminCreateChallenge)
		admin.PATCH("/challenges/:id", gamificationdelivery.AdminUpdateChallenge)
		admin.DELETE("/challenges/:id", gamificationdelivery.AdminDeleteChallenge)

		// Blog
		admin.GET("/blog", contentdelivery.AdminGetBlog)
		admin.POST("/blog", contentdelivery.AdminCreateBlogPost)
		admin.PATCH("/blog/:id", contentdelivery.AdminUpdateBlogPost)
		admin.DELETE("/blog/:id", contentdelivery.AdminDeleteBlogPost)

		// Events
		admin.GET("/events", handlers.AdminGetEvents)
		admin.POST("/events", handlers.AdminCreateEvent)
		admin.PATCH("/events", handlers.AdminUpdateEvent)
		admin.DELETE("/events", handlers.AdminDeleteEvent)

		// Automations
		admin.GET("/automations", handlers.AdminGetAutomations)
		admin.POST("/automations", handlers.AdminCreateAutomation)
		admin.PATCH("/automations/:id", handlers.AdminUpdateAutomation)
		admin.DELETE("/automations/:id", handlers.AdminDeleteAutomation)

		// Campaigns
		admin.GET("/marketing/campaigns", marketingdelivery.AdminGetCampaigns)
		admin.POST("/marketing/campaigns", marketingdelivery.AdminCreateCampaign)
		admin.PATCH("/marketing/campaigns/:id", marketingdelivery.AdminUpdateCampaign)
		admin.DELETE("/marketing/campaigns/:id", marketingdelivery.AdminDeleteCampaign)

		// AB Testing
		admin.GET("/ab-testing", marketingdelivery.AdminGetABTests)
		admin.POST("/ab-testing", marketingdelivery.AdminCreateABTest)
		admin.PATCH("/ab-testing/:id", marketingdelivery.AdminUpdateABTest)
		admin.DELETE("/ab-testing/:id", marketingdelivery.AdminDeleteABTest)
		admin.GET("/ab-testing/:id/variant", marketingdelivery.AdminGetABVariant)
		admin.POST("/ab-testing/:id/track", marketingdelivery.AdminTrackABEvent)

		// Forum Categories
		admin.GET("/forum", contentdelivery.AdminGetForum)
		admin.GET("/forum-categories", contentdelivery.AdminGetForumCategories)
		admin.POST("/forum-categories", contentdelivery.AdminCreateForumCategory)

		// Books
		admin.GET("/books", contentdelivery.AdminGetBooks)

		// Learning Paths
		admin.GET("/learning-paths", admindelivery.AdminListLearningPaths)
		admin.POST("/learning-paths", admindelivery.AdminCreateLearningPath)
		admin.GET("/learning-paths/:id", admindelivery.AdminGetLearningPath)
		admin.PATCH("/learning-paths/:id", admindelivery.AdminUpdateLearningPath)
		admin.DELETE("/learning-paths/:id", admindelivery.AdminDeleteLearningPath)

		// Bank Questions
		admin.GET("/bank-questions", admindelivery.AdminListBankQuestions)
		admin.GET("/bank-questions/:id", admindelivery.AdminGetBankQuestion)
		admin.POST("/bank-questions", admindelivery.AdminCreateBankQuestion)
		admin.PATCH("/bank-questions/:id", admindelivery.AdminUpdateBankQuestion)
		admin.DELETE("/bank-questions/:id", admindelivery.AdminDeleteBankQuestion)

		// Resources
		admin.GET("/resources", handlers.AdminGetResources)
		admin.POST("/resources", handlers.AdminCreateResource)
		admin.PATCH("/resources", handlers.AdminUpdateResource)
		admin.DELETE("/resources", handlers.AdminDeleteResource)

		// Media Assets
		admin.GET("/media", admindelivery.AdminListMedia)
		admin.POST("/media", admindelivery.AdminCreateMedia)
		admin.GET("/media/tags", admindelivery.AdminGetMediaTags)

		// Upload
		admin.POST("/upload/presign", handlers.PresignUpload)
		admin.POST("/upload", handlers.Upload)
		admin.DELETE("/upload", handlers.DeleteUpload)
		admin.POST("/upload/chunked", handlers.UploadChunked)
		admin.PUT("/upload/chunked", handlers.UploadChunked)
		admin.PATCH("/upload/chunked", handlers.UploadChunked)
		admin.GET("/upload/chunked/:uploadId/status", handlers.GetUploadStatus)

		// Landing Page
		admin.GET("/landing", admindelivery.AdminListLandingSections)
		admin.POST("/landing", admindelivery.AdminUpsertLandingSection)

		// Affiliates
		admin.GET("/affiliates", marketingdelivery.AdminGetAffiliates)
		admin.POST("/affiliates", marketingdelivery.AdminCreateAffiliate)
		admin.GET("/affiliates/:id", marketingdelivery.AdminGetAffiliate)
		admin.PATCH("/affiliates/:id", marketingdelivery.AdminUpdateAffiliate)
		admin.DELETE("/affiliates/:id", marketingdelivery.AdminDeleteAffiliate)
		admin.GET("/affiliates/:id/referrals", marketingdelivery.AdminGetAffiliateReferrals)
		admin.POST("/affiliates/:id/pay", marketingdelivery.AdminPayAffiliate)
		admin.POST("/books", contentdelivery.AdminCreateBook)
		admin.PATCH("/books/:id", contentdelivery.AdminUpdateBook)
		admin.DELETE("/books/:id", contentdelivery.AdminDeleteBook)
		admin.GET("/books/views", admindelivery.AdminBookReviews)
		admin.GET("/books/reviews", admindelivery.AdminBookReviews)
		admin.DELETE("/books/reviews", admindelivery.AdminBookReviews)

		// User/Subject Admin Operations
		// User read/update: moderators can access; delete is admin-only
		admin.GET("/users", userdelivery.GetUsers)
		admin.POST("/users", userdelivery.CreateUser)
		admin.GET(adminUserIDRoute, userdelivery.GetUserByID)
		// Permission edits are security-sensitive; role membership alone must not
		// allow an administrator to alter another account's effective access.
		admin.PATCH(adminUserIDRoute, middleware.PermissionRequired(models.PermUsersManage), userdelivery.UpdateUser)
		sensitive.DELETE(adminUserIDRoute, userdelivery.DeleteUser) // ADMIN-only
		admin.GET("/users/analytics", analyticsdelivery.AdminUsersAnalytics)
		admin.GET("/users/filter-options", analyticsdelivery.AdminUsersFilterOptions)
		admin.GET("/users/:id/enrollments", userdelivery.GetUserEnrollments)
		admin.POST("/users/:id/enrollments", userdelivery.AdminEnrollUser)
		admin.GET("/users/:id/orders", userdelivery.GetUserOrders)
		admin.GET("/users/:id/notifications", admindelivery.GetUserNotifications)
		admin.POST("/users/:id/notifications", admindelivery.SendUserNotification)
		admin.GET("/users/:id/login-attempts", authdelivery.GetUserLoginAttempts)
		admin.GET("/users/:id/video-engagement", userdelivery.GetUserVideoEngagement)
		admin.GET("/users/:id/wallet/transactions", paymentdelivery.GetUserWalletTransactions)
		admin.GET("/search/users", searchdelivery.SearchUsers)
		admin.POST("/users/search", searchdelivery.SearchUsers)

		// Bulk User Operations
		admin.POST("/users/bulk-create", userdelivery.BulkCreateUsers)
		admin.POST("/users/bulk-delete", userdelivery.BulkDeleteUsers)
		admin.POST("/users/bulk-notify", notificationdelivery.AdminBulkSendMessage)
		admin.POST("/users/bulk/suspend", userdelivery.BulkSuspendUsers)
		admin.POST("/users/bulk/activate", userdelivery.BulkActivateUsers)
		admin.POST("/users/bulk/restore", userdelivery.BulkRestoreUsers)
		admin.POST("/users/bulk/assign-role", userdelivery.BulkAssignRole)

		// User Actions
		admin.POST("/users/:id/ban", userdelivery.BanUser)
		admin.POST("/users/:id/suspend", userdelivery.SuspendUser)
		admin.POST("/users/:id/role", userdelivery.ChangeUserRole)
		admin.POST("/users/:id/password-reset", userdelivery.SendPasswordReset)
		admin.POST("/users/:id/activate", userdelivery.ActivateUser)
		admin.POST("/users/:id/verify-email", userdelivery.VerifyUserEmail)
		admin.POST("/users/:id/verify-phone", userdelivery.VerifyUserPhone)
		admin.POST("/users/:id/restore", userdelivery.RestoreUser)
		admin.POST("/users/:id/assign-role", userdelivery.AssignRole)
		admin.POST("/users/:id/permissions/add", userdelivery.AddPermission)
		admin.POST("/users/:id/permissions/remove", userdelivery.RemovePermission)
		admin.GET("/users/:id/permissions", analyticsdelivery.GetUserPermissions)
		admin.GET("/users/:id/sessions", analyticsdelivery.GetUserSessions)
		admin.POST("/users/:id/sessions/:sessionId/terminate", analyticsdelivery.TerminateSession)
		admin.POST("/users/:id/sessions/terminate-all", analyticsdelivery.TerminateAllSessions)
		admin.GET("/users/:id/audit-logs", analyticsdelivery.GetUserAuditLogs)
		admin.POST("/users/:id/send-activation-link", userdelivery.SendActivationLink)

		// Administrators are a first-class, server-filtered resource.  Keeping
		// this separate from the broad users endpoint prevents clients from
		// downloading a user directory and filtering privileged accounts locally.
		admin.GET("/admins", middleware.PermissionRequired(models.PermUsersManage), userdelivery.ListAdmins)

		// Parent Management
		admin.GET("/parents/statistics", userdelivery.GetParentStatistics)
		admin.GET("/users/:id/students", userdelivery.GetParentStudents)
		admin.POST("/users/:id/students/link", userdelivery.LinkStudentToParent)
		admin.DELETE("/users/:id/students/unlink", userdelivery.UnlinkStudentFromParent)

		// Subject
		admin.GET(adminSubjectsRoute, coursedelivery.GetSubjects)
		admin.POST(adminSubjectsRoute, coursedelivery.CreateSubject)
		admin.PATCH(adminSubjectsRoute, coursedelivery.UpdateSubject)
		admin.DELETE(adminSubjectsRoute, coursedelivery.DeleteSubject)

		// Course aliases for Admin panel compatibility.
		// GET and POST /courses are owned by CourseRESTHandler in hexagonal_routes.go.
		admin.PATCH("/courses", coursedelivery.UpdateSubject)
		admin.DELETE("/courses", coursedelivery.DeleteSubject)
		admin.GET("/courses/:id/curriculum", coursedelivery.GetSubjectCurriculum)
		admin.PUT("/courses/:id/curriculum", coursedelivery.UpdateCourseCurriculum)
		admin.PATCH("/courses/:id/curriculum", coursedelivery.UpdateCourseCurriculum)
		admin.POST("/courses/duplicate", coursedelivery.DuplicateCourse)
		admin.POST("/courses/batch", coursedelivery.BatchCourseAction)

		// Curriculum
		admin.PATCH("/subjects/:id/curriculum", coursedelivery.UpdateCourseCurriculum)
		admin.GET("/subjects/:id/curriculum", coursedelivery.GetSubjectCurriculum)

		// Course Students (view list of enrolled students)
		admin.GET("/courses/:id/students", handlers.GetCourseStudents)

		// Manual Enroll
		admin.GET("/courses/enrollments", handlers.GetCourseEnrollments)
		admin.POST("/courses/enroll", handlers.ManualEnroll)
		admin.POST("/courses/unenroll", handlers.UnenrollUser)
		admin.POST("/courses/lessons/attachments", coursedelivery.AddLessonAttachment)

		// Notifications Broadcast
		admin.POST("/notifications/broadcast", notificationdelivery.SendNotificationBroadcast)
		admin.POST("/notifications/schedule", notificationdelivery.ScheduleNotificationBroadcast)
		admin.POST("/notifications/broadcast/:id/cancel", notificationdelivery.CancelScheduledBroadcast)
		admin.POST("/notifications/broadcast/:id/retry", notificationdelivery.RetryFailedNotifications)
		admin.GET("/broadcasts", notificationdelivery.GetBroadcasts)
		admin.GET("/notifications/stats", notificationdelivery.GetNotificationStats)
		admin.POST("/notifications/push", notificationdelivery.SendPushNotification)

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
		admin.GET("/search/content", searchdelivery.SearchContent)

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
		admin.GET("/courses/export", admindelivery.AdminCourseAction)

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
		admin.GET("/courses/lessons/:lessonId/interactive-questions", handlers.GetInteractiveQuestions)
		admin.POST("/courses/lessons/:lessonId/interactive-questions", handlers.CreateInteractiveQuestion)
		admin.GET("/interactive-questions/:id", handlers.GetInteractiveQuestion)
		admin.PATCH("/interactive-questions", handlers.UpdateInteractiveQuestion)
		admin.DELETE("/interactive-questions/:id", handlers.DeleteInteractiveQuestion)

		// Course Pricing, Versioning and Bundles are registered in
		// hexagonal_routes.go via CourseRESTHandler.

		// -------------------------------
		// Lessons Management
		// -------------------------------
		admin.GET("/lessons", admindelivery.AdminListLessons)
		admin.GET("/lessons/:id", admindelivery.AdminGetLesson)
		admin.POST("/lessons", admindelivery.AdminCreateLesson)
		admin.PATCH("/lessons/:id", admindelivery.AdminUpdateLesson)
		admin.DELETE("/lessons/:id", admindelivery.AdminDeleteLesson)

		// -------------------------------
		// Payments Management
		// -------------------------------
		admin.GET("/payments", paymentdelivery.GetAdminPayments)
		admin.POST("/payments/refund", paymentdelivery.AdminRefundPayment)

		// Dunning — subscription payment failure tracking
		admin.GET("/dunning", admindelivery.AdminListDunning)

		// -------------------------------
		// Exams Management
		// -------------------------------
		admin.GET("/exams", handlers.GetExams)
		admin.POST("/exams", handlers.CreateExam)
		admin.PATCH("/exams", handlers.UpdateExam)
		admin.DELETE("/exams", handlers.DeleteExam)

		// -------------------------------
		// Refunds Management
		// -------------------------------
		admin.GET("/refunds", admindelivery.AdminListRefunds)
		admin.GET("/refunds/:id", admindelivery.AdminGetRefund)
		admin.POST("/refunds/:id/approve", admindelivery.AdminApproveRefund)
		admin.POST("/refunds/:id/reject", admindelivery.AdminRejectRefund)
		admin.POST("/refunds/:id/process", admindelivery.AdminProcessRefund)

		// -------------------------------
		// Tax Management
		// -------------------------------
		admin.GET("/taxes", admindelivery.AdminListTaxRates)
		admin.GET("/taxes/:id", admindelivery.AdminGetTaxRate)
		admin.POST("/taxes", admindelivery.AdminCreateTaxRate)
		admin.PATCH("/taxes/:id", admindelivery.AdminUpdateTaxRate)
		admin.DELETE("/taxes/:id", admindelivery.AdminDeleteTaxRate)

		// -------------------------------
		// Badges Management
		// -------------------------------
		admin.GET("/badges", admindelivery.AdminListBadges)
		admin.GET("/badges/:id", admindelivery.AdminGetBadge)
		admin.POST("/badges", admindelivery.AdminCreateBadge)
		admin.PATCH("/badges/:id", admindelivery.AdminUpdateBadge)
		admin.DELETE("/badges/:id", admindelivery.AdminDeleteBadge)

		// -------------------------------
		// Attendance Management
		// -------------------------------
		admin.GET("/attendance", admindelivery.AdminListAttendance)
		admin.GET("/attendance/stats", admindelivery.AdminGetAttendanceStats)
		admin.POST("/attendance", admindelivery.AdminCreateAttendance)
		admin.PATCH("/attendance/:id", admindelivery.AdminUpdateAttendance)

		// -------------------------------
		// CMS Pages Management
		// -------------------------------
		admin.GET("/cms/pages", admindelivery.AdminListCMSPages)
		admin.GET("/cms/pages/:id", admindelivery.AdminGetCMSPage)
		admin.POST("/cms/pages", admindelivery.AdminCreateCMSPage)
		admin.PATCH("/cms/pages/:id", admindelivery.AdminUpdateCMSPage)
		admin.DELETE("/cms/pages/:id", admindelivery.AdminDeleteCMSPage)

		// -------------------------------
		// Integrations Management
		// -------------------------------
		admin.GET("/integrations", admindelivery.AdminListIntegrations)
		admin.GET("/integrations/:id", admindelivery.AdminGetIntegration)
		admin.POST("/integrations", admindelivery.AdminCreateIntegration)
		admin.PATCH("/integrations/:id", admindelivery.AdminUpdateIntegration)
		admin.DELETE("/integrations/:id", admindelivery.AdminDeleteIntegration)
		admin.POST("/integrations/:id/test", admindelivery.AdminTestIntegration)

		// -------------------------------
		// Roles & Permissions Management
		// -------------------------------
		admin.GET("/roles", systemdelivery.AdminListRoles)
		admin.GET("/roles/:id", systemdelivery.AdminGetRole)
		admin.POST("/roles", systemdelivery.AdminCreateRole)
		admin.PATCH("/roles/:id", systemdelivery.AdminUpdateRole)
		admin.DELETE("/roles/:id", systemdelivery.AdminDeleteRole)

		// Permissions
		admin.GET("/permissions", systemdelivery.AdminListPermissions)
		admin.GET("/permissions/groups", systemdelivery.AdminGetPermissionGroups)

		// Role Permissions
		admin.GET("/roles/:id/permissions", systemdelivery.AdminGetRolePermissions)
		admin.POST("/roles/:id/permissions/assign", systemdelivery.AdminAssignPermissionsToRole)
		admin.POST("/roles/:id/permissions/remove", systemdelivery.AdminRemovePermissionsFromRole)
		admin.PUT("/roles/:id/permissions/replace", systemdelivery.AdminReplaceRolePermissions)

		// Role Users
		admin.GET("/roles/:id/users", systemdelivery.AdminGetUsersByRole)
		admin.POST("/roles/:id/users/assign", systemdelivery.AdminAssignUsersToRole)
		admin.POST("/roles/:id/users/remove", systemdelivery.AdminRemoveUsersFromRole)
	}
}
