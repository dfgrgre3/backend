package middleware

import (
	"fmt"
	"net/http"
	"slices"

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

		u := &models.User{Permissions: models.JSONStringArray(perms)}
		roleStr := getRoleFromContext(c)
		u.Role = models.UserRole(roleStr)

		if !u.HasPermission(permission) {
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
