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

func TestAdminCreateBook_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/books", protected.AdminCreateBook)

	body := map[string]interface{}{
		"title":  "Math Book",
		"author": "Author Name",
		"price":  29.99,
		"isFree": false,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/books", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAdminCreateBook_MissingTitle(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/books", protected.AdminCreateBook)

	body := map[string]interface{}{
		"author": "Author Name",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/books", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminGetBooks_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.Book{
		Title:  "Test Book",
		Author: "Author",
		Price:  19.99,
	})

	router := setupTestRouter()
	router.GET("/books", protected.AdminGetBooks)

	req := httptest.NewRequest(http.MethodGet, "/books", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminUpdateBook_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	book := models.Book{
		Title:  "Old Title",
		Author: "Author",
		Price:  19.99,
	}
	testDB.Create(&book)

	router := setupTestRouter()
	router.PATCH("/books/:id", protected.AdminUpdateBook)

	body := map[string]interface{}{
		"title": "New Title",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/books/"+book.ID, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminDeleteBook_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	book := models.Book{
		Title:  "To Delete",
		Author: "Author",
		Price:  19.99,
	}
	testDB.Create(&book)

	router := setupTestRouter()
	router.DELETE("/books/:id", protected.AdminDeleteBook)

	req := httptest.NewRequest(http.MethodDelete, "/books/"+book.ID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
