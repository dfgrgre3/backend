package shared

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

// _ suppresses unused import warning for gin
var _ = gin.H{}
