package pagination

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Integration tests for ApplyCursor forward/backward paging guarantees:
// no record is ever repeated and none is skipped, including ties on the
// sort column (tie-breaker on the primary key).

type cursorPaginationItem struct {
	ID    string `gorm:"primaryKey;column:id"`
	Value int64  `gorm:"column:value"`
}

func (cursorPaginationItem) TableName() string { return "cursor_pagination_items" }

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&cursorPaginationItem{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	return db
}

func paginateAll(t *testing.T, db *gorm.DB, pg CursorPagination) []string {
	t.Helper()
	var order []string
	seen := map[string]bool{}
	for page := 0; page < 100; page++ {
		q, err := pg.ApplyCursor(db.Model(&cursorPaginationItem{}))
		if err != nil {
			t.Fatalf("ApplyCursor failed: %v", err)
		}
		var rows []cursorPaginationItem
		if err := q.Find(&rows).Error; err != nil {
			t.Fatalf("find failed: %v", err)
		}
		hasNext := len(rows) > pg.Limit
		if hasNext {
			rows = rows[:pg.Limit]
		}
		for _, r := range rows {
			if seen[r.ID] {
				t.Fatalf("record %q appeared twice — cursor pagination duplicated data", r.ID)
			}
			seen[r.ID] = true
			order = append(order, r.ID)
		}
		if !hasNext {
			return order
		}
		last := rows[len(rows)-1]
		pg.Cursor = EncodeCursor(CursorData{ID: last.ID, Value: last.Value, SortField: pg.SortField})
	}
	t.Fatal("pagination did not terminate")
	return nil
}

func TestApplyCursor_ForwardPagingNoDuplicatesNoSkips(t *testing.T) {
	db := newTestDB(t)

	total := 25
	for i := 1; i <= total; i++ {
		item := cursorPaginationItem{ID: fmt.Sprintf("item-%02d", i), Value: int64(i)}
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}

	pg := DefaultPagination()
	pg.Limit = 10
	pg.SortField = "value"
	pg.SortOrder = "desc"

	got := paginateAll(t, db, pg)
	if len(got) != total {
		t.Fatalf("expected %d records, collected %d", total, len(got))
	}
	// Descending order: first page starts at item-25.
	if got[0] != "item-25" || got[total-1] != "item-01" {
		t.Fatalf("unexpected ordering: first=%s last=%s", got[0], got[total-1])
	}
}

func TestApplyCursor_TiesHandledByTieBreakerWithoutDupOrSkip(t *testing.T) {
	db := newTestDB(t)

	values := []struct {
		id    string
		value int64
	}{
		{"a", 5}, {"b", 5}, {"c", 5}, {"d", 4}, {"e", 4}, {"f", 3},
	}
	for _, it := range values {
		item := cursorPaginationItem{ID: it.id, Value: it.value}
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}

	pg := DefaultPagination()
	pg.Limit = 2
	pg.SortField = "value"
	pg.SortOrder = "asc"

	got := paginateAll(t, db, pg)
	if len(got) != len(values) {
		t.Fatalf("expected %d records with ties handled, collected %d", len(values), len(got))
	}
}

func TestEncodeDecodeCursor_RoundTrip(t *testing.T) {
	now := time.Now()
	original := CursorData{ID: "id-1", Value: now.Format(time.RFC3339Nano), SortField: "created_at"}
	encoded := EncodeCursor(original)
	if encoded == "" {
		t.Fatal("EncodeCursor returned empty string")
	}
	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeCursor failed: %v", err)
	}
	if decoded.ID != original.ID || decoded.SortField != original.SortField || decoded.Value != original.Value {
		t.Fatalf("round-trip mismatch: %+v vs %+v", decoded, original)
	}
}

func TestDecodeCursor_RejectsGarbage(t *testing.T) {
	if _, err := DecodeCursor("not-a-valid-cursor!!!"); err == nil {
		t.Fatal("garbage cursor must be rejected (opaque + validated)")
	}
}