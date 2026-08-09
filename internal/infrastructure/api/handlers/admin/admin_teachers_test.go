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

func TestCreateTeacher_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/teachers", protected.CreateTeacher)

	body := map[string]interface{}{
		"name": "John Doe",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/teachers", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateTeacher_Duplicate(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.User{
		Email:    "john.doe@thanawy.local",
		Name:     ptr("John Doe"),
		Username: ptr("John Doe"),
		Role:     models.RoleTeacher,
	})

	router := setupTestRouter()
	router.POST("/teachers", protected.CreateTeacher)

	body := map[string]interface{}{
		"name": "John Doe",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/teachers", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestCreateTeacher_MissingName(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/teachers", protected.CreateTeacher)

	body := map[string]interface{}{}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/teachers", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetTeachers_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.User{
		Email: "teacher1@thanawy.local",
		Name:  ptr("Teacher One"),
		Role:  models.RoleTeacher,
	})

	router := setupTestRouter()
	router.GET("/teachers", protected.GetTeachers)

	req := httptest.NewRequest(http.MethodGet, "/teachers", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateTeacher_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	teacher := models.User{
		Email: "teacher@thanawy.local",
		Name:  ptr("Old Name"),
		Role:  models.RoleTeacher,
	}
	testDB.Create(&teacher)

	router := setupTestRouter()
	router.PATCH("/teachers", protected.UpdateTeacher)

	body := map[string]interface{}{
		"id":   teacher.ID,
		"name": "New Name",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/teachers", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateTeacher_NotFound(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.PATCH("/teachers", protected.UpdateTeacher)

	body := map[string]interface{}{
		"id":   "non-existent",
		"name": "New Name",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/teachers", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteTeacher_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	teacher := models.User{
		Email: "todelete@thanawy.local",
		Name:  ptr("To Delete"),
		Role:  models.RoleTeacher,
	}
	testDB.Create(&teacher)

	router := setupTestRouter()
	router.DELETE("/teachers", protected.DeleteTeacher)

	body := map[string]interface{}{
		"id": teacher.ID,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodDelete, "/teachers", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
