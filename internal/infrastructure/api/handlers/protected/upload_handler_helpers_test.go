package protected

import (
	"strings"
	"testing"
)

func TestBuildFilenameCreatesDistinctKeysForSameFilename(t *testing.T) {
	first := buildFilename("uploads", ".png")
	second := buildFilename("uploads", ".png")

	if first == second {
		t.Fatalf("same-name uploads must receive distinct storage keys: %q", first)
	}
	for _, key := range []string{first, second} {
		if !strings.HasPrefix(key, "uploads/") || !strings.HasSuffix(key, ".png") {
			t.Fatalf("unexpected generated storage key %q", key)
		}
	}
}
