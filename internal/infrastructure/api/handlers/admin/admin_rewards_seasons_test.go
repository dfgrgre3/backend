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

func TestAdminCreateReward_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/rewards", protected.AdminCreateReward)

	body := map[string]interface{}{
		"name":        "Free Month",
		"description": "Get a free month",
		"cost":        500,
		"type":        "subscription",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/rewards", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAdminGetRewards_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.Reward{
		Title:       "Free Month",
		Description: "Get a free month",
		Cost:        500,
		Type:        "subscription",
	})

	router := setupTestRouter()
	router.GET("/rewards", protected.AdminGetRewards)

	req := httptest.NewRequest(http.MethodGet, "/rewards", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminUpdateReward_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	reward := models.Reward{
		Title:       "Old Reward",
		Description: "Old Desc",
		Cost:        100,
		Type:        "discount",
	}
	testDB.Create(&reward)

	router := setupTestRouter()
	router.PATCH("/rewards/:id", protected.AdminUpdateReward)

	body := map[string]interface{}{
		"name": "New Reward",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/rewards/"+reward.ID, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminDeleteReward_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	reward := models.Reward{
		Title: "To Delete",
		Cost:  100,
		Type:  "discount",
	}
	testDB.Create(&reward)

	router := setupTestRouter()
	router.DELETE("/rewards/:id", protected.AdminDeleteReward)

	req := httptest.NewRequest(http.MethodDelete, "/rewards/"+reward.ID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminCreateSeason_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/seasons", protected.AdminCreateSeason)

	body := map[string]interface{}{
		"name":     "Summer 2024",
		"isActive": true,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/seasons", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAdminGetSeasons_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.Season{
		Title:    "Summer 2024",
		IsActive: true,
	})

	router := setupTestRouter()
	router.GET("/seasons", protected.AdminGetSeasons)

	req := httptest.NewRequest(http.MethodGet, "/seasons", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminUpdateSeason_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	season := models.Season{
		Title:    "Old Season",
		IsActive: false,
	}
	testDB.Create(&season)

	router := setupTestRouter()
	router.PATCH("/seasons/:id", protected.AdminUpdateSeason)

	body := map[string]interface{}{
		"name":     "New Season",
		"isActive": true,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/seasons/"+season.ID, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminDeleteSeason_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	season := models.Season{
		Title:    "To Delete",
		IsActive: false,
	}
	testDB.Create(&season)

	router := setupTestRouter()
	router.DELETE("/seasons/:id", protected.AdminDeleteSeason)

	req := httptest.NewRequest(http.MethodDelete, "/seasons/"+season.ID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
