package db

import (
	"thanawy-backend/internal/infrastructure/database/dsn"
)

// Compatibility facade over the database/dsn package.
// New code should import database/dsn directly.

// DBRole represents the database role to use for connections
type DBRole = dsn.Role

const (
	// RoleApp is the restricted role for normal application operations
	RoleApp = dsn.RoleApp
	// RoleMigration is the privileged role for schema migrations
	RoleMigration = dsn.RoleMigration
)

// GetDSNForRole returns the DSN with the `role` query parameter set to the given role.
func GetDSNForRole(baseDSN string, role DBRole) (string, error) { return dsn.ForRole(baseDSN, role) }

// GetAppDSN returns the DSN configured for application operations
func GetAppDSN() (string, error) { return dsn.App() }

// StripRoleFromDSN returns the DSN with the 'role' query parameter removed.
func StripRoleFromDSN(rawDSN string) (string, error) { return dsn.StripRole(rawDSN) }

// GetMigrationDSN returns the DSN configured for migration operations
func GetMigrationDSN() (string, error) { return dsn.Migration() }
