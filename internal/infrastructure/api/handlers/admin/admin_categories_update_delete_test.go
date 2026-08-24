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

func TestUpdateCategory_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	category := models.Category{Name: "Old Name", Slug: "old-name", Type: models.CategoryTypeCourse}
	testDB.Create(&category)

	router := setupTestRouter()
	router.PATCH("/categories", protected.UpdateCategory)

	body := map[string]interface{}{"id": category.ID, "name": "New Name"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/categories", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateCategory_NotFound(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.PATCH("/categories", protected.UpdateCategory)

	body := map[string]interface{}{"id": "non-existent-id", "name": "New Name"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/categories", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteCategory_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	category := models.Category{Name: "To Delete", Slug: "to-delete", Type: models.CategoryTypeCourse}
	testDB.Create(&category)

	router := setupTestRouter()
	router.DELETE("/categories", protected.DeleteCategory)

	body := map[string]interface{}{"id": category.ID}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodDelete, "/categories", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteCategory_WithSubjects(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	category := models.Category{Name: "With Subjects", Slug: "with-subjects", Type: models.CategoryTypeCourse}
	testDB.Create(&category)

	testDB.Create(&models.Subject{Name: "Test Subject", CategoryId: &category.ID})

	router := setupTestRouter()
	router.DELETE("/categories", protected.DeleteCategory)

	body := map[string]interface{}{"id": category.ID}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodDelete, "/categories", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
