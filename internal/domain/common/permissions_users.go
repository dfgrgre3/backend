package models

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
	PermParentsView            = "parents:view"
	PermParentsManage          = "parents:manage"
)
