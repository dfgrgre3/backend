// Package shared contains constants and helper functions used by more than one
// handler package (admin, protected, ...).
package shared

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Shared query constants
const (
	IDQuery             = "id = ?"
	StatusQuery         = "status = ?"
	IDInQuery           = "id IN ?"
	CreatedAtDescSort   = "\"created_at\" desc"
	QueryRole           = "role = ?"
	CreatedAtRangeQuery = "\"created_at\" >= ? AND \"created_at\" < ?"
	CreatedAtGte        = "\"created_at\" >= ?"
	IsActiveQuery       = "is_active = ?"
	DateFormat          = "2006-01-02"
	CoalesceSumDuration = "COALESCE(SUM(duration_min), 0)"
)

// Shared error message constants
const (
	ErrUserNotFound         = "User not found"
	AuthRequired            = "Authentication required"
	ErrDBUnavailable        = "Database is temporarily unavailable"
	ErrServiceUnavailable   = "Service temporarily unavailable"
	MsgIDRequired           = "ID is required"
	MsgUserNotAuthenticated = "User not authenticated"
	MsgSubjectNotFound      = "Subject not found"
	MsgInvalidInput         = "Invalid input"
)

// SafeDB returns a non-nil *gorm.DB instance after checking db.DB.
// Returns true (abort) if DB is nil and writes a 503 response.
func SafeDB(c *gin.Context) (*gorm.DB, bool) {
	if db.DB == nil {
		api_response.Error(c, http.StatusServiceUnavailable, ErrDBUnavailable)
		return nil, true
	}
	return db.DB, false
}

// SafeReadDB returns a ReadDB or falls back to db.DB, checking for nil.
// Returns true (abort) if DB is nil.
func SafeReadDB(c *gin.Context) (*gorm.DB, bool) {
	if db.DB == nil {
		api_response.Error(c, http.StatusServiceUnavailable, ErrDBUnavailable)
		return nil, true
	}
	database := db.ReadDB()
	if database == nil {
		database = db.DB
	}
	return database, false
}

// StringOrEmpty safely dereferences a *string pointer, returning "" if nil.
func StringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// FirstNonEmpty returns the first non-empty string from the given values.
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// ParsePositiveInt parses value as a positive int, returning fallback otherwise.
func ParsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

// CalculateTotalPages returns the number of pages for the given total and limit.
func CalculateTotalPages(total int64, limit int) int64 {
	if limit <= 0 {
		return 1
	}
	pages := total / int64(limit)
	if total%int64(limit) != 0 {
		pages++
	}
	if pages == 0 {
		return 1
	}
	return pages
}

// UserNameLookup resolves user IDs to a human readable display name.
func UserNameLookup(userIDs []string) map[string]string {
	names := make(map[string]string, len(userIDs))
	unique := make([]string, 0, len(userIDs))
	seen := make(map[string]struct{}, len(userIDs))
	for _, id := range userIDs {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return names
	}

	database := db.ReadDB()
	if database == nil {
		database = db.DB
	}
	if database == nil {
		return names
	}

	var users []models.User
	database.Where(IDInQuery, unique).Find(&users)
	for _, u := range users {
		names[u.ID] = FirstNonEmpty(StringOrEmpty(u.Name), StringOrEmpty(u.Username), u.Email)
	}
	return names
}

// IsDuplicateKeyError checks if the error is a unique constraint violation.
func IsDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "23505") ||
		strings.Contains(msg, "record already exists")
}

// SafeCreate attempts to create a record and returns a friendly error if it's a duplicate.
func SafeCreate(database *gorm.DB, value interface{}) error {
	err := database.Create(value).Error
	if IsDuplicateKeyError(err) {
		return errors.New("record already exists")
	}
	return err
}

// UpsertBy performs a FirstOrCreate upsert using the given query conditions.
// Returns true if the record was created, false if it already existed.
func UpsertBy(database *gorm.DB, query interface{}, args []interface{}, value interface{}) (bool, error) {
	result := database.Where(query, args...).First(value)
	if result.Error == nil {
		return false, nil
	}
	if result.Error != gorm.ErrRecordNotFound {
		return false, result.Error
	}
	err := database.Clauses(clause.OnConflict{DoNothing: true}).Create(value).Error
	if IsDuplicateKeyError(err) {
		database.Where(query, args...).First(value)
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// CreateOrAssign performs an upsert: finds existing record or creates new one with assigned values.
func CreateOrAssign(database *gorm.DB, query interface{}, args []interface{}, value interface{}, assigns map[string]interface{}) error {
	result := database.Where(query, args...).First(value)
	if result.Error == nil {
		if len(assigns) > 0 {
			return database.Model(value).Updates(assigns).Error
		}
		return nil
	}
	if result.Error != gorm.ErrRecordNotFound {
		return result.Error
	}
	conds := append([]interface{}{query}, args...)
	err := database.Assign(assigns).FirstOrCreate(value, conds...).Error
	if IsDuplicateKeyError(err) {
		return database.Where(query, args...).First(value).Error
	}
	return err
}

// GetAuthenticatedUserID extracts and validates the authenticated user ID from context.
func GetAuthenticatedUserID(c *gin.Context) (string, bool) {
	userIdValue, exists := c.Get("userId")
	if !exists || userIdValue == nil {
		api_response.Error(c, http.StatusUnauthorized, MsgUserNotAuthenticated)
		return "", false
	}
	userId, ok := userIdValue.(string)
	if !ok {
		api_response.Error(c, http.StatusInternalServerError, "Invalid user ID type")
		return "", false
	}
	return userId, true
}
