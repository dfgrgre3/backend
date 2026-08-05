package db

import (
	"fmt"
	"net/url"
	"os"
)

// DBRole represents the database role to use for connections
type DBRole string

const (
	// RoleApp is the restricted role for normal application operations
	RoleApp DBRole = "app_user"
	// RoleMigration is the privileged role for schema migrations
	RoleMigration DBRole = "migration_user"
)

// GetDSNForRole returns the DSN with the `role` query parameter set to the
// given role. It uses net/url for correct, RFC-3986-compliant parsing so that
// existing query parameters are not corrupted by naive string manipulation.
func GetDSNForRole(baseDSN string, role DBRole) (string, error) {
	if baseDSN == "" {
		return "", fmt.Errorf("base DSN cannot be empty")
	}
	if role == "" {
		return "", fmt.Errorf("role cannot be empty")
	}

	u, err := url.Parse(baseDSN)
	if err != nil {
		return "", fmt.Errorf("parse DSN: %w", err)
	}

	q := u.Query()
	q.Set("role", string(role))
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// GetAppDSN returns the DSN configured for application operations
func GetAppDSN() (string, error) {
	baseDSN := os.Getenv("DATABASE_URL")
	if baseDSN == "" {
		baseDSN = os.Getenv("DATABASE_WRITE_DSN")
	}
	if baseDSN == "" {
		return "", fmt.Errorf("DATABASE_URL or DATABASE_WRITE_DSN must be set")
	}

	return GetDSNForRole(baseDSN, RoleApp)
}

// GetMigrationDSN returns the DSN configured for migration operations
func GetMigrationDSN() (string, error) {
	// Allow a dedicated superuser DSN for migrations (e.g. Supabase service role).
	if migrationDSN := os.Getenv("DATABASE_MIGRATION_DSN"); migrationDSN != "" {
		return migrationDSN, nil
	}

	baseDSN := os.Getenv("DATABASE_URL")
	if baseDSN == "" {
		baseDSN = os.Getenv("DATABASE_WRITE_DSN")
	}
	if baseDSN == "" {
		return "", fmt.Errorf("DATABASE_URL or DATABASE_WRITE_DSN must be set")
	}

	return GetDSNForRole(baseDSN, RoleMigration)
}
