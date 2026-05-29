package db

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
