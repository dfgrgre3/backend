// Package shared contains constants and helper functions used by more than one
// handler package (admin, protected, ...).
package shared

import (
	"net/http"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
