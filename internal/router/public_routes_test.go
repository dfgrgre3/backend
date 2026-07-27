package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSetupPublicRoutes_AuthCSRFEndpointBootstrapsToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	SetupPublicRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/csrf", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	setCookie := w.Header().Get("Set-Cookie")
	require.Contains(t, setCookie, "_csrf=")
}
