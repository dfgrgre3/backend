package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasPermission_AdminBypass(t *testing.T) {
	user := &User{
		Role:        RoleAdmin,
		Permissions: JSONStringArray{PermAdminBypass},
	}

	assert.True(t, user.HasPermission("users:view"))
	assert.True(t, user.HasPermission("users:manage"))
	assert.True(t, user.HasPermission("subjects:manage"))
	assert.True(t, user.HasPermission("any:random:permission"))
}

func TestHasPermission_ExplicitPermission(t *testing.T) {
	user := &User{
		Role:        RoleTeacher,
		Permissions: JSONStringArray{"subjects:manage"},
	}

	assert.True(t, user.HasPermission("subjects:manage"))
	assert.False(t, user.HasPermission("users:manage"))
	assert.True(t, user.HasPermission("subjects:view"))
}

func TestHasPermission_ModuleWildcard(t *testing.T) {
	user := &User{
		Role:        RoleTeacher,
		Permissions: JSONStringArray{"subjects:*"},
	}

	assert.True(t, user.HasPermission("subjects:view"))
	assert.True(t, user.HasPermission("subjects:manage"))
	assert.True(t, user.HasPermission("subjects:delete"))
	assert.False(t, user.HasPermission("users:view"))
}

func TestHasPermission_ManageWildcard(t *testing.T) {
	user := &User{
		Role:        RoleTeacher,
		Permissions: JSONStringArray{PermPermissionsCustom, "*:manage"},
	}

	assert.True(t, user.HasPermission("subjects:manage"))
	assert.True(t, user.HasPermission("users:manage"))
	assert.True(t, user.HasPermission("exams:manage"))
	assert.False(t, user.HasPermission("subjects:view"))
}

func TestHasPermission_NoPermissions(t *testing.T) {
	user := &User{
		Role:        RoleStudent,
		Permissions: JSONStringArray{},
	}

	assert.False(t, user.HasPermission("users:view"))
	assert.False(t, user.HasPermission("subjects:manage"))
}

func TestHasPermission_NilPermissions(t *testing.T) {
	user := &User{
		Role:        RoleStudent,
		Permissions: nil,
	}

	assert.False(t, user.HasPermission("users:view"))
}

func TestGetEffectivePermissions_Admin(t *testing.T) {
	user := &User{
		Role:        RoleAdmin,
		Permissions: JSONStringArray{"users:view"},
	}

	perms := user.GetEffectivePermissions()
	assert.Contains(t, perms, "users:view")
	// Admin gets a broad but explicit default set.
	assert.Contains(t, perms, PermUsersManage)
	assert.Contains(t, perms, PermSystemManage)
	// admin:bypass is the wildcard and stays exclusive to SUPER_ADMIN, so a
	// plain ADMIN must NOT inherit it from the role defaults.
	assert.NotContains(t, perms, PermAdminBypass)
}

func TestGetEffectivePermissions_SuperAdminHasBypass(t *testing.T) {
	user := &User{
		Role:        RoleSuperAdmin,
		Permissions: JSONStringArray{},
	}

	perms := user.GetEffectivePermissions()
	assert.Contains(t, perms, PermAdminBypass)
}

func TestGetEffectivePermissions_AdminWithBypass(t *testing.T) {
	user := &User{
		Role:        RoleAdmin,
		Permissions: JSONStringArray{PermAdminBypass},
	}

	perms := user.GetEffectivePermissions()
	// An explicit per-user grant still works and must not be duplicated.
	assert.Contains(t, perms, PermAdminBypass)
	count := 0
	for _, p := range perms {
		if p == PermAdminBypass {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestGetEffectivePermissions_Teacher(t *testing.T) {
	user := &User{
		Role:        RoleTeacher,
		Permissions: JSONStringArray{},
	}

	perms := user.GetEffectivePermissions()
	// Teacher role defaults are now merged in.
	assert.Contains(t, perms, "subjects:view")
	assert.Contains(t, perms, "exams:view")
}

func TestGetEffectivePermissions_Student(t *testing.T) {
	user := &User{
		Role:        RoleStudent,
		Permissions: JSONStringArray{},
	}

	perms := user.GetEffectivePermissions()
	// Student role defaults are now merged in.
	assert.Contains(t, perms, "subjects:view")
	assert.Contains(t, perms, "dashboard:view")
}

func TestGetDefaultPermissions_Admin(t *testing.T) {
	perms := GetDefaultPermissions(RoleAdmin)
	// The plain ADMIN role gets an explicit broad set but NOT the wildcard.
	assert.Contains(t, perms, PermUsersManage)
	assert.Contains(t, perms, PermSystemManage)
	assert.NotContains(t, perms, PermAdminBypass)
}

func TestGetDefaultPermissions_SuperAdmin(t *testing.T) {
	perms := GetDefaultPermissions(RoleSuperAdmin)
	assert.Contains(t, perms, PermAdminBypass)
}

func TestGetDefaultPermissions_Teacher(t *testing.T) {
	perms := GetDefaultPermissions(RoleTeacher)
	assert.Contains(t, perms, "subjects:view")
	assert.Contains(t, perms, "exams:view")
}

func TestGetDefaultPermissions_Student(t *testing.T) {
	perms := GetDefaultPermissions(RoleStudent)
	assert.Contains(t, perms, "subjects:view")
	assert.Contains(t, perms, "dashboard:view")
}

func TestGetDefaultPermissions_Moderator(t *testing.T) {
	perms := GetDefaultPermissions(RoleModerator)
	assert.Contains(t, perms, "users:view")
}

func TestGetDefaultPermissions_Unknown(t *testing.T) {
	perms := GetDefaultPermissions("UNKNOWN")
	assert.Empty(t, perms)
}

func TestPermissionGrantMatches_ExactMatch(t *testing.T) {
	assert.True(t, permissionGrantMatches("users:view", "users:view"))
	assert.True(t, permissionGrantMatches("subjects:manage", "subjects:manage"))
}

func TestPermissionGrantMatches_AdminBypass(t *testing.T) {
	assert.True(t, permissionGrantMatches(PermAdminBypass, "users:view"))
	assert.True(t, permissionGrantMatches(PermAdminBypass, "any:thing"))
}

func TestPermissionGrantMatches_ManageWildcard(t *testing.T) {
	assert.True(t, permissionGrantMatches("*:manage", "users:manage"))
	assert.True(t, permissionGrantMatches("*:manage", "subjects:manage"))
	assert.False(t, permissionGrantMatches("*:manage", "users:view"))
}

func TestPermissionGrantMatches_ModuleWildcard(t *testing.T) {
	assert.True(t, permissionGrantMatches("users:*", "users:view"))
	assert.True(t, permissionGrantMatches("users:*", "users:manage"))
	assert.True(t, permissionGrantMatches("users:*", "users:delete"))
	assert.False(t, permissionGrantMatches("users:*", "subjects:view"))
}

func TestPermissionGrantMatches_NoMatch(t *testing.T) {
	assert.False(t, permissionGrantMatches("users:view", "users:manage"))
	assert.False(t, permissionGrantMatches("subjects:view", "users:view"))
	assert.False(t, permissionGrantMatches("", "users:view"))
}

func TestUserRoleConstants(t *testing.T) {
	assert.Equal(t, UserRole("ADMIN"), RoleAdmin)
	assert.Equal(t, UserRole("TEACHER"), RoleTeacher)
	assert.Equal(t, UserRole("STUDENT"), RoleStudent)
	assert.Equal(t, UserRole("MODERATOR"), RoleModerator)
}

func TestPermissionConstants(t *testing.T) {
	assert.Equal(t, "dashboard:view", PermDashboardView)
	assert.Equal(t, "analytics:view", PermAnalyticsView)
	assert.Equal(t, "users:view", PermUsersView)
	assert.Equal(t, "users:manage", PermUsersManage)
	assert.Equal(t, "users:send:notifications", PermUsersSendNotifications)
	assert.Equal(t, "subjects:view", PermSubjectsView)
	assert.Equal(t, "subjects:manage", PermSubjectsManage)
	assert.Equal(t, "admin:bypass", PermAdminBypass)
	assert.Equal(t, "system:manage", PermSystemManage)
}

func TestPermissionGrantMatches_ManageIncludesSubPermissions(t *testing.T) {
	assert.True(t, permissionGrantMatches("users:manage", "users:send:notifications"))
	assert.True(t, permissionGrantMatches("resources:manage", "resources:publish"))
}

func TestHasPermission_MultiplePermissions(t *testing.T) {
	user := &User{
		Role: RoleTeacher,
		Permissions: JSONStringArray{
			"subjects:view",
			"subjects:manage",
			"exams:view",
		},
	}

	assert.True(t, user.HasPermission("subjects:view"))
	assert.True(t, user.HasPermission("subjects:manage"))
	assert.True(t, user.HasPermission("exams:view"))
	assert.False(t, user.HasPermission("users:manage"))
	assert.False(t, user.HasPermission("admin:bypass"))
}

func TestHasPermission_CaseSensitive(t *testing.T) {
	user := &User{
		Role:        RoleTeacher,
		Permissions: JSONStringArray{"subjects:manage"},
	}

	assert.True(t, user.HasPermission("subjects:manage"))
	assert.False(t, user.HasPermission("Subjects:Manage"))
	assert.False(t, user.HasPermission("SUBJECTS:MANAGE"))
}

func TestGetEffectivePermissions_Deduplication(t *testing.T) {
	user := &User{
		Role:        RoleAdmin,
		Permissions: JSONStringArray{PermAdminBypass, PermAdminBypass},
	}

	perms := user.GetEffectivePermissions()
	count := 0
	for _, p := range perms {
		if p == PermAdminBypass {
			count++
		}
	}
	assert.GreaterOrEqual(t, count, 1)
}

func TestHasPermission_StudentCannotManage(t *testing.T) {
	user := &User{
		Role:        RoleStudent,
		Permissions: JSONStringArray{},
	}

	managePermissions := []string{
		"users:manage",
		"subjects:manage",
		"exams:manage",
		"teachers:manage",
		"marketing:manage",
	}

	for _, perm := range managePermissions {
		assert.False(t, user.HasPermission(perm), "Student should not have %s", perm)
	}
}

func TestHasPermission_TeacherGetsRoleDefaults(t *testing.T) {
	user := &User{
		Role:        RoleTeacher,
		Permissions: JSONStringArray{},
	}

	// Teacher role defaults now include subjects:view and own_subjects:manage
	assert.True(t, user.HasPermission("subjects:view"))
	assert.True(t, user.HasPermission("own_subjects:manage"))
	// But not admin-level permissions
	assert.False(t, user.HasPermission("users:manage"))
	assert.False(t, user.HasPermission("admin:bypass"))
}

func TestJSONStringArray_Conversion(t *testing.T) {
	perms := JSONStringArray{"a", "b", "c"}
	strSlice := []string(perms)

	assert.Equal(t, []string{"a", "b", "c"}, strSlice)
}

func TestCategoryTypeConstants(t *testing.T) {
	assert.Equal(t, CategoryType("COURSE"), CategoryTypeCourse)
	assert.Equal(t, CategoryType("BLOG"), CategoryTypeBlog)
	assert.Equal(t, CategoryType("LIBRARY"), CategoryTypeLibrary)
}

func TestPermissionGrantMatches_EdgeCases(t *testing.T) {
	assert.False(t, permissionGrantMatches("*", "users:view"))
	assert.False(t, permissionGrantMatches("users", "users:view"))
	assert.True(t, permissionGrantMatches("users:*", "users:"))
	assert.False(t, permissionGrantMatches(":manage", "users:manage"))
}

func TestHasPermission_ExactPermissionOnly(t *testing.T) {
	user := &User{
		Role:        RoleTeacher,
		Permissions: JSONStringArray{"subjects:view"},
	}

	assert.True(t, user.HasPermission("subjects:view"))
	assert.False(t, user.HasPermission("subjects:manage"))
	assert.False(t, user.HasPermission("subjects:"))
}

func TestGetEffectivePermissions_PreservesExisting(t *testing.T) {
	user := &User{
		Role:        RoleTeacher,
		Permissions: JSONStringArray{"custom:permission"},
	}

	perms := user.GetEffectivePermissions()
	assert.Contains(t, perms, "custom:permission")
}

func TestPermissionGrantMatches_AdminBypassExact(t *testing.T) {
	assert.True(t, permissionGrantMatches(PermAdminBypass, PermAdminBypass))
	assert.True(t, permissionGrantMatches(PermAdminBypass, "dashboard:view"))
	assert.True(t, permissionGrantMatches(PermAdminBypass, "system:manage"))
}

func TestHasPermission_AdminHasBroadButNotWildcardDefaults(t *testing.T) {
	user := &User{
		Role:        RoleAdmin,
		Permissions: JSONStringArray{},
	}

	// The plain ADMIN role defaults cover the real admin surface but exclude
	// the admin:bypass wildcard, so unknown permissions are still denied.
	perms := user.GetEffectivePermissions()
	assert.NotContains(t, perms, PermAdminBypass)
	assert.True(t, user.HasPermission("users:manage"))
	assert.True(t, user.HasPermission("system:manage"))
	assert.False(t, user.HasPermission("any:permission"))
}

func TestHasPermission_SuperAdminBypassesEverything(t *testing.T) {
	user := &User{
		Role:        RoleSuperAdmin,
		Permissions: JSONStringArray{},
	}

	// SUPER_ADMIN inherits admin:bypass from the role defaults, which the
	// matcher treats as a wildcard over every permission.
	assert.Contains(t, user.GetEffectivePermissions(), PermAdminBypass)
	assert.True(t, user.HasPermission("any:permission"))
	assert.True(t, user.HasPermission("users:manage"))
}

func TestHasPermission_CustomModeRestrictsToStoredOnly(t *testing.T) {
	user := &User{
		Role:        RoleAdmin,
		Permissions: JSONStringArray{PermPermissionsCustom, "users:view"},
	}

	// In custom mode, role defaults are NOT merged — only stored permissions.
	perms := user.GetEffectivePermissions()
	assert.Contains(t, perms, "users:view")
	assert.NotContains(t, perms, PermAdminBypass)
	assert.False(t, user.HasPermission("any:permission"))
	assert.True(t, user.HasPermission("users:view"))
}

func TestGetDefaultPermissions_ReturnsEmptyForUnknown(t *testing.T) {
	perms := GetDefaultPermissions("NONEXISTENT")
	assert.Empty(t, perms)
}

func TestPermissionGrantMatches_PartialWildcard(t *testing.T) {
	assert.True(t, permissionGrantMatches("users:*", "users:delete"))
	assert.True(t, permissionGrantMatches("users:*", "users:export"))
	assert.False(t, permissionGrantMatches("users:*", "user:view"))
	assert.False(t, permissionGrantMatches("users:*", "users"))
}
