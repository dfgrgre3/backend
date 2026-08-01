package db

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
)

// DBRole represents the database role to use for connections
type DBRole string

const (
	// RoleApp is the restricted role for normal application operations
	RoleApp DBRole = "app_user"
	// RoleMigration is the privileged role for schema migrations
	RoleMigration DBRole = "migration_user"
	// RoleAdmin is the superuser role for administrative tasks
	RoleAdmin DBRole = "postgres"
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

// extractRole extracts the role parameter from a DSN string for logging purposes.
// It uses url.Parse for safety, falling back to an empty string on any error.
func extractRole(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return ""
	}
	return u.Query().Get("role")
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

// GetAdminDSN returns the DSN configured for administrative operations
func GetAdminDSN() (string, error) {
	if adminDSN := os.Getenv("DATABASE_ADMIN_DSN"); adminDSN != "" {
		return adminDSN, nil
	}

	baseDSN := os.Getenv("DATABASE_URL")
	if baseDSN == "" {
		baseDSN = os.Getenv("DATABASE_WRITE_DSN")
	}
	if baseDSN == "" {
		return "", fmt.Errorf("DATABASE_URL or DATABASE_WRITE_DSN must be set")
	}

	return GetDSNForRole(baseDSN, RoleAdmin)
}

// SetupDatabaseRoles logs the role configuration during application initialisation.
func SetupDatabaseRoles() {
	appDSN, err := GetAppDSN()
	if err != nil {
		log.Printf("[WARN] Failed to get app DSN: %v", err)
	} else {
		log.Printf("[DB Roles] App DSN configured with role: %s", extractRole(appDSN))
	}

	migrationDSN, err := GetMigrationDSN()
	if err != nil {
		log.Printf("[WARN] Failed to get migration DSN: %v", err)
	} else {
		// Avoid logging the full DSN; show only the role for safety.
		role := extractRole(migrationDSN)
		if role == "" {
			// DATABASE_MIGRATION_DSN may not have a role parameter
			role = "(dedicated migration DSN)"
		}
		log.Printf("[DB Roles] Migration DSN configured with role: %s", strings.TrimSpace(role))
	}
}
