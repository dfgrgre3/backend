package migration

import (
	"embed"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

// Migration running is split across several files in this package (all
// sharing package migration): this file (shared state + Run entrypoint),
// migration_sql_splitter.go (SQL statement splitting) and
// migration_apply.go (per-migration application logic).

// nonTransactionalStatements contains SQL patterns that cannot run inside a transaction.
var nonTransactionalStatements = []string{
	"CREATE INDEX CONCURRENTLY",
	"DROP INDEX CONCURRENTLY",
	"REINDEX CONCURRENTLY",
	"CLUSTER CONCURRENTLY",
	"ANALYZE CONCURRENTLY",
}

// needsNonTransactionalExecution checks if any statement in the migration
// requires execution outside a transaction block.
func needsNonTransactionalExecution(contents string) bool {
	upperContents := strings.ToUpper(contents)
	for _, pattern := range nonTransactionalStatements {
		if strings.Contains(upperContents, pattern) {
			return true
		}
	}
	return false
}

//go:embed migrations/*.sql
var migrationFiles embed.FS

type migrationRecord struct {
	ID        string    `gorm:"primaryKey;column:id"`
	Checksum  string    `gorm:"not null;column:checksum"`
	AppliedAt time.Time `gorm:"not null;column:appliedAt"`
}

func (migrationRecord) TableName() string {
	return "schema_migrations"
}

// Run applies all pending SQL migrations.
func Run(database *gorm.DB) error {
	if database == nil {
		return nil
	}

	// Always acquire the advisory lock on the write source so that concurrent
	// migration runners (e.g. during a rolling deploy) do not race each other.
	writeDB := database.Clauses(dbresolver.Write)

	if err := writeDB.Exec(`SELECT pg_advisory_lock(hashtext('thanawy_backend_schema_migrations'))`).Error; err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer releaseMigrationLock(writeDB)

	if err := ensureMigrationTable(writeDB); err != nil {
		return err
	}

	names, err := getMigrationNames()
	if err != nil {
		return err
	}

	for _, name := range names {
		if err := applyMigration(writeDB, name); err != nil {
			return err
		}
	}

	return nil
}
