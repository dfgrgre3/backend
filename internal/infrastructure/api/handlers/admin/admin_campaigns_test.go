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

func TestAdminCreateCampaign_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/marketing/campaigns", protected.AdminCreateCampaign)

	body := map[string]interface{}{
		"name":        "Summer Sale",
		"description": "50% off",
		"status":      "active",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/marketing/campaigns", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAdminCreateCampaign_MissingName(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/marketing/campaigns", protected.AdminCreateCampaign)

	body := map[string]interface{}{
		"description": "No name",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/marketing/campaigns", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminGetCampaigns_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.Campaign{
		Name:   "Summer Sale",
		Status: "active",
	})

	router := setupTestRouter()
	router.GET("/marketing/campaigns", protected.AdminGetCampaigns)

	req := httptest.NewRequest(http.MethodGet, "/marketing/campaigns", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminUpdateCampaign_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	campaign := models.Campaign{
		Name:   "Old Campaign",
		Status: "draft",
	}
	testDB.Create(&campaign)

	router := setupTestRouter()
	router.PATCH("/marketing/campaigns/:id", protected.AdminUpdateCampaign)

	body := map[string]interface{}{
		"name":   "New Campaign",
		"status": "active",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/marketing/campaigns/"+campaign.ID, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminDeleteCampaign_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	campaign := models.Campaign{
		Name:   "To Delete",
		Status: "draft",
	}
	testDB.Create(&campaign)

	router := setupTestRouter()
	router.DELETE("/marketing/campaigns/:id", protected.AdminDeleteCampaign)

	req := httptest.NewRequest(http.MethodDelete, "/marketing/campaigns/"+campaign.ID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
