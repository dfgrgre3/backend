package protected

import (
	"thanawy-backend/internal/infrastructure/api/handlers/shared"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// common.go re-exports the shared handler helpers under the unexported names
// historically used across this package. The canonical implementations live in
// the handlers/shared package so that the admin package can reuse them.

// Shared query constants
const (
	idQuery             = shared.IDQuery
	queryID             = shared.IDQuery // alias used in admin handlers
	statusQuery         = shared.StatusQuery
	idInQuery           = shared.IDInQuery
	createdAtDescSort   = shared.CreatedAtDescSort
	queryRole           = shared.QueryRole
	createdAtRangeQuery = shared.CreatedAtRangeQuery
	createdAtGte        = shared.CreatedAtGte
	isActiveQuery       = shared.IsActiveQuery
	dateFormat          = shared.DateFormat
)

// Shared error message constants
const (
	errUserNotFound         = shared.ErrUserNotFound
	authRequired            = shared.AuthRequired
	errDBUnavailable        = shared.ErrDBUnavailable
	errServiceUnavailable   = shared.ErrServiceUnavailable
	msgIDRequired           = shared.MsgIDRequired
	msgUserNotAuthenticated = shared.MsgUserNotAuthenticated
	msgSubjectNotFound      = shared.MsgSubjectNotFound
	msgInvalidInput         = shared.MsgInvalidInput
	msgMethodNotAllowed     = shared.MsgMethodNotAllowed
)

// IsDuplicateKeyError checks if the error is a unique constraint violation.
func IsDuplicateKeyError(err error) bool { return shared.IsDuplicateKeyError(err) }

// SafeCreate attempts to create a record and returns a friendly error if it's a duplicate.
func SafeCreate(db *gorm.DB, value interface{}) error { return shared.SafeCreate(db, value) }

// UpsertBy performs a FirstOrCreate upsert using the given query conditions.
func UpsertBy(db *gorm.DB, query interface{}, args []interface{}, value interface{}) (bool, error) {
	return shared.UpsertBy(db, query, args, value)
}

// CreateOrAssign performs an upsert: finds existing record or creates new one with assigned values.
func CreateOrAssign(db *gorm.DB, query interface{}, args []interface{}, value interface{}, assigns map[string]interface{}) error {
	return shared.CreateOrAssign(db, query, args, value, assigns)
}

func safeDB(c *gin.Context) (*gorm.DB, bool)     { return shared.SafeDB(c) }
func safeReadDB(c *gin.Context) (*gorm.DB, bool) { return shared.SafeReadDB(c) }
func stringOrEmpty(s *string) string             { return shared.StringOrEmpty(s) }
func firstNonEmpty(values ...string) string      { return shared.FirstNonEmpty(values...) }
func userNameLookup(userIDs []string) map[string]string {
	return shared.UserNameLookup(userIDs)
}

func getAuthenticatedUserID(c *gin.Context) (string, bool) {
	return shared.GetAuthenticatedUserID(c)
}
