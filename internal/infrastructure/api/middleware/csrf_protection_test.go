package middleware

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCSRFMiddleware_SkipsAuthHandshakePaths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(CSRFMiddleware())
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

func TestCSRFMiddleware_AllowsMissingOriginWhenTokenIsValid(t *testing.T) {
	r := newCSRFTestRouter()
	token := validTestCSRFToken()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/resource", nil)
	req.Header.Set(csrfHeaderName, token)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "session"})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestGenerateCSRFToken_IsPaddingFree(t *testing.T) {
	for i := 0; i < 50; i++ {
		token := generateCSRFToken()
		assert.NotContains(t, token, "=", "padded base64 gets percent-encoded by Go's Set-Cookie sanitizer")
		assert.NotContains(t, token, "%", "percent signs would be double-encoded by browsers")
		assert.True(t, isValidCSRFToken(token))
	}
}

func TestIsValidCSRFToken_AcceptsLegacyPaddedTokens(t *testing.T) {
	legacy := base64.URLEncoding.EncodeToString(make([]byte, csrfTokenLength)) // ends with "="
	assert.True(t, isValidCSRFToken(legacy))
	assert.False(t, isValidCSRFToken("short"))
}

// Regression: browsers store Go's percent-encoded padding (_csrf=...%3D)
// verbatim and echo it in X-CSRF-Token, while Go's cookie parser decodes
// %XX back to '='. The comparison must tolerate this asymmetry.
func TestCSRFMiddleware_AcceptsPercentEncodedLegacyCookie(t *testing.T) {
	r := newCSRFTestRouter()
	padded := validTestCSRFToken() // ends with "="
	encoded := strings.ReplaceAll(padded, "=", "%3D")

	req := httptest.NewRequest(http.MethodPost, "/api/admin/resource", nil)
	req.Header.Set(csrfHeaderName, encoded)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "session"})
	req.Header.Set("Cookie", "access_token=session; "+csrfCookieName+"="+encoded)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

// EnsureCSRFToken must be idempotent per request: CSRFMiddleware calls it for
// safe methods and /api/auth/csrf calls it again in its handler. Two different
// Set-Cookie values made tokens ambiguous for clients.
func TestEnsureCSRFToken_EmitsSingleSetCookiePerRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/api/auth/csrf", func(c *gin.Context) {
		CSRFMiddleware()(c) // simulate the global middleware running first
		EnsureCSRFToken(c)  // route handler bootstraps again
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/auth/csrf", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	cookies := w.Result().Cookies()
	assert.Len(t, cookies, 1, "expected exactly one _csrf Set-Cookie")
	token := w.Header().Get(csrfHeaderName)
	assert.Equal(t, cookies[0].Value, token, "cookie value must match X-CSRF-Token")
	assert.NotContains(t, token, "=")
}
