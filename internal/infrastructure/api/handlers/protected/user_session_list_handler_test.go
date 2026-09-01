package protected

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSessionTestDB(t *testing.T) {
	t.Helper()
	original := db.DB
	t.Cleanup(func() { db.DB = original })

	testDB := setupTestDB(t)
	require.NoError(t, testDB.AutoMigrate(&models.UserSession{}))
	db.DB = testDB
}

func seedSessions(t *testing.T) {
	t.Helper()
	now := time.Now()
	sessions := []models.UserSession{
		{UserID: "11111111-1111-1111-1111-111111111111", RefreshToken: "seed-token-1", IP: "1.1.1.1", Browser: "Chrome", OS: "Windows", Status: "active", IsActive: true, LastAccessed: now.Add(-1 * time.Hour), ExpiresAt: now.Add(24 * time.Hour), AbsoluteExpiresAt: now.Add(48 * time.Hour), CreatedAt: now.Add(-3 * time.Hour), UpdatedAt: now.Add(-1 * time.Hour)},
		{UserID: "11111111-1111-1111-1111-111111111111", RefreshToken: "seed-token-2", IP: "1.1.1.2", Browser: "Firefox", OS: "Linux", Status: "revoked", IsActive: false, LastAccessed: now.Add(-2 * time.Hour), ExpiresAt: now.Add(24 * time.Hour), AbsoluteExpiresAt: now.Add(48 * time.Hour), CreatedAt: now.Add(-4 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour)},
		{UserID: "22222222-2222-2222-2222-222222222222", RefreshToken: "seed-token-3", IP: "2.2.2.2", Browser: "Safari", OS: "macOS", Status: "active", IsActive: true, LastAccessed: now.Add(-30 * time.Minute), ExpiresAt: now.Add(24 * time.Hour), AbsoluteExpiresAt: now.Add(48 * time.Hour), CreatedAt: now.Add(-1 * time.Hour), UpdatedAt: now.Add(-30 * time.Minute)},
	}
	for i := range sessions {
		require.NoError(t, db.DB.Create(&sessions[i]).Error)
	}
	// The IsActive column has a gorm default:true tag, so a seeded zero value
	// (false) is silently replaced by the default on insert. Force the revoked
	// state explicitly so the filter assertions see the intended rows.
	require.NoError(t, db.DB.Model(&models.UserSession{}).
		Where("id = ?", sessions[1].ID).
		Updates(map[string]interface{}{"is_active": false, "status": "revoked"}).Error)
}

func TestListUserSessions_Paginated(t *testing.T) {
	setupSessionTestDB(t)
	seedSessions(t)

	router := setupTestRouter()
	router.GET("/user-sessions", ListUserSessions)

	req := httptest.NewRequest(http.MethodGet, "/user-sessions?page=1&limit=2", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Items []map[string]interface{} `json:"items"`
			Pagination struct {
				Page       int   `json:"page"`
				Limit      int   `json:"limit"`
				Total      int64 `json:"total"`
				TotalPages int64 `json:"totalPages"`
			} `json:"pagination"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	assert.True(t, body.Success)
	assert.Len(t, body.Data.Items, 2)
	assert.Equal(t, int64(3), body.Data.Pagination.Total)
	assert.Equal(t, int64(2), body.Data.Pagination.TotalPages)
	assert.Equal(t, 2, body.Data.Pagination.Limit)
	// Most recently accessed session first.
	assert.Equal(t, "2.2.2.2", body.Data.Items[0]["ip"])
}

func TestListUserSessions_Filters(t *testing.T) {
	setupSessionTestDB(t)
	seedSessions(t)

	router := setupTestRouter()
	router.GET("/user-sessions", ListUserSessions)

	// active=true returns only the two active sessions.
	req := httptest.NewRequest(http.MethodGet, "/user-sessions?active=true", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data struct {
			Items []map[string]interface{} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Data.Items, 2)
	for _, item := range body.Data.Items {
		assert.Equal(t, true, item["isActive"])
	}

	// userId narrows to a single user's sessions.
	req = httptest.NewRequest(http.MethodGet, "/user-sessions?userId=22222222-2222-2222-2222-222222222222", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Data.Items, 1)
	assert.Equal(t, "2.2.2.2", body.Data.Items[0]["ip"])
	assert.Equal(t, "22222222-2222-2222-2222-222222222222", body.Data.Items[0]["userId"])
}