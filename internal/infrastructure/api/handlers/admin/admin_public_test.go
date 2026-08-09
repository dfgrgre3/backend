package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	models "thanawy-backend/internal/domain/common"
	protected "thanawy-backend/internal/infrastructure/api/handlers/protected"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/stretchr/testify/assert"
)

func TestGetPublicBlogPosts_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.BlogPost{
		Title:   "Public Post",
		Content: "Content",
		Slug:    "public-post",
		Status:  "PUBLISHED",
	})

	router := setupTestRouter()
	router.GET("/blog/posts", protected.GetPublicBlogPosts)

	req := httptest.NewRequest(http.MethodGet, "/blog/posts", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetPublicBlogPost_NotFound(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.GET("/blog/posts/:slug", protected.GetPublicBlogPost)

	req := httptest.NewRequest(http.MethodGet, "/blog/posts/non-existent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetPublicEvents_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.GET("/events", protected.GetPublicEvents)

	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
