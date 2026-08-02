package middleware

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"thanawy-backend/internal/models"

	"github.com/gin-gonic/gin"
)

// getUserIDFromContext returns the current user ID from the preferred or legacy gin context keys.
func getUserIDFromContext(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists && userID != nil {
		if value, ok := userID.(string); ok && value != "" {
			return value
		}
	}
	if userID, exists := c.Get("userId"); exists && userID != nil {
		if value, ok := userID.(string); ok && value != "" {
			return value
		}
	}
	return ""
}

// AdminAPIPermissionRequired applies a server-side, deny-by-default permission
// check to every administrative endpoint. Route visibility in the frontend is
// only a convenience; this guard is the API security boundary.
func AdminAPIPermissionRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		required := adminAPIPermission(c.Request.URL.Path, c.Request.Method)
		if required == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Access denied", "code": "insufficient_permission"})
			return
		}
		permsRaw, _ := c.Get("permissions")
		perms, _ := permsRaw.([]string)
		for _, grant := range perms {
			if models.PermissionGrantMatches(grant, required) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": fmt.Sprintf("Permission '%s' required", required), "code": "insufficient_permission"})
	}
}

func adminAPIPermission(path, method string) string {
	write := method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
	for _, rule := range []struct{ prefix, view, manage string }{
		{"/api/admin/dashboard", models.PermDashboardView, ""},
		{"/api/admin/users", models.PermUsersView, models.PermUsersManage},
		{"/api/admin/admins", models.PermUsersManage, models.PermUsersManage},
		{"/api/admin/teachers", models.PermTeachersView, models.PermTeachersManage},
		{"/api/admin/instructors", models.PermTeachersView, models.PermTeachersManage},
		{"/api/admin/analytics", models.PermAnalyticsView, ""},
		{"/api/admin/reports", models.PermReportsView, models.PermReportsManage},
		{"/api/admin/courses", models.PermSubjectsView, models.PermSubjectsManage},
		{"/api/admin/subjects", models.PermSubjectsView, models.PermSubjectsManage},
		{"/api/admin/course-categories", models.PermSubjectsView, models.PermSubjectsManage},
		{"/api/admin/learning-paths", models.PermSubjectsView, models.PermSubjectsManage},
		{"/api/admin/books", models.PermBooksView, models.PermBooksManage},
		{"/api/admin/resources", models.PermResourcesView, models.PermResourcesManage},
		{"/api/admin/exams", models.PermExamsView, models.PermExamsManage},
		{"/api/admin/bank-questions", models.PermExamsView, models.PermExamsManage},
		{"/api/admin/challenges", models.PermChallengesView, models.PermChallengesManage},
		{"/api/admin/achievements", models.PermAchievementsView, models.PermAchievementsManage},
		{"/api/admin/rewards", models.PermRewardsView, models.PermRewardsManage},
		{"/api/admin/seasons", models.PermSeasonsView, models.PermSeasonsManage},
		{"/api/admin/marketing", models.PermMarketingView, models.PermMarketingManage},
		{"/api/admin/coupons", models.PermMarketingView, models.PermMarketingManage},
		{"/api/admin/announcements", models.PermAnnouncementsView, models.PermAnnouncementsManage},
		{"/api/admin/notifications", models.PermNotificationsManage, models.PermNotificationsManage},
		{"/api/admin/forum", models.PermForumView, models.PermForumManage},
		{"/api/admin/blog", models.PermBlogView, models.PermBlogManage},
		{"/api/admin/events", models.PermEventsView, models.PermEventsManage},
		{"/api/admin/contests", models.PermContestsView, models.PermContestsManage},
		{"/api/admin/tickets", models.PermTicketsView, models.PermTicketsManage},
		{"/api/admin/live", models.PermLiveMonitorView, models.PermLiveMonitorView},
		{"/api/admin/health", models.PermLiveMonitorView, models.PermLiveMonitorView},
		{"/api/admin/ab-testing", models.PermAbTestingView, models.PermAbTestingView},
		{"/api/admin/media", models.PermResourcesView, models.PermResourcesManage},
		{"/api/admin/lessons", models.PermSubjectsView, models.PermSubjectsManage},
		{"/api/admin/landing", models.PermSettingsView, models.PermSettingsView},
		{"/api/admin/affiliates", models.PermAnalyticsView, models.PermAnalyticsView},
		{"/api/admin/dunning", models.PermAnalyticsView, models.PermAnalyticsView},
		{"/api/admin/security", models.PermSettingsView, models.PermSettingsView},
		{"/api/admin/audit-logs", models.PermAuditLogsView, ""},
		{"/api/admin/ai", models.PermAiManage, models.PermAiManage},
		{"/api/admin/automations", models.PermAdminBypass, models.PermAdminBypass},
		{"/api/admin/backups", models.PermSettingsView, models.PermSettingsView},
		{"/api/admin/infrastructure", models.PermSettingsView, models.PermSettingsView},
	} {
		if strings.HasPrefix(path, rule.prefix) {
			if write && rule.manage != "" {
				return rule.manage
			}
			return rule.view
		}
	}
	// New endpoints must be explicitly mapped above. Only an explicitly stored
	// admin:bypass grant can access an unmapped endpoint.
	return models.PermAdminBypass
}

// getRoleFromContext returns the current role from the preferred or legacy gin context keys.
func getRoleFromContext(c *gin.Context) string {
	if role, exists := c.Get("user_role"); exists && role != nil {
		if value, ok := role.(string); ok && value != "" {
			return value
		}
	}
	if role, exists := c.Get("role"); exists && role != nil {
		if value, ok := role.(string); ok && value != "" {
			return value
		}
	}
	return ""
}

// AdminRequired ensures the user has ADMIN or SUPER_ADMIN role.
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleStr := getRoleFromContext(c)
		if roleStr != string(models.RoleAdmin) && roleStr != string(models.RoleSuperAdmin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Admin access required",
				"code":  "insufficient_role",
			})
			return
		}
		c.Next()
	}
}

// ModeratorRequired ensures the user has at least MODERATOR role.
func ModeratorRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleStr := getRoleFromContext(c)
		allowed := []string{
			string(models.RoleModerator),
			string(models.RoleAdmin),
			string(models.RoleSuperAdmin),
		}
		if !slices.Contains(allowed, roleStr) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Moderator or higher access required",
				"code":  "insufficient_role",
			})
			return
		}
		c.Next()
	}
}

// AdminOrModerator allows ADMIN, SUPER_ADMIN, MODERATOR, and SUPPORT.
func AdminOrModerator() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleStr := getRoleFromContext(c)
		allowed := []string{
			string(models.RoleModerator),
			string(models.RoleAdmin),
			string(models.RoleSuperAdmin),
			string(models.RoleSupport),
		}
		if !slices.Contains(allowed, roleStr) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Admin, moderator or support access required",
				"code":  "insufficient_role",
			})
			return
		}
		c.Next()
	}
}

// RoleRequired ensures the user has one of the given roles.
func RoleRequired(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleStr := getRoleFromContext(c)
		if !slices.Contains(roles, roleStr) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("One of roles %v required", roles),
				"code":  "insufficient_role",
			})
			return
		}
		c.Next()
	}
}

// PermissionRequired validates that the user holds a specific permission string.
func PermissionRequired(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		permsRaw, _ := c.Get("permissions")
		perms, _ := permsRaw.([]string)

		// Auth already resolved the effective permissions from the database. Do
		// not resolve them again here, or role defaults would be restored for a
		// deliberately restrictive custom permission set.
		allowed := false
		for _, grant := range perms {
			if models.PermissionGrantMatches(grant, permission) {
				allowed = true
				break
			}
		}

		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("Permission '%s' required", permission),
				"code":  "insufficient_permission",
			})
			return
		}
		c.Next()
	}
}

// AnyAuthenticatedUser ensures that at least a valid user ID is set.
func AnyAuthenticatedUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if userID := getUserIDFromContext(c); userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
				"code":  "not_authenticated",
			})
			return
		}
		c.Next()
	}
}

// StrictRBAC is a Deny-by-Default guard that passes if Auth() already set a user.
func StrictRBAC() gin.HandlerFunc {
	return func(c *gin.Context) {
		if userID := getUserIDFromContext(c); userID == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Access denied",
				"code":  "rbac_deny",
			})
			return
		}
		c.Next()
	}
}

// TeacherRequired ensures the user has TEACHER, ADMIN, or SUPER_ADMIN role.
func TeacherRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleStr := getRoleFromContext(c)
		if roleStr != string(models.RoleTeacher) && roleStr != string(models.RoleAdmin) && roleStr != string(models.RoleSuperAdmin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Teacher access required",
				"code":  "insufficient_role",
			})
			return
		}
		c.Next()
	}
}

// StudentRequired ensures the user has STUDENT, ADMIN, or SUPER_ADMIN role.
func StudentRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleStr := getRoleFromContext(c)
		if roleStr != string(models.RoleStudent) && roleStr != string(models.RoleAdmin) && roleStr != string(models.RoleSuperAdmin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Student access required",
				"code":  "insufficient_role",
			})
			return
		}
		c.Next()
	}
}
