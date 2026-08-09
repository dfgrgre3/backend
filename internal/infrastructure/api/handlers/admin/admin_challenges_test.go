package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	models "thanawy-backend/internal/domain/common"
	protected "thanawy-backend/internal/infrastructure/api/handlers/protected"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/stretchr/testify/assert"
)

func TestAdminCreateChallenge_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/challenges", protected.AdminCreateChallenge)

	body := map[string]interface{}{
		"title":       "Math Challenge",
		"description": "Solve 10 math problems",
		"points":      50,
		"isActive":    true,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/challenges", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAdminGetChallenges_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.Challenge{
		Title:    "Math Challenge",
		XpReward: 50,
		IsActive: true,
	})

	router := setupTestRouter()
	router.GET("/challenges", protected.AdminGetChallenges)

	req := httptest.NewRequest(http.MethodGet, "/challenges", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminUpdateChallenge_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	challenge := models.Challenge{
		Title:    "Old Challenge",
		XpReward: 50,
		IsActive: true,
	}
	testDB.Create(&challenge)

	router := setupTestRouter()
	router.PATCH("/challenges/:id", protected.AdminUpdateChallenge)

	body := map[string]interface{}{
		"title": "New Challenge",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/challenges/"+challenge.ID, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminDeleteChallenge_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	challenge := models.Challenge{
		Title:    "To Delete",
		XpReward: 50,
		IsActive: true,
	}
	testDB.Create(&challenge)

	router := setupTestRouter()
	router.DELETE("/challenges/:id", protected.AdminDeleteChallenge)

	req := httptest.NewRequest(http.MethodDelete, "/challenges/"+challenge.ID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
