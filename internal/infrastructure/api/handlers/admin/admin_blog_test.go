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

func TestAdminCreateBlogPost_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/blog", protected.AdminCreateBlogPost)

	body := map[string]interface{}{
		"title":   "New Blog Post",
		"content": "Content here",
		"slug":    "new-blog-post",
		"status":  "DRAFT",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/blog", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAdminGetBlog_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.BlogPost{
		Title:  "Test Post",
		Slug:   "test-post",
		Status: "DRAFT",
	})

	router := setupTestRouter()
	router.GET("/blog", protected.AdminGetBlog)

	req := httptest.NewRequest(http.MethodGet, "/blog", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminUpdateBlogPost_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	post := models.BlogPost{
		Title:  "Old Title",
		Slug:   "old-title",
		Status: "DRAFT",
	}
	testDB.Create(&post)

	router := setupTestRouter()
	router.PATCH("/blog/:id", protected.AdminUpdateBlogPost)

	body := map[string]interface{}{
		"title": "New Title",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/blog/"+post.ID, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminDeleteBlogPost_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	post := models.BlogPost{
		Title:  "To Delete",
		Slug:   "to-delete",
		Status: "DRAFT",
	}
	testDB.Create(&post)

	router := setupTestRouter()
	router.DELETE("/blog/:id", protected.AdminDeleteBlogPost)

	req := httptest.NewRequest(http.MethodDelete, "/blog/"+post.ID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
