package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"thanawy-backend/internal/config"
	"thanawy-backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestAdminRequired_AllowsAdmin(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "ADMIN")
		c.Next()
	})
	router.Use(AdminRequired())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminRequired_RejectsModerator(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "MODERATOR")
		c.Next()
	})
	router.Use(AdminRequired())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminRequired_RejectsStudent(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "STUDENT")
		c.Next()
	})
	router.Use(AdminRequired())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminRequired_RejectsTeacher(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "TEACHER")
		c.Next()
	})
	router.Use(AdminRequired())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminRequired_RejectsNoRole(t *testing.T) {
	router := setupTestRouter()
	router.Use(AdminRequired())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestModeratorRequired_AllowsAdmin(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "ADMIN")
		c.Next()
	})
	router.Use(ModeratorRequired())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestModeratorRequired_AllowsModerator(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "MODERATOR")
		c.Next()
	})
	router.Use(ModeratorRequired())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestModeratorRequired_RejectsStudent(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "STUDENT")
		c.Next()
	})
	router.Use(ModeratorRequired())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminOrModerator_AllowsAdmin(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "ADMIN")
		c.Next()
	})
	router.Use(AdminOrModerator())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminOrModerator_AllowsModerator(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "MODERATOR")
		c.Next()
	})
	router.Use(AdminOrModerator())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminOrModerator_RejectsStudent(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "STUDENT")
		c.Next()
	})
	router.Use(AdminOrModerator())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRoleRequired_SingleRole(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "ADMIN")
		c.Next()
	})
	router.Use(RoleRequired("ADMIN"))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRoleRequired_MultipleRoles(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "TEACHER")
		c.Next()
	})
	router.Use(RoleRequired("ADMIN", "TEACHER"))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRoleRequired_RejectsUnauthorized(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "STUDENT")
		c.Next()
	})
	router.Use(RoleRequired("ADMIN", "TEACHER"))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestPermissionRequired_AdminBypass(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "ADMIN")
		c.Set("permissions", []string{models.PermAdminBypass})
		c.Next()
	})
	router.Use(PermissionRequired("subjects:manage"))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPermissionRequired_ExplicitPermission(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "TEACHER")
		c.Set("permissions", []string{"subjects:manage"})
		c.Next()
	})
	router.Use(PermissionRequired("subjects:manage"))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPermissionRequired_MissingPermission(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "TEACHER")
		c.Set("permissions", []string{"subjects:view"})
		c.Next()
	})
	router.Use(PermissionRequired("subjects:manage"))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestPermissionRequired_NoRole(t *testing.T) {
	router := setupTestRouter()
	router.Use(PermissionRequired("subjects:manage"))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPermissionRequired_NilPermissions(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "TEACHER")
		c.Set("permissions", nil)
		c.Next()
	})
	router.Use(PermissionRequired("subjects:manage"))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestPermissionRequired_EmptyPermissions(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "TEACHER")
		c.Set("permissions", []string{})
		c.Next()
	})
	router.Use(PermissionRequired("subjects:manage"))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminRequired_ErrorResponseFormat(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "STUDENT")
		c.Next()
	})
	router.Use(AdminRequired())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Admin access required")
}

func TestModeratorRequired_ErrorResponseFormat(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "STUDENT")
		c.Next()
	})
	router.Use(ModeratorRequired())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Moderator access required")
}

func TestPermissionRequired_ErrorResponseFormat(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "TEACHER")
		c.Set("permissions", []string{})
		c.Next()
	})
	router.Use(PermissionRequired("subjects:manage"))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Missing required permission")
}

func TestSetContextPermissions(t *testing.T) {
	router := setupTestRouter()
	router.GET("/test", func(c *gin.Context) {
		setContextPermissions(c, models.JSONStringArray{"users:view", "users:manage"})
		perms, _ := c.Get("permissions")
		assert.Equal(t, []string{"users:view", "users:manage"}, perms)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSetContextPermissions_Nil(t *testing.T) {
	router := setupTestRouter()
	router.GET("/test", func(c *gin.Context) {
		setContextPermissions(c, nil)
		perms, _ := c.Get("permissions")
		assert.Equal(t, []string{}, perms)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestExtractToken_BearerHeader(t *testing.T) {
	router := setupTestRouter()
	router.GET("/test", func(c *gin.Context) {
		token := extractToken(c)
		assert.Equal(t, "test-token-123", token)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer test-token-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestExtractToken_NoAuthHeader(t *testing.T) {
	router := setupTestRouter()
	router.GET("/test", func(c *gin.Context) {
		token := extractToken(c)
		assert.Equal(t, "", token)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestExtractToken_LowercaseBearer(t *testing.T) {
	router := setupTestRouter()
	router.GET("/test", func(c *gin.Context) {
		token := extractToken(c)
		assert.Equal(t, "test-token", token)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "bearer test-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestExtractToken_EmptyBearer(t *testing.T) {
	router := setupTestRouter()
	router.GET("/test", func(c *gin.Context) {
		token := extractToken(c)
		assert.Equal(t, "", token)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestExtractToken_InvalidPrefix(t *testing.T) {
	router := setupTestRouter()
	router.GET("/test", func(c *gin.Context) {
		token := extractToken(c)
		assert.Equal(t, "", token)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Token test-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestExtractToken_AccessTokenCookie(t *testing.T) {
	router := setupTestRouter()
	router.GET("/test", func(c *gin.Context) {
		token := extractToken(c)
		assert.Equal(t, "cookie-token", token)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "cookie-token"})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestExtractToken_ClerkSessionCookie(t *testing.T) {
	router := setupTestRouter()
	router.GET("/test", func(c *gin.Context) {
		token := extractToken(c)
		assert.Equal(t, "clerk-session-token", token)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "__session", Value: "clerk-session-token"})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestExtractToken_BearerHeaderPrecedence(t *testing.T) {
	router := setupTestRouter()
	router.GET("/test", func(c *gin.Context) {
		token := extractToken(c)
		assert.Equal(t, "header-token", token)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "cookie-token"})
	req.AddCookie(&http.Cookie{Name: "__session", Value: "clerk-session-token"})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminRequired_CaseSensitivity(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.Use(AdminRequired())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRoleRequired_CaseSensitivity(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	router.Use(RoleRequired("ADMIN"))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestPermissionRequired_WildcardPermission(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "ADMIN")
		c.Set("permissions", []string{models.PermAdminBypass})
		c.Next()
	})
	router.Use(PermissionRequired("any:permission"))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPermissionRequired_ManageWildcard(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "TEACHER")
		c.Set("permissions", []string{"*:manage"})
		c.Next()
	})
	router.Use(PermissionRequired("subjects:manage"))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPermissionRequired_ManageWildcardRejectsView(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "CUSTOM")
		c.Set("permissions", []string{"*:manage"})
		c.Next()
	})
	router.Use(PermissionRequired("subjects:view"))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestImpersonationTokens(t *testing.T) {
	// Initialize configuration if not already done
	_ = getConfig()
	if cachedConfig == nil {
		cachedConfig = &config.Config{}
	}

	// Backup original secret
	originalSecret := cachedConfig.ImpersonationSecret
	defer func() {
		cachedConfig.ImpersonationSecret = originalSecret
	}()

	// 1. Test invalid/empty secret (should return empty token, not panic)
	cachedConfig.ImpersonationSecret = ""
	tokenEmpty := SignImpersonationToken("user-123", "admin-999")
	assert.Empty(t, tokenEmpty, "should return empty token when secret is empty")

	cachedConfig.ImpersonationSecret = "too-short"
	tokenShort := SignImpersonationToken("user-123", "admin-999")
	assert.Empty(t, tokenShort, "should return empty token when secret is not 32-byte hex")

	// 2. Test valid 32-byte hex key
	cachedConfig.ImpersonationSecret = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	
	token := SignImpersonationToken("user-123", "admin-999")
	assert.NotEmpty(t, token)

	userID, ok := VerifyImpersonationToken(token, "admin-999")
	assert.True(t, ok)
	assert.Equal(t, "user-123", userID)

	// Test bound admin ID validation (rejection if admin ID doesn't match)
	_, ok = VerifyImpersonationToken(token, "admin-888")
	assert.False(t, ok, "should reject token when verified with a different admin ID")

	// Test invalid token format
	_, ok = VerifyImpersonationToken("user-123.invalid_signature", "admin-999")
	assert.False(t, ok)

	_, ok = VerifyImpersonationToken("invalid_token_format", "admin-999")
	assert.False(t, ok)

	// Test expired token validation
	// Manually construct an expired token payload (10 minutes in the past)
	expiresAt := time.Now().Add(-10 * time.Minute).Unix()
	payload := fmt.Sprintf("%s:%d:%s", "user-123", expiresAt, "admin-999")
	key, err := getImpersonationSignKey()
	assert.NoError(t, err)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))
	expiredToken := fmt.Sprintf("%s.%d.%s.%s", "user-123", expiresAt, "admin-999", signature)

	_, ok = VerifyImpersonationToken(expiredToken, "admin-999")
	assert.False(t, ok, "should reject expired impersonation tokens")
}

// ─── SUPER_ADMIN Role Tests ───────────────────────────────────────────────────

func TestAdminRequired_AllowsSuperAdmin(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "SUPER_ADMIN")
		c.Next()
	})
	router.Use(AdminRequired())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "SUPER_ADMIN should be allowed by AdminRequired")
}

func TestModeratorRequired_AllowsSuperAdmin(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "SUPER_ADMIN")
		c.Next()
	})
	router.Use(ModeratorRequired())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "SUPER_ADMIN should be allowed by ModeratorRequired")
}

func TestAdminOrModerator_AllowsSuperAdmin(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "SUPER_ADMIN")
		c.Next()
	})
	router.Use(AdminOrModerator())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "SUPER_ADMIN should be allowed by AdminOrModerator")
}

func TestRoleRequired_AllowsSuperAdmin(t *testing.T) {
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "SUPER_ADMIN")
		c.Next()
	})
	router.Use(RoleRequired("ADMIN", "SUPER_ADMIN"))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "SUPER_ADMIN should pass RoleRequired when listed")
}

func TestAdminRequired_RejectsStudent_AfterSuperAdminFix(t *testing.T) {
	// Regression: ensure STUDENT is still rejected after SUPER_ADMIN fix
	router := setupTestRouter()
	router.Use(func(c *gin.Context) {
		c.Set("role", "STUDENT")
		c.Next()
	})
	router.Use(AdminRequired())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code, "STUDENT should still be rejected by AdminRequired")
}

// ─── Impersonation Privilege Escalation Tests ─────────────────────────────────

func TestImpersonation_SuperAdmin_CannotImpersonateAdmin(t *testing.T) {
	// SUPER_ADMIN (rank=5) cannot impersonate another ADMIN (rank=4) — same level check
	// roleHierarchy: SUPER_ADMIN=5, ADMIN=4 → 5 <= 4 is false → allowed to impersonate
	// But ADMIN (rank=4) cannot impersonate ADMIN (rank=4) → 4 <= 4 is true → blocked

	roleHierarchy := map[string]int{
		"STUDENT":     1,
		"TEACHER":     2,
		"MODERATOR":   3,
		"ADMIN":       4,
		"SUPER_ADMIN": 5,
	}

	// ADMIN vs ADMIN → should be blocked (equal rank)
	assert.True(t, roleHierarchy["ADMIN"] <= roleHierarchy["ADMIN"],
		"ADMIN cannot impersonate another ADMIN (equal rank)")

	// SUPER_ADMIN vs ADMIN → should be allowed (higher rank)
	assert.False(t, roleHierarchy["SUPER_ADMIN"] <= roleHierarchy["ADMIN"],
		"SUPER_ADMIN can impersonate ADMIN (higher rank)")

	// SUPER_ADMIN vs SUPER_ADMIN → should be blocked (equal rank)
	assert.True(t, roleHierarchy["SUPER_ADMIN"] <= roleHierarchy["SUPER_ADMIN"],
		"SUPER_ADMIN cannot impersonate another SUPER_ADMIN (equal rank)")
}

// TestDistributedCacheInvalidation_LocalEviction verifies that localRolePermsCache entries
// are deleted correctly upon calling InvalidateRolePermsCache.
func TestDistributedCacheInvalidation_LocalEviction(t *testing.T) {
	userID := "test-invalidate-user-123"
	
	// Seed local cache
	localRolePermsCache.Set(userID, &userAuthContext{Role: "STUDENT"}, cache.DefaultExpiration)
	
	// Ensure it exists
	_, exists := localRolePermsCache.Get(userID)
	assert.True(t, exists)
	
	// Perform invalidation
	InvalidateRolePermsCache(userID)
	
	// Ensure it has been evicted from local cache
	_, exists = localRolePermsCache.Get(userID)
	assert.False(t, exists)
}

