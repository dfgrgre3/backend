package api

import (
	models "thanawy-backend/internal/domain/common"
	admindelivery "thanawy-backend/internal/infrastructure/api/handlers/admin"
	handlers "thanawy-backend/internal/infrastructure/api/handlers/protected"
	"thanawy-backend/internal/infrastructure/api/middleware"

	"github.com/gin-gonic/gin"
)

// registerAdminUserRoutes registers User/Subject admin operations, bulk user
// operations, user actions, administrators, and parent management routes.
func registerAdminUserRoutes(admin, sensitive *gin.RouterGroup) {
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
	admin.GET("/users/:id/certificates", handlers.GetUserCertificates)
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
	// Paginated cross-user session list for the admin "user-sessions" page.
	// Supports ?page=&limit=&userId=&active=&status= filters.
	admin.GET("/user-sessions", handlers.ListUserSessions)
	admin.GET("/users/:id/sessions", handlers.GetUserSessions)
	admin.POST("/users/:id/sessions/:sessionId/terminate", handlers.TerminateSession)
	admin.POST("/users/:id/sessions/terminate-all", handlers.TerminateAllSessions)
	admin.GET("/users/:id/audit-logs", handlers.GetUserAuditLogs)
	admin.POST("/users/:id/send-activation-link", handlers.SendActivationLink)
	admin.GET("/users/:id/notes", handlers.GetUserAdminNotes)
	admin.POST("/users/:id/notes", handlers.CreateUserAdminNote)
	admin.PATCH("/users/:id/notes/:noteId", handlers.UpdateUserAdminNote)
	admin.DELETE("/users/:id/notes/:noteId", handlers.DeleteUserAdminNote)

	// Administrators are a first-class, server-filtered resource.  Keeping
	// this separate from the broad users endpoint prevents clients from
	// downloading a user directory and filtering privileged accounts locally.
	admin.GET("/admins", middleware.PermissionRequired(models.PermUsersManage), handlers.ListAdmins)

	// Parent Management
	admin.GET("/parents/statistics", handlers.GetParentStatistics)
	admin.GET("/users/:id/students", handlers.GetParentStudents)
	admin.POST("/users/:id/students/link", handlers.LinkStudentToParent)
	admin.DELETE("/users/:id/students/unlink", handlers.UnlinkStudentFromParent)
}
