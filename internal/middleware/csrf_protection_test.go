package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCSRFProtection_SkipsAuthHandshakePaths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(CSRFProtection())
	r.POST("/api/auth/login", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
