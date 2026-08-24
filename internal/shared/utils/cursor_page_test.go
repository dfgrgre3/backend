package pagination

import (
	"encoding/json"
	"testing"
)

func TestNewCursorPage_JSONShape(t *testing.T) {
	b, err := json.Marshal(NewCursorPage([]string{"a", "b"}, "CURSOR-1", true))
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	want := `{"data":["a","b"],"nextCursor":"CURSOR-1","hasNextPage":true}`
	if string(b) != want {
		t.Fatalf("unexpected JSON shape:\n got  %s\n want %s", b, want)
	}
}

func TestNewCursorPage_LastPageOmitsCursor(t *testing.T) {
	b, err := json.Marshal(NewCursorPage([]int{}, "", false))
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	want := `{"data":[],"hasNextPage":false}`
	if string(b) != want {
		t.Fatalf("last page must omit nextCursor:\n got  %s\n want %s", b, want)
	}
}