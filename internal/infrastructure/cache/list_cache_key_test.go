package cache

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestBuildListCacheKey_DeterministicRegardlessOfFilterOrder(t *testing.T) {
	a := BuildListCacheKey(ListQueryIdentity{
		Entity: ListEntityUserSubjects, UserID: "u1", Limit: 20,
		Filters: map[string]string{"level": "sec1", "category": "math"},
	})
	b := BuildListCacheKey(ListQueryIdentity{
		Entity: ListEntityUserSubjects, UserID: "u1", Limit: 20,
		Filters: map[string]string{"category": "math", "level": "sec1"},
	})
	if a != b {
		t.Fatalf("filter order must not change the key:\n a=%s\n b=%s", a, b)
	}
}

// SECURITY INVARIANT: user A must never be able to obtain user B's cached page.
func TestBuildListCacheKey_UserIsolation(t *testing.T) {
	userA := BuildListCacheKey(ListQueryIdentity{Entity: ListEntityUserSubjects, UserID: "user-A", Limit: 20})
	userB := BuildListCacheKey(ListQueryIdentity{Entity: ListEntityUserSubjects, UserID: "user-B", Limit: 20})
	if userA == userB {
		t.Fatal("different users must never share a cache key")
	}

	// An unset userID must not collapse into any real user's key either.
	anon := BuildListCacheKey(ListQueryIdentity{Entity: ListEntityUserSubjects, Limit: 20})
	if anon == userA || anon == userB {
		t.Fatal("unset userID collapsed with a real user key")
	}

	key := BuildListCacheKey(ListQueryIdentity{Entity: ListEntityUserSubjects, UserID: "user-A"})
	if !strings.Contains(key, ":u:user-A:") {
		t.Fatalf("userID must be embedded in the key, got %q", key)
	}
}

func TestBuildListCacheKey_EveryDimensionChangesTheKey(t *testing.T) {
	base := BuildListCacheKey(ListQueryIdentity{
		Entity: "entity", UserID: "u", Limit: 20, Sort: "updated_at:desc",
	})

	cases := map[string]ListQueryIdentity{
		"different limit":  {Entity: "entity", UserID: "u", Limit: 50, Sort: "updated_at:desc"},
		"different cursor": {Entity: "entity", UserID: "u", Limit: 20, Sort: "updated_at:desc", Cursor: "curs0r"},
		"different sort":   {Entity: "entity", UserID: "u", Limit: 20, Sort: "enrolled_at:desc"},
		"different role":   {Entity: "entity", UserID: "u", Limit: 20, Sort: "updated_at:desc", Role: "admin"},
		"different filter": {Entity: "entity", UserID: "u", Limit: 20, Sort: "updated_at:desc", Filters: map[string]string{"level": "s1"}},
	}
	for name, id := range cases {
		if BuildListCacheKey(id) == base {
			t.Errorf("%s produced an identical key (must differ)", name)
		}
	}

	// Same inputs => same key (cursor hashing is deterministic).
	same := BuildListCacheKey(ListQueryIdentity{Entity: "entity", UserID: "u", Limit: 20, Sort: "updated_at:desc"})
	if same != base {
		t.Error("identical identities must produce identical keys")
	}
}

func TestListCachePattern_UserScoped(t *testing.T) {
	if got := ListCachePattern(ListEntityUserSubjects, "user-A"); got != "user_subjects:u:user-A:*" {
		t.Fatalf("unexpected pattern %q", got)
	}
	if got := ListCachePattern(ListEntityMyCourses, ""); got != "my_courses:u:*:*" {
		t.Fatalf("unexpected all-users pattern %q", got)
	}
}

// Cache semantics on the existing EnhancedCache (L1-only, Redis nil):
//
//   - miss does NOT serve data
//   - store-then-read is a hit (no second DB load needed)
//   - invalidation prevents stale reads
//   - user B cannot read user A's entry
func TestEnhancedCache_ListPageHitMissAndInvalidation_L1Only(t *testing.T) {
	ec := NewEnhancedCache(100)
	ctx := context.Background()

	keyA := BuildListCacheKey(ListQueryIdentity{Entity: ListEntityUserSubjects, UserID: "user-A", Limit: 20})
	keyB := BuildListCacheKey(ListQueryIdentity{Entity: ListEntityUserSubjects, UserID: "user-B", Limit: 20})

	pageA := map[string]interface{}{"owner": "user-A", "nextCursor": "abc"}

	// 1) Miss before anything is stored.
	var dest map[string]string
	if err := ec.Get(ctx, keyA, &dest); err == nil {
		t.Fatal("expected cache miss before store")
	}

	// 2) Store after "DB load", then hit without hitting the loader again.
	if err := ec.Set(ctx, keyA, pageA, time.Minute); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	var hit map[string]string
	if err := ec.Get(ctx, keyA, &hit); err != nil {
		t.Fatalf("expected hit after store: %v", err)
	}
	if hit["owner"] != "user-A" || hit["nextCursor"] != "abc" {
		t.Fatalf("cached payload mismatch: %+v", hit)
	}

	// 3) User B cannot read user A's cached page.
	if err := ec.Get(ctx, keyB, &hit); err == nil && hit["owner"] == "user-A" {
		t.Fatal("SECURITY: user B read user A's cached page")
	}

	// 4) Invalidation removes exactly user A's pages; stale reads are gone.
	if err := ec.InvalidatePattern(ctx, ListCachePattern(ListEntityUserSubjects, "user-A")); err != nil {
		t.Fatalf("InvalidatePattern failed: %v", err)
	}
	if err := ec.Get(ctx, keyA, &hit); err == nil {
		t.Fatal("stale data served after invalidation")
	}
}