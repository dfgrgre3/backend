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

func TestAdminCreateABTest_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/ab-testing", protected.AdminCreateABTest)

	body := map[string]interface{}{
		"name":         "Homepage Test",
		"description":  "Test two homepage variants",
		"status":       "draft",
		"trafficSplit": 50.0,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/ab-testing", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAdminGetABTests_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.ABExperiment{
		Name:       "Test A/B",
		Status:     "draft",
		TrafficPct: 50,
	})

	router := setupTestRouter()
	router.GET("/ab-testing", protected.AdminGetABTests)

	req := httptest.NewRequest(http.MethodGet, "/ab-testing", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminUpdateABTest_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	experiment := models.ABExperiment{
		Name:       "Old Test",
		Status:     "draft",
		TrafficPct: 50,
	}
	testDB.Create(&experiment)

	router := setupTestRouter()
	router.PATCH("/ab-testing/:id", protected.AdminUpdateABTest)

	body := map[string]interface{}{
		"name": "New Test",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/ab-testing/"+experiment.ID, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminDeleteABTest_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	experiment := models.ABExperiment{
		Name:       "To Delete",
		Status:     "draft",
		TrafficPct: 50,
	}
	testDB.Create(&experiment)

	router := setupTestRouter()
	router.DELETE("/ab-testing/:id", protected.AdminDeleteABTest)

	req := httptest.NewRequest(http.MethodDelete, "/ab-testing/"+experiment.ID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
