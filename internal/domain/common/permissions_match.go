package models

import "strings"

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
