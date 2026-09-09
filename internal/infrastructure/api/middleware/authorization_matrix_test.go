package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	models "thanawy-backend/internal/domain/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func runAuthorizationGuard(t *testing.T, guard gin.HandlerFunc, setup func(*gin.Context)) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		setup(c)
		c.Next()
	})
	router.Use(guard)
	router.GET("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	router.ServeHTTP(recorder, request)
	return recorder.Code
}

func TestAuthorizationMatrix_AuthenticatedCourseProgressEnrollmentAndPayment(t *testing.T) {
	for _, guard := range []struct {
		name string
		fn   gin.HandlerFunc
	}{
		{"enrollment", AnyAuthenticatedUser()},
		{"progress", AnyAuthenticatedUser()},
		{"payments", AnyAuthenticatedUser()},
	} {
		t.Run(guard.name, func(t *testing.T) {
			require.Equal(t, http.StatusUnauthorized, runAuthorizationGuard(t, guard.fn, func(*gin.Context) {}))
			require.Equal(t, http.StatusNoContent, runAuthorizationGuard(t, guard.fn, func(c *gin.Context) {
				c.Set("userId", "student-1")
			}))
		})
	}
}

func TestAuthorizationMatrix_TeachingAndQuizRoles(t *testing.T) {
	require.Equal(t, http.StatusForbidden, runAuthorizationGuard(t, TeacherRequired(), func(c *gin.Context) {
		c.Set("user_role", string(models.RoleStudent))
	}))
	require.Equal(t, http.StatusNoContent, runAuthorizationGuard(t, TeacherRequired(), func(c *gin.Context) {
		c.Set("user_role", string(models.RoleTeacher))
	}))

	require.Equal(t, http.StatusForbidden, runAuthorizationGuard(t, StudentRequired(), func(c *gin.Context) {
		c.Set("user_role", string(models.RoleTeacher))
	}))
	require.Equal(t, http.StatusNoContent, runAuthorizationGuard(t, StudentRequired(), func(c *gin.Context) {
		c.Set("user_role", string(models.RoleStudent))
	}))
}

func TestAuthorizationMatrix_AdminAndStoragePermissions(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		method   string
		grant    string
		wantCode int
	}{
		{"course view", "/api/v1/admin/courses", http.MethodGet, models.PermSubjectsView, http.StatusNoContent},
		{"course write without manage", "/api/v1/admin/courses", http.MethodPost, models.PermSubjectsView, http.StatusForbidden},
		{"course write", "/api/v1/admin/courses", http.MethodPost, models.PermSubjectsManage, http.StatusNoContent},
		{"storage view", "/api/v1/admin/upload", http.MethodGet, models.PermResourcesView, http.StatusNoContent},
		{"storage write", "/api/v1/admin/upload", http.MethodPost, models.PermResourcesManage, http.StatusNoContent},
		{"unmapped route", "/api/v1/admin/not-registered", http.MethodGet, models.PermResourcesView, http.StatusForbidden},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("permissions", []string{tc.grant})
				c.Next()
			})
			router.Use(AdminAPIPermissionRequired())
			router.Handle(tc.method, tc.path, func(c *gin.Context) { c.Status(http.StatusNoContent) })

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, tc.path, nil)
			router.ServeHTTP(recorder, request)
			require.Equal(t, tc.wantCode, recorder.Code)
		})
	}
}
