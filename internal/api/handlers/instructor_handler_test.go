package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeInstructorName_UsesEmailPrefixAsFallback(t *testing.T) {
	assert.Equal(t, "alice", normalizeInstructorName("", "alice@example.com"))
}

func TestGetInstructors_ReturnsInstructorList(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	name := "Alice Teacher"
	bio := "Math instructor"
	require.NoError(t, testDB.Create(&models.User{
		Email:                 "alice@thanawy.local",
		Name:                  &name,
		Username:              &name,
		PasswordHash:          "hashed",
		Role:                  models.RoleTeacher,
		Bio:                   &bio,
		InstructorStatus:      "PENDING",
		InstructorSpecialties: models.JSONStringArray{"Math"},
		InstructorLanguages:   models.JSONStringArray{"Arabic"},
	}).Error)

	router := setupTestRouter()
	router.GET("/instructors", GetInstructors)

	req := httptest.NewRequest(http.MethodGet, "/instructors", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	instructors, ok := payload["instructors"].([]interface{})
	require.True(t, ok)
	assert.Len(t, instructors, 1)
}

func TestApproveInstructor_UpdatesStatus(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	name := "Bob Teacher"
	teacher := models.User{
		Email:            "bob@thanawy.local",
		Name:             &name,
		Username:         &name,
		PasswordHash:     "hashed",
		Role:             models.RoleTeacher,
		InstructorStatus: "PENDING",
	}
	require.NoError(t, testDB.Create(&teacher).Error)

	router := setupTestRouter()
	router.POST("/instructors/:id/approve", ApproveInstructor)

	body := map[string]interface{}{}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/instructors/"+teacher.ID+"/approve", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteInstructor_ReturnsNotFoundForMissingRecord(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.DELETE("/instructors/:id", DeleteInstructor)

	req := httptest.NewRequest(http.MethodDelete, "/instructors/missing-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateInstructor_PreservesUsernameWhenNameOnlyIsProvided(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	name := "Carol Teacher"
	username := "carolteacher"
	teacher := models.User{
		Email:            "carol@thanawy.local",
		Name:             &name,
		Username:         &username,
		PasswordHash:     "hashed",
		Role:             models.RoleTeacher,
		InstructorStatus: "PENDING",
	}
	require.NoError(t, testDB.Create(&teacher).Error)

	router := setupTestRouter()
	router.PATCH("/instructors/:id", UpdateInstructor)

	body := map[string]interface{}{"name": "Carol Updated"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/instructors/"+teacher.ID, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var updated models.User
	require.NoError(t, testDB.Where("id = ?", teacher.ID).First(&updated).Error)
	assert.Equal(t, username, *updated.Username)
}

func TestUpdateInstructor_ReturnsUpdatedInstructorPayload(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	name := "Dina Teacher"
	username := "dina"
	teacher := models.User{
		Email:            "dina@thanawy.local",
		Name:             &name,
		Username:         &username,
		PasswordHash:     "hashed",
		Role:             models.RoleTeacher,
		InstructorStatus: "PENDING",
	}
	require.NoError(t, testDB.Create(&teacher).Error)

	router := setupTestRouter()
	router.PATCH("/instructors/:id", UpdateInstructor)

	body := map[string]interface{}{"name": "Dina Updated"}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/instructors/"+teacher.ID, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	instructor, ok := payload["instructor"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Dina Updated", instructor["name"])
}
