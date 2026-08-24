package models

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
