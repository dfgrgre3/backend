package api

import (
	handlers "thanawy-backend/internal/infrastructure/api/handlers/protected"

	"github.com/gin-gonic/gin"
)

// registerAdminRoleRoutes registers Roles, Permissions, Role Permissions and
// Role Users management routes.
func registerAdminRoleRoutes(admin *gin.RouterGroup) {
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
