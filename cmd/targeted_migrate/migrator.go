package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

type migrationRecord struct {
	ID        string    `gorm:"primaryKey;column:id"`
	Checksum  string    `gorm:"not null;column:checksum"`
	AppliedAt time.Time `gorm:"not null;column:appliedAt"`
}

func (migrationRecord) TableName() string { return "schema_migrations" }

type migrationResult int

const (
	migrationSkipped migrationResult = iota
	migrationRegistered
	migrationApplied
)

type migrationResults struct {
	applied    int
	registered int
	skipped    int
}

// migrationsDir is the on-disk location of the SQL migration files, relative to
// the repository root (this command must be run from there).
const migrationsDir = "internal/infrastructure/database/migration/migrations"

func ensureMigrationsTable(db *gorm.DB) {
	db.Exec(`CREATE TABLE IF NOT EXISTS "schema_migrations" (id text PRIMARY KEY, checksum text NOT NULL, "appliedAt" timestamptz NOT NULL DEFAULT now())`)
}

func readMigrationFiles() []string {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		log.Fatalf("Read migrations: %v", err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

func processAllMigrations(db *gorm.DB, names []string, knownApplied map[string]bool) migrationResults {
	results := migrationResults{}

	for _, name := range names {
		id := strings.TrimSuffix(name, ".sql")
		checksum := computeChecksum(name)

		result := processMigration(db, name, id, checksum, knownApplied)
		switch result {
		case migrationSkipped:
			results.skipped++
		case migrationRegistered:
			results.registered++
		case migrationApplied:
			results.applied++
		}
	}
	return results
}

func computeChecksum(name string) string {
	contents, err := os.ReadFile(migrationsDir + "/" + name)
	if err != nil {
		log.Fatalf("Read %s: %v", name, err)
	}

	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}

func knownAppliedMigrations() map[string]bool {
	return map[string]bool{
		"0000_baseline_schema":                         true,
		"0001_add_user_session":                        true,
		"0021_add_missing_tables":                      true,
		"0022_fix_notification_table":                  true,
		"0023_add_foreign_keys":                        true,
		"0024_add_check_constraints":                   true,
		"0025_add_not_null_unique_constraints":         true,
		"0026_add_performance_indexes":                 true,
		"0027_create_materialized_views":               true, // superseded by 0033
		"0028_create_analytics_event_log":              true, // Prisma schema differs
		"0029_cleanup_constraints_and_integrity":       true,
		"0030_table_partitioning":                      true,
		"0031_enforce_critical_constraints":            true,
		"0033_fix_materialized_views":                  true,
		"0039_cuid_function":                           true,
		"0043_database_health_hardening":               true,
		"0044_safe_database_optimization":              true,
		"0045_auth_log_uuid_and_password_hash_compat":  true,
		"0046_add_missing_user_archive_reason":         true,
		"0047_user_session_schema_compat":              true,
		"0048_security_and_subscription_schema_compat": true,
		"0049_security_invoker_active_enrollments":     true,
		"0050_add_broadcasts_and_push_tokens":          true,
		"0051_fix_course_review_schema":                true,
		"0052_default_reviews_visible":                 true,
		"0053_lesson_attachment_compat":                true,
		"0054_fix_user_permissions_jsonb":              true,
		"0055_add_certificates":                        true,
		"0055_add_super_admin_role":                    true,
		"0056_add_lesson_table":                        true,
	}
}

func processMigration(db *gorm.DB, name, id, checksum string, knownApplied map[string]bool) migrationResult {
	if isAlreadyTracked(db, id) {
		return migrationSkipped
	}

	if knownApplied[id] {
		registerMigration(db, id, checksum)
		return migrationRegistered
	}

	applyMigration(db, name, id, checksum)
	return migrationApplied
}

func isAlreadyTracked(db *gorm.DB, id string) bool {
	var existing migrationRecord
	dbErr := db.First(&existing, "id = ?", id).Error
	if dbErr == nil {
		log.Printf("  ↷ Already tracked: %s", id)
		return true
	}
	if dbErr != gorm.ErrRecordNotFound {
		log.Fatalf("Check %s: %v", id, dbErr)
	}
	return false
}

func registerMigration(db *gorm.DB, id, checksum string) {
	if err := db.Create(&migrationRecord{ID: id, Checksum: checksum, AppliedAt: time.Now().UTC()}).Error; err != nil {
		log.Fatalf("Register %s: %v", id, err)
	}
	log.Printf("  ✎ Registered (already applied): %s", id)
}

func applyMigration(db *gorm.DB, name, id, checksum string) {
	log.Printf("Applying migration: %s", name)
	txErr := db.Transaction(func(tx *gorm.DB) error {
		stmts := splitSQL(readMigrationContent(name))
		for i, stmt := range stmts {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" || strings.HasPrefix(stmt, "--") {
				continue
			}
			if err := tx.Exec(stmt).Error; err != nil {
				return fmt.Errorf("statement %d: %w\nSQL: %.300s", i+1, err, stmt)
			}
		}
		return tx.Create(&migrationRecord{ID: id, Checksum: checksum, AppliedAt: time.Now().UTC()}).Error
	})
	if txErr != nil {
		log.Fatalf("FAILED migration %s: %v", name, txErr)
	}
	log.Printf("  ✓ Applied: %s", name)
}

func readMigrationContent(name string) string {
	contents, err := os.ReadFile(migrationsDir + "/" + name)
	if err != nil {
		log.Fatalf("Read %s: %v", name, err)
	}
	return string(contents)
}
