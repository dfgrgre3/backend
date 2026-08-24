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

func TestCreateCategory_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/categories", protected.CreateCategory)

	body := map[string]interface{}{"name": "Mathematics", "description": "Math courses"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/categories", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateCategory_Duplicate(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.Category{Name: "Mathematics", Slug: "mathematics", Type: models.CategoryTypeCourse})

	router := setupTestRouter()
	router.POST("/categories", protected.CreateCategory)

	body := map[string]interface{}{"name": "Mathematics"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/categories", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestCreateCategory_MissingName(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/categories", protected.CreateCategory)

	body := map[string]interface{}{"description": "No name"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/categories", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateCategory_WithType(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/categories", protected.CreateCategory)

	body := map[string]interface{}{"name": "Library Books", "type": "LIBRARY"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/categories", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}
