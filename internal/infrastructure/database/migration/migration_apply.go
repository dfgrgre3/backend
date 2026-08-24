package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

func releaseMigrationLock(database *gorm.DB) {
	if err := database.Exec(`SELECT pg_advisory_unlock(hashtext('thanawy_backend_schema_migrations'))`).Error; err != nil {
		log.Printf("failed to release migration lock: %v", err)
	}
}

func ensureMigrationTable(database *gorm.DB) error {
	err := database.Exec(`
		CREATE TABLE IF NOT EXISTS "schema_migrations" (
			id text PRIMARY KEY,
			checksum text NOT NULL,
			"appliedAt" timestamptz NOT NULL DEFAULT now()
		)
	`).Error
	if err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}
	return nil
}

func getMigrationNames() ([]string, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sql" {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func applyMigration(database *gorm.DB, name string) error {
	id := name[:len(name)-len(filepath.Ext(name))]
	contents, err := migrationFiles.ReadFile("migrations/" + name)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}

	if len(contents) == 0 {
		log.Printf("Skipping empty migration file %s", name)
		return nil
	}

	sum := sha256.Sum256(contents)
	checksum := hex.EncodeToString(sum[:])

	// Always read migration records from the write source to avoid reading
	// stale data from a replica that may be behind by a few milliseconds.
	var existing migrationRecord
	err = database.First(&existing, "id = ?", id).Error
	if err == nil {
		if existing.Checksum != checksum {
			return fmt.Errorf("migration %s checksum mismatch: applied checksum %s, file checksum %s. Do not edit applied migrations; create a new migration instead", id, existing.Checksum, checksum)
		}
		return nil
	}

	if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("check migration %s: %w", id, err)
	}

	if strings.Contains(id, "baseline") && shouldSkipBaseline(database) {
		log.Printf("Existing database detected. Marking baseline migration %s as applied without executing.", id)
		return database.Create(&migrationRecord{ID: id, Checksum: checksum, AppliedAt: time.Now().UTC()}).Error
	}

	log.Printf("Applying database migration %s", id)

	// Check if this migration contains non-transactional statements
	if needsNonTransactionalExecution(string(contents)) {
		log.Printf("Migration %s contains non-transactional statements (CONCURRENTLY), executing outside transaction", id)
		return executeNonTransactionalMigration(database, id, string(contents), checksum)
	}

	// For other migrations, use transaction for atomicity
	return database.Transaction(func(tx *gorm.DB) error {
		return executeMigrationStatements(tx, id, string(contents), checksum)
	})
}

// executeNonTransactionalMigration executes a migration that contains statements
// which cannot run inside a transaction (e.g., CREATE INDEX CONCURRENTLY).
// These statements must be executed one by one, outside any transaction.
// We use the raw *sql.DB connection to bypass GORM's transaction handling.
func executeNonTransactionalMigration(database *gorm.DB, id, contents, checksum string) error {
	// Get raw database connection to bypass GORM's transaction handling
	sqlDB, err := database.DB()
	if err != nil {
		return fmt.Errorf("get database connection: %w", err)
	}

	statements := splitSQLStatements(contents)
	executedNonTransactional := false

	for i, stmt := range statements {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if shouldSkipMigrationStatement(id, stmt) {
			continue
		}

		// Check if this specific statement needs non-transactional execution
		upperStmt := strings.ToUpper(stmt)
		isNonTransactional := false
		for _, pattern := range nonTransactionalStatements {
			if strings.Contains(upperStmt, pattern) {
				isNonTransactional = true
				break
			}
		}

		if isNonTransactional {
			// Execute outside transaction using raw sql.DB connection
			log.Printf("Executing non-transactional statement %d: %.50s...", i+1, stmt)
			_, err := sqlDB.Exec(stmt)
			if err != nil {
				return fmt.Errorf("apply non-transactional migration %s statement %d: %w\nStatement: %.200s", id, i+1, err, stmt)
			}
			executedNonTransactional = true
		} else {
			// Execute using GORM for consistency
			if err := database.Exec(stmt).Error; err != nil {
				return fmt.Errorf("apply migration %s statement %d: %w\nStatement: %.200s", id, i+1, err, stmt)
			}
		}
	}

	// Only record the migration after ALL statements have succeeded
	if err := database.Create(&migrationRecord{ID: id, Checksum: checksum, AppliedAt: time.Now().UTC()}).Error; err != nil {
		return fmt.Errorf("record migration %s: %w", id, err)
	}

	if executedNonTransactional {
		log.Printf("Migration %s completed with non-transactional statements", id)
	}
	return nil
}

// shouldSkipBaseline returns true when an existing schema is already in place,
// meaning the baseline migration should be recorded but not executed.
// Uses a single query to check both conditions together.
func shouldSkipBaseline(database *gorm.DB) bool {
	var result struct {
		MigrationCount  int64
		UserTableExists int64
	}
	database.Raw(`
		SELECT
			(SELECT COUNT(*) FROM "schema_migrations") AS migration_count,
			(SELECT COUNT(*) FROM information_schema.tables
			 WHERE table_schema = 'public' AND table_name = 'User') AS user_table_exists
	`).Scan(&result)

	return result.UserTableExists > 0 || result.MigrationCount > 0
}

func executeMigrationStatements(tx *gorm.DB, id, contents, checksum string) error {
	statements := splitSQLStatements(contents)
	if len(statements) == 0 {
		log.Printf("Warning: migration %s contains no executable statements", id)
	}

	for i, stmt := range statements {
		// splitSQLStatements already strips comments; only skip truly empty strings.
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if shouldSkipMigrationStatement(id, stmt) {
			continue
		}

		if err := tx.Exec(stmt).Error; err != nil {
			return fmt.Errorf("apply migration %s statement %d: %w\nStatement: %.200s", id, i+1, err, stmt)
		}
	}

	return tx.Create(&migrationRecord{ID: id, Checksum: checksum, AppliedAt: time.Now().UTC()}).Error
}

func shouldSkipMigrationStatement(id, stmt string) bool {
	if id != "0000_baseline_schema" {
		return false
	}

	normalized := strings.ToLower(strings.Join(strings.Fields(stmt), " "))
	return strings.HasPrefix(normalized, "create table public.schema_migrations ") ||
		strings.HasPrefix(normalized, "alter table only public.schema_migrations add constraint schema_migrations_pkey ") ||
		strings.Contains(normalized, `public."deletedrecordarchive"`) ||
		strings.HasPrefix(normalized, "create index ") ||
		strings.HasPrefix(normalized, "create unique index ") ||
		strings.HasPrefix(normalized, "alter index ")
}
