package admin

import (
	"thanawy-backend/internal/infrastructure/api/handlers/shared"
	"thanawy-backend/internal/infrastructure/cache"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// common.go re-exports the shared handler helpers under the unexported names
// used across this package. The canonical implementations live in the
// handlers/shared package, which is also used by the protected package.

// Shared query constants
const (
	idQuery             = shared.IDQuery
	statusQuery         = shared.StatusQuery
	idInQuery           = shared.IDInQuery
	createdAtDescSort   = shared.CreatedAtDescSort
	createdAtRangeQuery = shared.CreatedAtRangeQuery
	createdAtGte        = shared.CreatedAtGte
	dateFormat          = shared.DateFormat
	coalesceSumDuration = shared.CoalesceSumDuration
	isActiveQuery       = shared.IsActiveQuery
	queryRole           = shared.QueryRole
	userIDQuery         = "user_id = ?"
)

var calculateTotalPages = shared.CalculateTotalPages

// Shared error message constants
const (
	errDBUnavailable        = shared.ErrDBUnavailable
	msgUserNotAuthenticated = shared.MsgUserNotAuthenticated
)

// IsDuplicateKeyError checks if the error is a unique constraint violation.
func IsDuplicateKeyError(err error) bool { return shared.IsDuplicateKeyError(err) }

// SafeCreate attempts to create a record and returns a friendly error if it's a duplicate.
func SafeCreate(db *gorm.DB, value interface{}) error { return shared.SafeCreate(db, value) }

// LogAudit logs an administrative action asynchronously.
func LogAudit(c *gin.Context, action string, resource string, resourceId string, metadata interface{}) {
	shared.LogAudit(c, action, resource, resourceId, metadata)
}

// InvalidateSettingsCache drops the cached admin settings.
func InvalidateSettingsCache() { cache.InvalidateSettingsCache() }

func safeDB(c *gin.Context) (*gorm.DB, bool)     { return shared.SafeDB(c) }
func safeReadDB(c *gin.Context) (*gorm.DB, bool) { return shared.SafeReadDB(c) }
func stringOrEmpty(s *string) string             { return shared.StringOrEmpty(s) }
func firstNonEmpty(values ...string) string      { return shared.FirstNonEmpty(values...) }

func getAuthenticatedUserID(c *gin.Context) (string, bool) {
	return shared.GetAuthenticatedUserID(c)
}
