package queries

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	db "thanawy-backend/internal/infrastructure/database"
)

// sqlCaptureWriter implements gorm/logger.Writer so the exact generated SQL can be
// inspected by the test.
type sqlCaptureWriter struct {
	buf bytes.Buffer
}

func (w *sqlCaptureWriter) Printf(format string, args ...interface{}) {
	fmt.Fprintf(&w.buf, format+"\n", args...)
}

func (w *sqlCaptureWriter) String() string { return w.buf.String() }

// TestGetSuggestionsSQLIsolation is a regression test for the GORM shared-statement
// pollution bug that made GET /api/ai/recommendations return HTTP 500.
//
// Previously, the two query chains inside GetSuggestions (preferredCategories +
// searchHistory, and the main "Subject" s chain) were all built on the same root
// *gorm.DB (ReadDB) without isolating each chain in its own session. GORM mutates the
// statement of a non-clone *gorm.DB when chaining methods, so the leftover
// `JOIN "Subject" s ON s.id = se.subject_id` and the subject-enrollment WHERE clauses
// from the preferredCategories chain leaked into the base chain, generating invalid
// SQL like:
//
//	SELECT COUNT(DISTINCT("s"."category_id"))
//	FROM "Subject" s JOIN "Subject" s ON s.id = se.subject_id
//	WHERE (se.user_id = $1 ...) AND (s.deleted_at IS NULL ...) ...
//
// which PostgreSQL rejects with SQLSTATE 42712 ("table name \"s\" specified more
// than once").
//
// The test executes the real service method against an in-memory SQLite database and
// asserts both the returned page's correctness and the shape of the generated SQL.
func TestGetSuggestionsSQLIsolation(t *testing.T) {
	logWriter := &sqlCaptureWriter{}
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.New(logWriter, logger.Config{
			LogLevel: logger.Info,
			Colorful: false,
		}),
	})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	// Keep a single connection so the in-memory database is shared by every query.
	if sqlDB, dbErr := gormDB.DB(); dbErr == nil {
		sqlDB.SetMaxOpenConns(1)
	}

	exec := func(q string) {
		t.Helper()
		if err := gormDB.Exec(q).Error; err != nil {
			t.Fatalf("setup SQL failed: %v\nSQL: %s", err, q)
		}
	}

	exec(`CREATE TABLE "Subject" (
		id TEXT, name TEXT, name_ar TEXT, description TEXT,
		category_id TEXT, rating REAL, enrolled_count INTEGER,
		duration_hours INTEGER, level TEXT, thumbnail_url TEXT,
		instructor_id TEXT, status TEXT,
		is_published INTEGER, is_active INTEGER,
		deleted_at TEXT, created_at TEXT, updated_at TEXT
	)`)
	exec(`CREATE TABLE "SubjectEnrollment" (
		id TEXT, user_id TEXT, subject_id TEXT,
		enrolled_at TEXT, updated_at TEXT, deleted_at TEXT
	)`)
	exec(`CREATE TABLE "Category" (id TEXT, name TEXT, deleted_at TEXT)`)
	exec(`CREATE TABLE "search_history" (
		id TEXT, user_id TEXT, query TEXT, created_at DATETIME
	)`)
	exec(`INSERT INTO "Category" (id, name) VALUES ('c1', 'علوم'), ('c2', 'رياضيات')`)
	exec(`INSERT INTO "Subject"
		(id, name, name_ar, description, category_id, rating, enrolled_count, duration_hours, level, thumbnail_url, instructor_id, status, is_published, is_active, created_at, updated_at)
		VALUES
		('sA', 'Chemistry', 'كيمياء', NULL, 'c1', 4.5, 200, 20, 'intermediate', NULL, 't1', 'PUBLISHED', 1, 1, '2026-01-01', '2026-01-01'),
		('sB', 'Math', 'رياضيات', NULL, 'c2', 5.0, 10, 15, 'beginner', NULL, 't2', 'PUBLISHED', 1, 1, '2026-01-02', '2026-01-02'),
		('sC', 'SecretPhy', 'فيزياء', NULL, 'c1', 4.0, 5, 10, 'beginner', NULL, 't3', 'DRAFT', 0, 0, '2026-01-03', '2026-01-03'),
		('sD', 'Biology', 'أحياء', NULL, 'c1', 4.0, 50, 12, 'advanced', NULL, 't4', 'PUBLISHED', 1, 1, '2026-01-04', '2026-01-04')`)
	exec(`INSERT INTO "SubjectEnrollment" (id, user_id, subject_id, enrolled_at, updated_at)
		VALUES ('e1', 'u1', 'sA', '2026-01-05', '2026-01-05')`)
	exec(`INSERT INTO "search_history" (id, user_id, query, created_at)
		VALUES ('h1', 'u1', 'كيمياء', '2026-01-06 12:00:00')`)

	useDB(t, gormDB)

	svc := NewCourseSuggestionsQueryService()
	page, err := svc.GetSuggestions("u1", 1, 8)
	if err != nil {
		t.Fatalf("GetSuggestions returned error: %v\nGenerated SQL:\n%s", err, logWriter.String())
	}

	if page.Total != 2 {
		t.Errorf("expected Total=2 (sB and sD, excluding enrolled sA and draft sC), got %d\nGenerated SQL:\n%s", page.Total, logWriter.String())
	}
	if len(page.Recommendations) != 2 {
		t.Fatalf("expected 2 recommendations, got %d\nGenerated SQL:\n%s", len(page.Recommendations), logWriter.String())
	}
	if len(page.SearchHistory) != 1 {
		t.Errorf("expected 1 search history entry, got %d", len(page.SearchHistory))
	}

	// Courses in the learner's preferred category (c1) must rank first.
	if page.Recommendations[0].ID != "sD" || page.Recommendations[1].ID != "sB" {
		t.Errorf("unexpected ordering: first=%s second=%s\nGenerated SQL:\n%s",
			page.Recommendations[0].ID, page.Recommendations[1].ID, logWriter.String())
	}

	sqlLog := logWriter.String()

	// Regression: the Subject base query must never combine with the enrollment join.
	if strings.Contains(sqlLog, `FROM "Subject" s JOIN "Subject" s`) {
		t.Errorf("duplicated Subject alias regression detected (SQLSTATE 42712)\nGenerated SQL:\n%s", sqlLog)
	}
	// The SubjectEnrollment -> Subject join may only appear in the preferredCategories
	// query, exactly once.
	if got := strings.Count(sqlLog, `JOIN "Subject" s ON s.id = se.subject_id`); got != 1 {
		t.Errorf("expected exactly 1 SubjectEnrollment join, got %d\nGenerated SQL:\n%s", got, sqlLog)
	}
	// The count statement must not reference the enrollment table ('se') alias.
	for _, line := range strings.Split(sqlLog, "\n") {
		if strings.Contains(line, "count(*)") && strings.Contains(line, "se.") {
			t.Errorf("count query leaked enrollment chain clauses:\n%s", line)
		}
	}
}

// useDB points the package-level database singleton at the provided in-memory DB for
// the duration of the test.
func useDB(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	prev := db.DB
	db.DB = gormDB
	t.Cleanup(func() { db.DB = prev })
}

// setupSuggestionTestDB provisions an in-memory SQLite database with the minimal
// schema and seed data needed by CourseSuggestionsQueryService:
//
//	c1, c2  -> categories
//	sA      -> enrolled by u1 (must be excluded from suggestions)
//	sD      -> published + active, category c1 (learner's preferred category)
//	sB      -> published + active, category c2
//	sC      -> DRAFT (must be excluded, even though it is category c1)
func setupSuggestionTestDB(t *testing.T) (*gorm.DB, *sqlCaptureWriter) {
	t.Helper()

	logWriter := &sqlCaptureWriter{}
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.New(logWriter, logger.Config{
			LogLevel: logger.Info,
			Colorful: false,
		}),
	})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	// Keep a single connection so the in-memory database is shared by every query.
	if sqlDB, dbErr := gormDB.DB(); dbErr == nil {
		sqlDB.SetMaxOpenConns(1)
	}

	exec := func(q string) {
		t.Helper()
		if err := gormDB.Exec(q).Error; err != nil {
			t.Fatalf("setup SQL failed: %v\nSQL: %s", err, q)
		}
	}

	exec(`CREATE TABLE "Subject" (
		id TEXT, name TEXT, name_ar TEXT, description TEXT,
		category_id TEXT, rating REAL, enrolled_count INTEGER,
		duration_hours INTEGER, level TEXT, thumbnail_url TEXT,
		instructor_id TEXT, status TEXT,
		is_published INTEGER, is_active INTEGER,
		deleted_at TEXT, created_at TEXT, updated_at TEXT
	)`)
	exec(`CREATE TABLE "SubjectEnrollment" (
		id TEXT, user_id TEXT, subject_id TEXT,
		enrolled_at TEXT, updated_at TEXT, deleted_at TEXT
	)`)
	exec(`CREATE TABLE "Category" (id TEXT, name TEXT, deleted_at TEXT)`)
	exec(`CREATE TABLE "search_history" (
		id TEXT, user_id TEXT, query TEXT, created_at DATETIME
	)`)

	exec(`INSERT INTO "Category" (id, name) VALUES ('c1', 'علوم'), ('c2', 'رياضيات')`)
	exec(`INSERT INTO "Subject"
		(id, name, name_ar, description, category_id, rating, enrolled_count, duration_hours, level, thumbnail_url, instructor_id, status, is_published, is_active, created_at, updated_at)
		VALUES
		('sA', 'Chemistry', 'كيمياء', NULL, 'c1', 4.5, 200, 20, 'intermediate', NULL, 't1', 'PUBLISHED', 1, 1, '2026-01-01', '2026-01-01'),
		('sB', 'Math', 'رياضيات', NULL, 'c2', 5.0, 10, 15, 'beginner', NULL, 't2', 'PUBLISHED', 1, 1, '2026-01-02', '2026-01-02'),
		('sC', 'SecretPhy', 'فيزياء', NULL, 'c1', 4.0, 5, 10, 'beginner', NULL, 't3', 'DRAFT', 0, 0, '2026-01-03', '2026-01-03'),
		('sD', 'Biology', 'أحياء', NULL, 'c1', 4.0, 50, 12, 'advanced', NULL, 't4', 'PUBLISHED', 1, 1, '2026-01-04', '2026-01-04')`)
	exec(`INSERT INTO "SubjectEnrollment" (id, user_id, subject_id, enrolled_at, updated_at)
		VALUES ('e1', 'u1', 'sA', '2026-01-05', '2026-01-05')`)
	exec(`INSERT INTO "search_history" (id, user_id, query, created_at)
		VALUES ('h1', 'u1', 'كيمياء', '2026-01-06 12:00:00')`)

	return gormDB, logWriter
}

// TestPreferredCategoriesPopulated is a probe for the preferredCategories pluck: the
// enrollment chain must return the learner's enrolled course categories so the main
// query can rank matching courses first.
func TestPreferredCategoriesPopulated(t *testing.T) {
	gormDB, logWriter := setupSuggestionTestDB(t)
	useDB(t, gormDB)

	svc := NewCourseSuggestionsQueryService()
	// Replicate the exact connection used inside GetSuggestions (db.ReadDB), not the
	// raw db.DB, so the probe exercises the same Session/Clauses chain.
	cats, err := svc.preferredCategories(db.ReadDB(), "u1")
	if err != nil {
		t.Fatalf("preferredCategories returned error: %v\nGenerated SQL:\n%s", err, logWriter.String())
	}
	t.Logf("preferredCategories = %v\nGenerated SQL:\n%s", cats, logWriter.String())

	if !reflect.DeepEqual(cats, []string{"c1"}) {
		t.Errorf("expected preferredCategories [c1], got %v", cats)
	}
}
