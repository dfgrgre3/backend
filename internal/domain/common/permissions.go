package models

import "strings"

// Role Levels for Hierarchy Comparison
const (
	RoleLevelStudent    = 1
	RoleLevelParent     = 2
	RoleLevelTeacher    = 3
	RoleLevelSupport    = 4
	RoleLevelModerator  = 5
	RoleLevelAdmin      = 6
	RoleLevelSuperAdmin = 7
)

// ─────────────────────────────────────────────
//  Permission Constants
//  Convention: Module:Action or Module:Submodule:Action
//  Wildcards: * for all, manage for full CRUD
// ─────────────────────────────────────────────

// ── Super Permissions ────────────────────────
const (
	PermAdminBypass    = "admin:bypass"    // Super Admin bypasses all checks
	PermSystemManage   = "system:manage"   // Full system management
	PermSystemSettings = "system:settings" // View/Edit system settings
)

// ── Dashboard & Analytics ────────────────────
const (
	PermDashboardView = "dashboard:view"
	PermAnalyticsView = "analytics:view"
	PermReportsView   = "reports:view"
	PermReportsManage = "reports:manage"
	PermAuditLogsView = "audit_logs:view"
)

// ── Dashboard Widget-Level Permissions ───────
// Each admin dashboard widget is gated independently so a role can be granted
// operational visibility without financial or system-internal visibility.
// A `dashboard:manage` grant covers every key below via PermissionGrantMatches.
const (
	PermDashboardAccess               = "dashboard:access"
	PermDashboardViewKPIs             = "dashboard:view_kpis"
	PermDashboardViewLearningMetrics  = "dashboard:view_learning_metrics"
	PermDashboardViewFinancialMetrics = "dashboard:view_financial_metrics"
	PermDashboardViewSupportMetrics   = "dashboard:view_support_metrics"
	PermDashboardViewContentMetrics   = "dashboard:view_content_metrics"
	PermDashboardViewSystemHealth     = "dashboard:view_system_health"
	PermDashboardViewRecentActivity   = "dashboard:view_recent_activity"
	PermDashboardViewPendingItems     = "dashboard:view_pending_items"
	PermDashboardViewAlerts           = "dashboard:view_alerts"
	PermDashboardViewTopCourses       = "dashboard:view_top_courses"
	PermDashboardViewExports          = "dashboard:view_exports"
	PermDashboardViewSensitive        = "dashboard:view_sensitive_metrics"
	PermDashboardRefreshCache         = "dashboard:refresh_cache"
	PermDashboardExport               = "dashboard:export"
	PermDashboardSaveFilters          = "dashboard:save_filters"
	PermDashboardDeleteSavedFilters   = "dashboard:delete_saved_filters"
	PermDashboardApplySavedFilters    = "dashboard:apply_saved_filters"
	PermDashboardAcknowledgeAlerts    = "dashboard:acknowledge_alerts"
	PermDashboardManage               = "dashboard:manage"
)

// DashboardWidgetPermissions lists every widget-level dashboard permission.
func DashboardWidgetPermissions() []string {
	return []string{
		PermDashboardAccess,
		PermDashboardViewKPIs,
		PermDashboardViewLearningMetrics,
		PermDashboardViewFinancialMetrics,
		PermDashboardViewSupportMetrics,
		PermDashboardViewContentMetrics,
		PermDashboardViewSystemHealth,
		PermDashboardViewRecentActivity,
		PermDashboardViewPendingItems,
		PermDashboardViewAlerts,
		PermDashboardViewTopCourses,
		PermDashboardViewExports,
		PermDashboardViewSensitive,
		PermDashboardRefreshCache,
		PermDashboardExport,
		PermDashboardSaveFilters,
		PermDashboardDeleteSavedFilters,
		PermDashboardApplySavedFilters,
		PermDashboardAcknowledgeAlerts,
	}
}

// ── Users Management ─────────────────────────
const (
	PermUsersView              = "users:view"
	PermUsersCreate            = "users:create"
	PermUsersUpdate            = "users:update"
	PermUsersDelete            = "users:delete"
	PermUsersManage            = "users:manage" // Full CRUD + roles
	PermUsersImpersonate       = "users:impersonate"
	PermUsersExport            = "users:export"
	PermUsersImport            = "users:import"
	PermUsersSendNotifications = "users:send:notifications"
	PermStudentsView           = "students:view"
	PermStudentsManage         = "students:manage"
	PermTeachersView           = "teachers:view"
	PermTeachersManage         = "teachers:manage"
	PermParentsView            = "parents:view"   // NEW
	PermParentsManage          = "parents:manage" // NEW
)

// ── Content Management ──────────────────────
const (
	PermSubjectsView       = "subjects:view"
	PermSubjectsCreate     = "subjects:create"
	PermSubjectsUpdate     = "subjects:update"
	PermSubjectsDelete     = "subjects:delete"
	PermSubjectsManage     = "subjects:manage"
	PermSubjectsPublish    = "subjects:publish" // NEW
	PermSubjectsApprove    = "subjects:approve" // NEW
	PermOwnSubjectsManage  = "own_subjects:manage"
	PermBooksView          = "books:view"
	PermBooksCreate        = "books:create"
	PermBooksUpdate        = "books:update"
	PermBooksDelete        = "books:delete"
	PermBooksManage        = "books:manage"
	PermBooksPublish       = "books:publish" // NEW
	PermOwnBooksManage     = "own_books:manage"
	PermResourcesView      = "resources:view"
	PermResourcesManage    = "resources:manage"
	PermResourcesPublish   = "resources:publish" // NEW
	PermOwnResourcesManage = "own_resources:manage"
)

// ── Educational ─────────────────────────────
const (
	PermExamsView           = "exams:view"
	PermExamsCreate         = "exams:create"
	PermExamsUpdate         = "exams:update"
	PermExamsDelete         = "exams:delete"
	PermExamsManage         = "exams:manage"
	PermExamsApprove        = "exams:approve" // NEW
	PermExamsPublish        = "exams:publish" // NEW
	PermOwnExamsManage      = "own_exams:manage"
	PermChallengesView      = "challenges:view"
	PermChallengesManage    = "challenges:manage"
	PermOwnChallengesManage = "own_challenges:manage"
	PermContestsView        = "contests:view"
	PermContestsManage      = "contests:manage"
)

// ── Community ───────────────────────────────
const (
	PermBlogView            = "blog:view"
	PermBlogCreate          = "blog:create"
	PermBlogUpdate          = "blog:update"
	PermBlogDelete          = "blog:delete"
	PermBlogManage          = "blog:manage"
	PermBlogPublish         = "blog:publish"
	PermForumView           = "forum:view"
	PermForumCreate         = "forum:create"
	PermForumUpdate         = "forum:update"
	PermForumDelete         = "forum:delete"
	PermForumModerate       = "forum:moderate"
	PermForumManage         = "forum:manage"
	PermCommentsView        = "comments:view"
	PermCommentsCreate      = "comments:create"
	PermCommentsModerate    = "comments:moderate"
	PermEventsView          = "events:view"
	PermEventsManage        = "events:manage"
	PermAnnouncementsView   = "announcements:view"
	PermAnnouncementsManage = "announcements:manage"
)

// ── Support (NEW Module) ────────────────────
const (
	PermTicketsView    = "tickets:view"
	PermTicketsCreate  = "tickets:create"
	PermTicketsUpdate  = "tickets:update"
	PermTicketsManage  = "tickets:manage" // Assign, escalate, close
	PermTicketsResolve = "tickets:resolve"
	PermFAQsManage     = "faqs:manage"
)

// ── Parent Dashboard (NEW Module) ───────────
const (
	PermChildrenView        = "children:view"
	PermChildrenGrades      = "children:grades"
	PermChildrenProgress    = "children:progress"
	PermChildrenAttendance  = "children:attendance"
	PermChildrenCommunicate = "children:communicate"
	PermChildrenPayment     = "children:payment"
)

// ── Misc ────────────────────────────────────
const (
	// PermPermissionsCustom is a sentinel marker stored alongside a user's
	// permissions array. When present, GetEffectivePermissions() returns ONLY
	// the stored permissions without merging role defaults — enabling
	// restrictive per-user customization. The sentinel itself is not a real
	// permission and never matches any permission check.
	PermPermissionsCustom = "permissions:custom"

	PermAchievementsView    = "achievements:view"
	PermAchievementsManage  = "achievements:manage"
	PermRewardsView         = "rewards:view"
	PermRewardsManage       = "rewards:manage"
	PermAiManage            = "ai:manage"
	PermAiUsage             = "ai:usage"
	PermLiveMonitorView     = "live_monitor:view"
	PermMarketingView       = "marketing:view"
	PermMarketingManage     = "marketing:manage"
	PermAbTestingView       = "ab_testing:view"
	PermSettingsView        = "settings:view"
	PermSeasonsView         = "seasons:view"
	PermSeasonsManage       = "seasons:manage"
	PermNotificationsSend   = "notifications:send"   // NEW
	PermNotificationsManage = "notifications:manage" // NEW
)

// ─────────────────────────────────────────────
//  Role Hierarchy Map
// ─────────────────────────────────────────────

// GetRoleLevel returns the hierarchy level for a role.
// Higher number = more privileges.
func GetRoleLevel(role UserRole) int {
	switch role {
	case RoleSuperAdmin:
		return RoleLevelSuperAdmin
	case RoleAdmin:
		return RoleLevelAdmin
	case RoleModerator:
		return RoleLevelModerator
	case RoleSupport:
		return RoleLevelSupport
	case RoleTeacher:
		return RoleLevelTeacher
	case RoleParent:
		return RoleLevelParent
	case RoleStudent:
		return RoleLevelStudent
	default:
		return 0
	}
}

// IsRoleAtLeast checks if the given role is at or above the minimum level.
func IsRoleAtLeast(role UserRole, minLevel int) bool {
	return GetRoleLevel(role) >= minLevel
}

// ─────────────────────────────────────────────
//  Default Permissions Per Role
// ─────────────────────────────────────────────

// GetDefaultPermissions returns the default permission set for a given role.
func GetDefaultPermissions(role UserRole) []string {
	switch role {
	case RoleSuperAdmin:
		return getSuperAdminPermissions()
	case RoleAdmin:
		return getAdminPermissions()
	case RoleModerator:
		return getModeratorPermissions()
	case RoleSupport:
		return getSupportPermissions()
	case RoleTeacher:
		return getTeacherPermissions()
	case RoleParent:
		return getParentPermissions()
	case RoleStudent:
		return getStudentPermissions()
	default:
		return []string{}
	}
}

func getSuperAdminPermissions() []string {
	return []string{PermAdminBypass}
}

func getAdminPermissions() []string {
	return append([]string{
		PermAdminBypass,
		PermDashboardView, PermAnalyticsView, PermReportsView, PermReportsManage, PermAuditLogsView,
		PermUsersView, PermUsersCreate, PermUsersUpdate, PermUsersDelete, PermUsersManage, PermUsersImpersonate, PermUsersExport, PermUsersImport,
		PermStudentsView, PermStudentsManage,
		PermTeachersView, PermTeachersManage,
		PermParentsView, PermParentsManage,
		PermSubjectsView, PermSubjectsCreate, PermSubjectsUpdate, PermSubjectsDelete, PermSubjectsManage, PermSubjectsPublish, PermSubjectsApprove,
		PermBooksView, PermBooksCreate, PermBooksUpdate, PermBooksDelete, PermBooksManage, PermBooksPublish,
		PermResourcesView, PermResourcesManage, PermResourcesPublish,
		PermExamsView, PermExamsCreate, PermExamsUpdate, PermExamsDelete, PermExamsManage, PermExamsApprove, PermExamsPublish,
		PermChallengesView, PermChallengesManage,
		PermContestsView, PermContestsManage,
		PermBlogView, PermBlogCreate, PermBlogUpdate, PermBlogDelete, PermBlogManage, PermBlogPublish,
		PermForumView, PermForumCreate, PermForumUpdate, PermForumDelete, PermForumModerate, PermForumManage,
		PermCommentsView, PermCommentsCreate, PermCommentsModerate,
		PermEventsView, PermEventsManage,
		PermAnnouncementsView, PermAnnouncementsManage,
		PermTicketsView, PermTicketsCreate, PermTicketsUpdate, PermTicketsManage, PermTicketsResolve, PermFAQsManage,
		PermAchievementsView, PermAchievementsManage,
		PermRewardsView, PermRewardsManage,
		PermAiManage, PermAiUsage,
		PermLiveMonitorView, PermMarketingView, PermMarketingManage,
		PermAbTestingView, PermSettingsView,
		PermSeasonsView, PermSeasonsManage,
		PermSystemManage, PermSystemSettings,
		PermNotificationsSend, PermNotificationsManage,
	}, DashboardWidgetPermissions()...)
}

func getModeratorPermissions() []string {
	return []string{
		PermDashboardView, PermAnalyticsView, PermReportsView,
		// Operational dashboard visibility without financial or system internals.
		PermDashboardAccess, PermDashboardViewKPIs,
		PermDashboardViewLearningMetrics, PermDashboardViewContentMetrics,
		PermDashboardViewSupportMetrics, PermDashboardViewRecentActivity,
		PermDashboardViewPendingItems, PermDashboardViewAlerts,
		PermDashboardViewTopCourses, PermDashboardAcknowledgeAlerts,
		PermDashboardSaveFilters, PermDashboardDeleteSavedFilters, PermDashboardApplySavedFilters,
		PermUsersView, PermStudentsView, PermTeachersView, PermParentsView,
		PermSubjectsView, PermSubjectsApprove,
		PermBooksView, PermBooksPublish,
		PermResourcesView, PermResourcesPublish,
		PermExamsView, PermExamsApprove, PermExamsPublish,
		PermChallengesView,
		PermContestsView,
		PermBlogView, PermBlogCreate, PermBlogUpdate, PermBlogDelete, PermBlogPublish,
		PermForumView, PermForumCreate, PermForumUpdate, PermForumDelete, PermForumModerate,
		PermCommentsView, PermCommentsModerate,
		PermEventsView, PermEventsManage,
		PermAnnouncementsView, PermAnnouncementsManage,
		PermTicketsView, PermTicketsManage, PermTicketsResolve,
		PermAchievementsView,
		PermRewardsView,
		PermLiveMonitorView, PermMarketingView,
		PermSettingsView,
		PermNotificationsSend, PermNotificationsManage,
	}
}

func getSupportPermissions() []string {
	return []string{
		PermDashboardView,
		// Support sees the support queue widgets only.
		PermDashboardAccess, PermDashboardViewSupportMetrics,
		PermDashboardViewPendingItems, PermDashboardViewAlerts,
		PermDashboardAcknowledgeAlerts, PermDashboardApplySavedFilters,
		PermUsersView, PermStudentsView, PermTeachersView, PermParentsView,
		PermTicketsView, PermTicketsCreate, PermTicketsUpdate, PermTicketsManage, PermTicketsResolve,
		PermFAQsManage,
		PermForumView,
		PermCommentsView,
		PermAnnouncementsView,
		PermNotificationsSend,
		PermSettingsView,
	}
}

func getTeacherPermissions() []string {
	return []string{
		PermDashboardView, PermAnalyticsView,
		PermStudentsView,
		PermSubjectsView, PermOwnSubjectsManage,
		PermBooksView, PermOwnBooksManage,
		PermResourcesView, PermOwnResourcesManage,
		PermExamsView, PermOwnExamsManage,
		PermChallengesView, PermOwnChallengesManage,
		PermBlogView, PermBlogCreate, PermBlogUpdate,
		PermForumView, PermForumCreate,
		PermCommentsView, PermCommentsCreate,
		PermAchievementsView,
		PermRewardsView,
		PermAiUsage,
	}
}

func getParentPermissions() []string {
	return []string{
		PermDashboardView,
		PermChildrenView, PermChildrenGrades, PermChildrenProgress, PermChildrenAttendance, PermChildrenCommunicate, PermChildrenPayment,
		PermSubjectsView,
		PermBooksView,
		PermExamsView,
		PermBlogView,
		PermForumView,
		PermCommentsView,
		PermAchievementsView,
		PermNotificationsSend,
	}
}

func getStudentPermissions() []string {
	return []string{
		PermDashboardView, PermAnalyticsView,
		PermSubjectsView,
		PermBooksView,
		PermResourcesView,
		PermExamsView,
		PermChallengesView,
		PermBlogView,
		PermForumView, PermForumCreate,
		PermCommentsView, PermCommentsCreate,
		PermAchievementsView,
		PermRewardsView,
		PermAiUsage,
	}
}

// ─────────────────────────────────────────────
//  Permission Validation & Matching
// ─────────────────────────────────────────────

// PermissionGrantMatches checks if a grant matches a required permission with wildcard support.
// It is exported so middleware can check the already-resolved effective grant list
// without resolving role defaults a second time.
func PermissionGrantMatches(grant, required string) bool {
	if grant == required || grant == PermAdminBypass {
		return true
	}
	// A module-level manage grant includes read access to that same module and
	// grants more specific submodule actions under the same module.
	if strings.HasSuffix(grant, ":manage") {
		module := strings.TrimSuffix(grant, ":manage")
		if required == module+":view" || strings.HasPrefix(required, module+":") {
			return true
		}
	}
	if grant == "*:manage" {
		return strings.HasSuffix(required, ":manage")
	}
	if len(grant) > 2 && strings.HasSuffix(grant, ":*") {
		mod := grant[:len(grant)-2]
		return strings.HasPrefix(required, mod+":")
	}
	return false
}

// permissionGrantMatches remains for package-local compatibility.
func permissionGrantMatches(grant, required string) bool {
	return PermissionGrantMatches(grant, required)
}

// ─────────────────────────────────────────────
//  All Permission Keys (for seeding/migration)
// ─────────────────────────────────────────────

// AllPermissions returns every defined permission constant.
func AllPermissions() []string {
	return append(basePermissions(), DashboardWidgetPermissions()...)
}

func basePermissions() []string {
	return []string{
		PermAdminBypass, PermSystemManage, PermSystemSettings,
		PermDashboardView, PermAnalyticsView, PermReportsView, PermReportsManage, PermAuditLogsView,
		PermDashboardManage,
		PermUsersView, PermUsersCreate, PermUsersUpdate, PermUsersDelete, PermUsersManage, PermUsersImpersonate, PermUsersExport, PermUsersImport,
		PermStudentsView, PermStudentsManage,
		PermTeachersView, PermTeachersManage,
		PermParentsView, PermParentsManage,
		PermSubjectsView, PermSubjectsCreate, PermSubjectsUpdate, PermSubjectsDelete, PermSubjectsManage, PermSubjectsPublish, PermSubjectsApprove, PermOwnSubjectsManage,
		PermBooksView, PermBooksCreate, PermBooksUpdate, PermBooksDelete, PermBooksManage, PermBooksPublish, PermOwnBooksManage,
		PermResourcesView, PermResourcesManage, PermResourcesPublish, PermOwnResourcesManage,
		PermExamsView, PermExamsCreate, PermExamsUpdate, PermExamsDelete, PermExamsManage, PermExamsApprove, PermExamsPublish, PermOwnExamsManage,
		PermChallengesView, PermChallengesManage, PermOwnChallengesManage,
		PermContestsView, PermContestsManage,
		PermBlogView, PermBlogCreate, PermBlogUpdate, PermBlogDelete, PermBlogManage, PermBlogPublish,
		PermForumView, PermForumCreate, PermForumUpdate, PermForumDelete, PermForumModerate, PermForumManage,
		PermCommentsView, PermCommentsCreate, PermCommentsModerate,
		PermEventsView, PermEventsManage,
		PermAnnouncementsView, PermAnnouncementsManage,
		PermTicketsView, PermTicketsCreate, PermTicketsUpdate, PermTicketsManage, PermTicketsResolve, PermFAQsManage,
		PermChildrenView, PermChildrenGrades, PermChildrenProgress, PermChildrenAttendance, PermChildrenCommunicate, PermChildrenPayment,
		PermAchievementsView, PermAchievementsManage,
		PermRewardsView, PermRewardsManage,
		PermAiManage, PermAiUsage,
		PermLiveMonitorView, PermMarketingView, PermMarketingManage,
		PermAbTestingView, PermSettingsView,
		PermSeasonsView, PermSeasonsManage,
		PermNotificationsSend, PermNotificationsManage,
		PermUsersSendNotifications,
	}
}

// PermissionModules returns all unique permission modules.
func PermissionModules() []string {
	return []string{
		"admin", "system",
		"dashboard", "analytics", "reports", "audit_logs",
		"users", "students", "teachers", "parents",
		"subjects", "books", "resources",
		"exams", "challenges", "contests",
		"blog", "forum", "comments",
		"events", "announcements",
		"tickets", "faqs",
		"children",
		"achievements", "rewards",
		"ai", "live_monitor", "marketing", "ab_testing",
		"settings", "seasons",
		"notifications",
	}
}
