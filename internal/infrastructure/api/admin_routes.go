package api

import (
	models "thanawy-backend/internal/domain/common"

	admindelivery "thanawy-backend/internal/infrastructure/api/handlers/admin"

	handlers "thanawy-backend/internal/infrastructure/api/handlers/protected"

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
		admin.GET("/marketing/campaigns", handlers.AdminGetCampaigns)
		admin.POST("/marketing/campaigns", handlers.AdminCreateCampaign)
		admin.PATCH("/marketing/campaigns/:id", handlers.AdminUpdateCampaign)
		admin.DELETE("/marketing/campaigns/:id", handlers.AdminDeleteCampaign)

		// AB Testing
		admin.GET("/ab-testing", handlers.AdminGetABTests)
		admin.POST("/ab-testing", handlers.AdminCreateABTest)
		admin.PATCH("/ab-testing/:id", handlers.AdminUpdateABTest)
		admin.DELETE("/ab-testing/:id", handlers.AdminDeleteABTest)
		admin.GET("/ab-testing/:id/variant", handlers.AdminGetABVariant)
		admin.POST("/ab-testing/:id/track", handlers.AdminTrackABEvent)

		// Forum Categories
		admin.GET("/forum", handlers.AdminGetForum)
		admin.GET("/forum-categories", handlers.AdminGetForumCategories)
		admin.POST("/forum-categories", handlers.AdminCreateForumCategory)

		// Books
		admin.GET("/books", handlers.AdminGetBooks)

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
		admin.GET("/books/views", admindelivery.AdminBookReviews)
		admin.GET("/books/reviews", admindelivery.AdminBookReviews)
		admin.DELETE("/books/reviews", admindelivery.AdminBookReviews)

		// User/Subject Admin Operations
		// User read/update: moderators can access; delete is admin-only
		admin.GET("/users", handlers.GetUsers)
		admin.POST("/users", handlers.CreateUser)
		admin.GET(adminUserIDRoute, handlers.GetUserByID)
		// Permission edits are security-sensitive; role membership alone must not
		// allow an administrator to alter another account's effective access.
		admin.PATCH(adminUserIDRoute, middleware.PermissionRequired(models.PermUsersManage), handlers.UpdateUser)
		sensitive.DELETE(adminUserIDRoute, handlers.DeleteUser) // ADMIN-only
		admin.GET("/users/analytics", handlers.AdminUsersAnalytics)
		admin.GET("/users/filter-options", handlers.AdminUsersFilterOptions)
		admin.GET("/users/:id/enrollments", handlers.GetUserEnrollments)
		admin.POST("/users/:id/enrollments", handlers.AdminEnrollUser)
		admin.GET("/users/:id/orders", handlers.GetUserOrders)
		admin.GET("/users/:id/notifications", admindelivery.GetUserNotifications)
		admin.POST("/users/:id/notifications", admindelivery.SendUserNotification)
		admin.GET("/users/:id/login-attempts", handlers.GetUserLoginAttempts)
		admin.GET("/users/:id/video-engagement", handlers.GetUserVideoEngagement)
		admin.GET("/users/:id/wallet/transactions", handlers.GetUserWalletTransactions)
		admin.GET("/search/users", handlers.SearchUsers)
		admin.POST("/users/search", handlers.SearchUsers)

		// Bulk User Operations
		admin.POST("/users/bulk-create", handlers.BulkCreateUsers)
		admin.POST("/users/bulk-delete", handlers.BulkDeleteUsers)
		admin.POST("/users/bulk-notify", handlers.AdminBulkSendMessage)
		admin.POST("/users/bulk/suspend", handlers.BulkSuspendUsers)
		admin.POST("/users/bulk/activate", handlers.BulkActivateUsers)
		admin.POST("/users/bulk/restore", handlers.BulkRestoreUsers)
		admin.POST("/users/bulk/assign-role", handlers.BulkAssignRole)

		// User Actions
		admin.POST("/users/:id/ban", handlers.BanUser)
		admin.POST("/users/:id/suspend", handlers.SuspendUser)
		admin.POST("/users/:id/role", handlers.ChangeUserRole)
		admin.POST("/users/:id/password-reset", handlers.SendPasswordReset)
		admin.POST("/users/:id/activate", handlers.ActivateUser)
		admin.POST("/users/:id/verify-email", handlers.VerifyUserEmail)
		admin.POST("/users/:id/verify-phone", handlers.VerifyUserPhone)
		admin.POST("/users/:id/restore", handlers.RestoreUser)
		admin.POST("/users/:id/assign-role", handlers.AssignRole)
		admin.POST("/users/:id/permissions/add", handlers.AddPermission)
		admin.POST("/users/:id/permissions/remove", handlers.RemovePermission)
		admin.GET("/users/:id/permissions", handlers.GetUserPermissions)
		admin.GET("/users/:id/sessions", handlers.GetUserSessions)
		admin.POST("/users/:id/sessions/:sessionId/terminate", handlers.TerminateSession)
		admin.POST("/users/:id/sessions/terminate-all", handlers.TerminateAllSessions)
		admin.GET("/users/:id/audit-logs", handlers.GetUserAuditLogs)
		admin.POST("/users/:id/send-activation-link", handlers.SendActivationLink)

		// Administrators are a first-class, server-filtered resource.  Keeping
		// this separate from the broad users endpoint prevents clients from
		// downloading a user directory and filtering privileged accounts locally.
		admin.GET("/admins", middleware.PermissionRequired(models.PermUsersManage), handlers.ListAdmins)

		// Parent Management
		admin.GET("/parents/statistics", handlers.GetParentStatistics)
		admin.GET("/users/:id/students", handlers.GetParentStudents)
		admin.POST("/users/:id/students/link", handlers.LinkStudentToParent)
		admin.DELETE("/users/:id/students/unlink", handlers.UnlinkStudentFromParent)

		// Subject
		admin.GET(adminSubjectsRoute, handlers.GetSubjects)
		admin.POST(adminSubjectsRoute, handlers.CreateSubject)
		admin.PATCH(adminSubjectsRoute, handlers.UpdateSubject)
		admin.DELETE(adminSubjectsRoute, handlers.DeleteSubject)

		// Course aliases for Admin panel compatibility.
		// GET and POST /courses are owned by CourseRESTHandler in hexagonal_routes.go.
		admin.PATCH("/courses", handlers.UpdateSubject)
		admin.DELETE("/courses", handlers.DeleteSubject)
		admin.GET("/courses/:id/curriculum", handlers.GetSubjectCurriculum)
		admin.PUT("/courses/:id/curriculum", handlers.UpdateCourseCurriculum)
		admin.PATCH("/courses/:id/curriculum", handlers.UpdateCourseCurriculum)
		admin.POST("/courses/duplicate", handlers.DuplicateCourse)
		admin.POST("/courses/batch", handlers.BatchCourseAction)

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
		admin.GET("/payments", handlers.GetAdminPayments)
		admin.POST("/payments/refund", handlers.AdminRefundPayment)

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
		admin.GET("/roles", handlers.AdminListRoles)
		admin.GET("/roles/:id", handlers.AdminGetRole)
		admin.POST("/roles", handlers.AdminCreateRole)
		admin.PATCH("/roles/:id", handlers.AdminUpdateRole)
		admin.DELETE("/roles/:id", handlers.AdminDeleteRole)

		// Permissions
		admin.GET("/permissions", handlers.AdminListPermissions)
		admin.GET("/permissions/groups", handlers.AdminGetPermissionGroups)

		// Role Permissions
		admin.GET("/roles/:id/permissions", handlers.AdminGetRolePermissions)
		admin.POST("/roles/:id/permissions/assign", handlers.AdminAssignPermissionsToRole)
		admin.POST("/roles/:id/permissions/remove", handlers.AdminRemovePermissionsFromRole)
		admin.PUT("/roles/:id/permissions/replace", handlers.AdminReplaceRolePermissions)

		// Role Users
		admin.GET("/roles/:id/users", handlers.AdminGetUsersByRole)
		admin.POST("/roles/:id/users/assign", handlers.AdminAssignUsersToRole)
		admin.POST("/roles/:id/users/remove", handlers.AdminRemoveUsersFromRole)
	}
}
