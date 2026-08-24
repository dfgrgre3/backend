package models

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
//  Permission Constants — role levels and super/dashboard permissions.
//
//  This RBAC module is split across several files in this package:
//    permissions.go           — role levels + super/system/dashboard permissions
//    permissions_users.go     — user/student/teacher/parent permissions
//    permissions_content.go   — content, books, resources, exams, challenges
//    permissions_community.go — blog, forum, comments, events, announcements
//    permissions_misc.go      — support, parent dashboard, misc
//    permissions_hierarchy.go — role level lookup
//    permissions_defaults.go  — per-role default permission sets
//    permissions_match.go     — grant matching/wildcard logic
//    permissions_registry.go  — AllPermissions/PermissionModules enumeration
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
