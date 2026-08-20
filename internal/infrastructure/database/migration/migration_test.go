package migration

import "testing"

func TestDatabaseHealthHardeningMigrationIsExecutable(t *testing.T) {
	stmt := `CREATE INDEX IF NOT EXISTS idx_subject_active_rows
		ON public."Subject" (id)
		WHERE deleted_at IS NULL;`

	if shouldSkipMigrationStatement("0043_database_health_hardening", stmt) {
		t.Fatal("database health hardening migration must not be skipped")
	}
}

func TestSafeDatabaseOptimizationMigrationIsExecutable(t *testing.T) {
	stmt := `SELECT pg_temp.create_index_if_columns(
		'public."Subject"',
		ARRAY['deleted_at', 'created_at', 'id'],
		'CREATE INDEX IF NOT EXISTS idx_subject_created_active_safe ON public."Subject" (created_at DESC, id) WHERE deleted_at IS NULL'
	);`

	if shouldSkipMigrationStatement("0044_safe_database_optimization", stmt) {
		t.Fatal("safe database optimization migration must not be skipped")
	}
}

func TestSplitSQLStatementsHandlesDollarQuotedBlocks(t *testing.T) {
	sql := `BEGIN;
DO $$
BEGIN
	IF true THEN
		RAISE NOTICE 'semicolon; inside block';
	END IF;
END $$;
COMMIT;`

	statements := splitSQLStatements(sql)
	if len(statements) != 3 {
		t.Fatalf("expected 3 statements, got %d: %#v", len(statements), statements)
	}
}

func TestAllEmbeddedMigrationFilesAreValidAndParsable(t *testing.T) {
	names, err := getMigrationNames()
	if err != nil {
		t.Fatalf("failed to get migration names: %v", err)
	}

	if len(names) == 0 {
		t.Fatal("no migration files found")
	}

	for _, name := range names {
		contents, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatalf("failed to read migration %s: %v", name, err)
		}

		if len(contents) == 0 {
			continue
		}

		statements := splitSQLStatements(string(contents))
		if len(statements) == 0 {
			t.Errorf("migration %s has non-empty file but 0 parsed statements", name)
		}
	}
}

