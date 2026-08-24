package cache

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Entity names used by lazy-loaded / infinite-scroll list endpoints.
const (
	ListEntityUserSubjects = "user_subjects"
	ListEntityMyCourses    = "my_courses"
)

// TTL constants for cached list pages.
// Kept intentionally short because enrollment/course state changes affect
// these lists; explicit invalidation hooks are also wired into the
// enroll/unenroll handlers (invalidateUserListCaches).
const (
	TTLUserSubjectsList = 30 * time.Second
	TTLMyCoursesList    = 30 * time.Second
)

// ListQueryIdentity captures EVERY dimension that can change the result of a
// paginated list query. A cache key built from this struct is shared between
// two requests only when every field matches — this is what guarantees that
// one user's cached page can never be served to another user.
//
// This type is deliberately entity-agnostic: it knows nothing about subjects,
// courses or exams, and contains no database logic. Database queries stay in
// the repositories/services/handlers that own each entity; this file only
// standardizes CACHE KEY SHAPE so keys are always deterministic and complete.
type ListQueryIdentity struct {
	Entity  string            // logical list name, e.g. ListEntityUserSubjects
	UserID  string            // data owner (authenticated user id) — REQUIRED for per-user lists
	Cursor  string            // opaque pagination cursor ("" = first page)
	Limit   int               // page size
	Sort    string            // sort signature, e.g. "enrolled_at:desc" (from whitelist)
	Role    string            // requester role when it affects visibility ("-" otherwise)
	Filters map[string]string // additional query filters affecting the result
}

// BuildListCacheKey returns a deterministic cache key for one list page.
// Every identity dimension is embedded in the key:
//
//	<entity>:u:<userID>:c:<cursorHash>:l:<limit>:s:<sort>:r:<role>[:f:<sorted filters>]
//
// Filters are sorted before joining so map iteration order never changes the
// key. Cursors are hashed (not embedded raw) to keep keys compact and
// log-safe while remaining fully discriminating.
func BuildListCacheKey(id ListQueryIdentity) string {
	if id.Entity == "" {
		id.Entity = "list"
	}
	if id.UserID == "" {
		// Defensive default: an unset user id must NEVER collapse keys of
		// different requests together (per-user isolation invariant).
		id.UserID = "anonymous"
	}
	if id.Role == "" {
		id.Role = "-"
	}
	if id.Cursor == "" {
		id.Cursor = "first"
	} else {
		id.Cursor = shortHash(id.Cursor)
	}
	if id.Sort == "" {
		id.Sort = "-"
	}

	key := fmt.Sprintf("%s:u:%s:c:%s:l:%d:s:%s:r:%s",
		id.Entity, id.UserID, id.Cursor, id.Limit, id.Sort, id.Role)

	if len(id.Filters) > 0 {
		parts := make([]string, 0, len(id.Filters))
		for k, v := range id.Filters {
			parts = append(parts, k+"="+v)
		}
		sort.Strings(parts)
		key += ":f:" + strings.Join(parts, ";")
	}
	return key
}

// ListCachePattern returns the SCAN pattern matching every cached page of one
// entity for ONE specific user (pass "*" or "" for all users). Used by the
// enroll/unenroll handlers to invalidate exactly the affected caches.
func ListCachePattern(entity, userID string) string {
	if userID == "" {
		userID = "*"
	}
	return fmt.Sprintf("%s:u:%s:*", entity, userID)
}

func shortHash(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}