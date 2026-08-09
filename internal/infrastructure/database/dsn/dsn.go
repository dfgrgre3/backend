package dsn

import (
	"fmt"
	"net/url"
	"os"
)

// Role represents the database role to use for connections
type Role string

const (
	// RoleApp is the restricted role for normal application operations
	RoleApp Role = "app_user"
	// RoleMigration is the privileged role for schema migrations
	RoleMigration Role = "migration_user"
)

// ForRole returns the DSN with the `role` query parameter set to the
// given role. It uses net/url for correct, RFC-3986-compliant parsing so that
// existing query parameters are not corrupted by naive string manipulation.
func ForRole(baseDSN string, role Role) (string, error) {
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

// App returns the DSN configured for application operations
func App() (string, error) {
	baseDSN := os.Getenv("DATABASE_URL")
	if baseDSN == "" {
		baseDSN = os.Getenv("DATABASE_WRITE_DSN")
	}
	if baseDSN == "" {
		return "", fmt.Errorf("DATABASE_URL or DATABASE_WRITE_DSN must be set")
	}

	return ForRole(baseDSN, RoleApp)
}

// StripRole returns the DSN with the 'role' query parameter removed.
// This allows creating a connection that bypasses Row-Level Security by
// connecting as the base database user (e.g., the table owner/superuser)
// without switching to the restricted app_user role.
func StripRole(dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Del("role")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// Migration returns the DSN configured for migration operations
func Migration() (string, error) {
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

	return ForRole(baseDSN, RoleMigration)
}
