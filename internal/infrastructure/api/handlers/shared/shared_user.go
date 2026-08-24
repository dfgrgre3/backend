package shared

import (
	"net/http"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

// Shared error message constants.
const (
	ErrUserNotFound         = "User not found"
	AuthRequired            = "Authentication required"
	ErrDBUnavailable        = "Database is temporarily unavailable"
	ErrServiceUnavailable   = "Service temporarily unavailable"
	MsgIDRequired           = "ID is required"
	MsgUserNotAuthenticated = "User not authenticated"
	MsgSubjectNotFound      = "Subject not found"
	MsgInvalidInput         = "Invalid input"
	MsgMethodNotAllowed     = "Method not allowed"
)

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
