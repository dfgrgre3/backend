package main

import (
	"log"
)

// fix_migrations.go: Marks all previously-applied migrations as applied in schema_migrations,
// then applies any truly pending ones.
// Run from repository root.

func main() {
	db := initDB()
	ensureMigrationsTable(db)
	names := readMigrationFiles()
	knownApplied := knownAppliedMigrations()
	results := processAllMigrations(db, names, knownApplied)

	log.Printf("\nDone. Applied: %d, Registered: %d, Skipped: %d", results.applied, results.registered, results.skipped)
}
