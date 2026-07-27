package middleware

import (
	"encoding/base64"
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

func newCSRFTestRouter() *gin.Engine {
	r := gin.New()
	r.Use(CSRFMiddleware())
	r.POST("/api/admin/resource", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	return r
}

func validTestCSRFToken() string {
	return base64.URLEncoding.EncodeToString(make([]byte, csrfTokenLength))
}

func TestCSRFMiddleware_AcceptsMatchingCookieAndHeader(t *testing.T) {
	t.Setenv("CORS_ORIGINS", "https://admin.example.com")
	r := newCSRFTestRouter()
	token := validTestCSRFToken()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/resource", nil)
	req.Header.Set("Origin", "https://admin.example.com")
	req.Header.Set(csrfHeaderName, token)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "session"})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestCSRFMiddleware_RejectsMissingHeader(t *testing.T) {
	r := newCSRFTestRouter()
	token := validTestCSRFToken()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/resource", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "session"})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "CSRF token validation failed")
}

func TestCSRFMiddleware_RejectsMismatchedToken(t *testing.T) {
	r := newCSRFTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/resource", nil)
	req.Header.Set(csrfHeaderName, validTestCSRFToken())
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "session"})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: generateCSRFToken()})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
