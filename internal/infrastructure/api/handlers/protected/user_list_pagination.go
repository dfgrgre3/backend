package protected

import (
	"context"

	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"
)

// Sort whitelists — every ?sort= value MUST be one of these; anything else
// falls back to the default sort field. Sort fields end up concatenated into
// SQL by ApplyCursor, so free-form input is never allowed.
var (
	enrolledSubjectsSortWhitelist = map[string]bool{
		"id":          true,
		"enrolled_at": true,
		"created_at":  true,
		"updated_at":  true,
	}
	myCoursesSortWhitelist = map[string]bool{
		"id":         true,
		"created_at": true,
		"updated_at": true,
	}
)

// normalizeListSort returns the requested sort field only if it is in the
// whitelist; otherwise the default sort field is used.
func normalizeListSort(requested string, whitelist map[string]bool, defaultField string) string {
	if requested != "" && whitelist[requested] {
		return requested
	}
	return defaultField
}

// enrollmentCursorValue returns the cursor comparison value for the given
// sort column, keeping cursors consistent between EncodeCursor and
// DecodeCursor across requests.
func enrollmentCursorValue(e models.Enrollment, sortField string) interface{} {
	switch sortField {
	case "enrolled_at":
		return e.EnrolledAt
	case "created_at":
		return e.CreatedAt
	case "updated_at":
		return e.UpdatedAt
	default:
		return e.ID
	}
}

// invalidateUserListCaches clears ONLY the cached list pages affected by an
// enrollment change for ONE user. Redis keys matching the user-scoped pattern
// are deleted (EnhancedCache also clears its process-local L1 as designed).
func invalidateUserListCaches(ctx context.Context, userID string) {
	if userID == "" {
		return
	}
	ec := cache.GetEnhancedCache()
	_ = ec.InvalidatePattern(ctx, cache.ListCachePattern(cache.ListEntityUserSubjects, userID))
	_ = ec.InvalidatePattern(ctx, cache.ListCachePattern(cache.ListEntityMyCourses, userID))
}