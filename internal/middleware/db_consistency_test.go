package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"thanawy-backend/internal/db"
)

func TestDBConsistencyMiddleware_ForcesSourceAfterWrite(t *testing.T) {
	gormDB := newConsistencyTestDB(t)
	oldRedis := db.Redis
	db.Redis = nil
	t.Cleanup(func() { db.Redis = oldRedis })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Next()
	})
	router.Use(dbConsistencyMiddleware(ctx, gormDB, time.Second, time.Hour))
	router.Any("/test", func(c *gin.Context) {
		_, exists := c.Get("db")
		c.JSON(http.StatusOK, gin.H{"forced": exists})
	})

	writeReq := httptest.NewRequest(http.MethodPost, "/test", nil)
	writeResp := httptest.NewRecorder()
	router.ServeHTTP(writeResp, writeReq)
	assert.Equal(t, http.StatusOK, writeResp.Code)

	readReq := httptest.NewRequest(http.MethodGet, "/test", nil)
	readResp := httptest.NewRecorder()
	router.ServeHTTP(readResp, readReq)

	assert.Equal(t, http.StatusOK, readResp.Code)
	assert.JSONEq(t, `{"forced":true}`, readResp.Body.String())
}

func TestDBConsistencyMiddleware_StrongHeaderForcesSource(t *testing.T) {
	gormDB := newConsistencyTestDB(t)
	oldRedis := db.Redis
	db.Redis = nil
	t.Cleanup(func() { db.Redis = oldRedis })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	router := setupTestRouter()
	router.Use(dbConsistencyMiddleware(ctx, gormDB, time.Second, time.Hour))
	router.GET("/test", func(c *gin.Context) {
		_, exists := c.Get("db")
		c.JSON(http.StatusOK, gin.H{"forced": exists})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Consistency-Level", "strong")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.JSONEq(t, `{"forced":true}`, resp.Body.String())
}

func TestDBConsistencyMiddleware_DoesNotForceAfterWindow(t *testing.T) {
	gormDB := newConsistencyTestDB(t)
	oldRedis := db.Redis
	db.Redis = nil
	t.Cleanup(func() { db.Redis = oldRedis })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	router := setupTestRouter()
	router.Use(dbConsistencyMiddleware(ctx, gormDB, time.Nanosecond, time.Hour))
	router.Any("/test", func(c *gin.Context) {
		_, exists := c.Get("db")
		c.JSON(http.StatusOK, gin.H{"forced": exists})
	})

	writeReq := httptest.NewRequest(http.MethodPatch, "/test", nil)
	writeResp := httptest.NewRecorder()
	router.ServeHTTP(writeResp, writeReq)
	assert.Equal(t, http.StatusOK, writeResp.Code)

	time.Sleep(time.Millisecond)

	readReq := httptest.NewRequest(http.MethodGet, "/test", nil)
	readResp := httptest.NewRecorder()
	router.ServeHTTP(readResp, readReq)

	assert.Equal(t, http.StatusOK, readResp.Code)
	assert.JSONEq(t, `{"forced":false}`, readResp.Body.String())
}

func newConsistencyTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}

	return gormDB
}
