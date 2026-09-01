package api

import (
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
//
// Route registration is split across several files in this package (all
// sharing package api), grouped by area: admin_routes_dashboard.go,
// admin_routes_content.go, admin_routes_support_backups.go,
// admin_routes_security.go, admin_routes_gamification.go,
// admin_routes_users.go, admin_routes_courses.go,
// admin_routes_notifications_reports.go, admin_routes_misc.go,
// admin_routes_lessons_payments.go, admin_routes_badges_cms.go and
// admin_routes_roles.go. Each registers its routes on the same `admin`
// (AdminOrModerator) and `sensitive` (AdminRequired) groups constructed
// here, called in the same order the routes were originally registered in
// so route-matching behavior is unchanged.
func SetupAdminRoutes(router *gin.Engine) {
	admin := router.Group("/api/v1/admin")
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

	// ---------------------------------------------------------------
	// Sensitive operations: ADMIN and SUPER_ADMIN only.
	// Created as a sub-group of `admin` so it inherits Auth() +
	// AdminOrModerator() + StrictRBAC() automatically; we then layer
	// AdminRequired() on top to ALSO block MODERATOR.
	// ---------------------------------------------------------------
	sensitive := admin.Group("")
	sensitive.Use(middleware.AdminRequired()) // additional check: blocks MODERATOR

	registerAdminDashboardRoutes(admin, sensitive)
	registerAdminContentRoutes(admin)
	registerAdminSupportBackupRoutes(admin, sensitive)
	registerAdminSecurityRoutes(admin, sensitive)
	registerAdminGamificationRoutes(admin)
	registerAdminUserRoutes(admin, sensitive)
	registerAdminCourseRoutes(admin)
	registerAdminNotificationReportRoutes(admin)
	registerAdminMiscRoutes(admin, sensitive)
	registerAdminLessonPaymentRoutes(admin)
	registerAdminBadgeCMSRoutes(admin)
	registerAdminRoleRoutes(admin)
	registerAdminAntiCheatRoutes(admin)
}
