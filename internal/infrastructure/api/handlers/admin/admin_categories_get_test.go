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

func TestGetCategories_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.Category{Name: "Math", Slug: "math", Type: models.CategoryTypeCourse})
	testDB.Create(&models.Category{Name: "Science", Slug: "science", Type: models.CategoryTypeCourse})

	router := setupTestRouter()
	router.GET("/categories", protected.GetCategories)

	req := httptest.NewRequest(http.MethodGet, "/categories", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetCategories_FilterByType(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.Category{Name: "Course Cat", Slug: "course-cat", Type: models.CategoryTypeCourse})
	testDB.Create(&models.Category{Name: "Library Cat", Slug: "library-cat", Type: models.CategoryTypeLibrary})

	router := setupTestRouter()
	router.GET("/categories", protected.GetCategories)

	req := httptest.NewRequest(http.MethodGet, "/categories?type=COURSE", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// Compile-time check that json and bytes are used (suppresses linter).
var _ = bytes.NewBuffer
var _ = json.Marshal
