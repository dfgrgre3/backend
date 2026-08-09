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

func TestAdminCreateAchievement_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/achievements", protected.AdminCreateAchievement)

	body := map[string]interface{}{
		"key":         "first_achievement",
		"title":       "First Achievement",
		"description": "Complete first task",
		"xpReward":    100,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/achievements", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAdminGetAchievements_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.Achievement{
		Key:         "achievement_1",
		Title:       "Achievement 1",
		Description: "Desc 1",
		XpReward:    100,
	})

	router := setupTestRouter()
	router.GET("/achievements", protected.AdminGetAchievements)

	req := httptest.NewRequest(http.MethodGet, "/achievements", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminUpdateAchievement_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	achievement := models.Achievement{
		Key:         "old_key",
		Title:       "Old Name",
		Description: "Old Desc",
		XpReward:    100,
	}
	testDB.Create(&achievement)

	router := setupTestRouter()
	router.PATCH("/achievements/:id", protected.AdminUpdateAchievement)

	body := map[string]interface{}{
		"name": "New Name",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/achievements/"+achievement.ID, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminDeleteAchievement_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	achievement := models.Achievement{
		Key:      "to_delete",
		Title:    "To Delete",
		XpReward: 100,
	}
	testDB.Create(&achievement)

	router := setupTestRouter()
	router.DELETE("/achievements/:id", protected.AdminDeleteAchievement)

	req := httptest.NewRequest(http.MethodDelete, "/achievements/"+achievement.ID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
