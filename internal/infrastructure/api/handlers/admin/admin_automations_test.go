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

func TestAdminCreateAutomation_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/automations", protected.AdminCreateAutomation)

	body := map[string]interface{}{
		"name":     "Auto Email",
		"type":     "email",
		"trigger":  "user_signup",
		"action":   "send_welcome_email",
		"isActive": true,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/automations", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAdminGetAutomations_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.Automation{
		Name:     "Auto Email",
		Event:    "email",
		IsActive: true,
	})

	router := setupTestRouter()
	router.GET("/automations", protected.AdminGetAutomations)

	req := httptest.NewRequest(http.MethodGet, "/automations", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminUpdateAutomation_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	automation := models.Automation{
		Name:     "Old Automation",
		Event:    "email",
		IsActive: false,
	}
	testDB.Create(&automation)

	router := setupTestRouter()
	router.PATCH("/automations/:id", protected.AdminUpdateAutomation)

	body := map[string]interface{}{
		"name":     "New Automation",
		"isActive": true,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/automations/"+automation.ID, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminDeleteAutomation_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	automation := models.Automation{
		Name:     "To Delete",
		Event:    "email",
		IsActive: false,
	}
	testDB.Create(&automation)

	router := setupTestRouter()
	router.DELETE("/automations/:id", protected.AdminDeleteAutomation)

	req := httptest.NewRequest(http.MethodDelete, "/automations/"+automation.ID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
