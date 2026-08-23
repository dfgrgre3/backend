package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setTestEnv(t *testing.T, key, value string) {
	t.Helper()
	old := os.Getenv(key)
	os.Setenv(key, value)
	t.Cleanup(func() {
		if old == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, old)
		}
	})
}

func unsetTestEnv(t *testing.T, key string) {
	t.Helper()
	old := os.Getenv(key)
	os.Unsetenv(key)
	t.Cleanup(func() {
		if old != "" {
			os.Setenv(key, old)
		}
	})
}

func setAllValidRequiredVars(t *testing.T) {
	setTestEnv(t, "DATABASE_URL", "postgresql://user:pass@localhost:5432/test")
	setTestEnv(t, "S3_ENDPOINT", "s3_endpoint_actual_secret_value_here_and_long_enough")
	setTestEnv(t, "S3_ACCESS_KEY", "s3_access_key_actual_secret_value_here_and_long_enough")
	setTestEnv(t, "S3_SECRET_KEY", "s3_secret_key_actual_secret_value_here_and_long_enough")
	setTestEnv(t, "S3_BUCKET", "s3_bucket_actual_secret_value_here_and_long_enough")
	// DefaultSecretsValidatorConfig() also requires a JWT signing secret
	// (RequireJWTSecret) — part of "all required vars" for that config.
	setTestEnv(t, "JWT_SECRET_KEY", "jwt_signing_secret_actual_value_here_and_long_enough")
}

func TestValidateSecrets_MissingVars(t *testing.T) {
	setAllValidRequiredVars(t)
	unsetTestEnv(t, "DATABASE_URL")
	setTestEnv(t, "NODE_ENV", "production")

	router := setupTestRouter()
	router.Use(ValidateSecrets(DefaultSecretsValidatorConfig()))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestValidateSecrets_PlaceholderValues(t *testing.T) {
	setAllValidRequiredVars(t)
	setTestEnv(t, "DATABASE_URL", "your-database-url-here")
	setTestEnv(t, "NODE_ENV", "production")

	router := setupTestRouter()
	router.Use(ValidateSecrets(DefaultSecretsValidatorConfig()))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestValidateSecrets_ValidSecrets(t *testing.T) {
	setAllValidRequiredVars(t)
	setTestEnv(t, "NODE_ENV", "production")

	router := setupTestRouter()
	router.Use(ValidateSecrets(DefaultSecretsValidatorConfig()))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestValidateSecrets_SkipInDev(t *testing.T) {
	setTestEnv(t, "NODE_ENV", "development")
	unsetTestEnv(t, "DATABASE_URL")

	router := setupTestRouter()
	router.Use(ValidateSecrets(DefaultSecretsValidatorConfig()))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestIsPlaceholderValue(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		val      string
		expected bool
	}{
		{"your- prefix", "JWT_SECRET", "your-secret-here", true},
		{"CHANGE_ME", "DATABASE_URL", "CHANGE_ME", true},
		{"placeholder", "API_KEY", "placeholder-value", true},
		{"example", "SECRET", "example-secret", true},
		{"dummy", "KEY", "dummy-key", true},
		{"test prefix", "TOKEN", "test-token", true},
		{"test in url", "DATABASE_URL", "postgresql://user:pass@localhost:5432/test", false},
		{"short JWT_SECRET", "JWT_SECRET", "short", true},
		{"valid JWT_SECRET", "JWT_SECRET", "a-very-long-and-random-secret-key-12345", false},
		{"valid DATABASE_URL", "DATABASE_URL", "postgresql://user:pass@host:5432/db", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPlaceholderValue(tt.key, tt.val)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateSecrets_CustomVars(t *testing.T) {
	setTestEnv(t, "CUSTOM_SECRET", "CHANGE_ME")
	setTestEnv(t, "NODE_ENV", "production")

	cfg := SecretsValidatorConfig{
		RequiredVars: []string{"CUSTOM_SECRET"},
		SkipInDev:    false,
	}

	router := setupTestRouter()
	router.Use(ValidateSecrets(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestValidateSecrets_AllPlaceholders(t *testing.T) {
	setAllValidRequiredVars(t)
	setTestEnv(t, "DATABASE_URL", "your-database-url")
	setTestEnv(t, "NODE_ENV", "production")

	router := setupTestRouter()
	router.Use(ValidateSecrets(DefaultSecretsValidatorConfig()))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestValidateSecrets_MixedMissingAndPlaceholders(t *testing.T) {
	setAllValidRequiredVars(t)
	setTestEnv(t, "DATABASE_URL", "postgresql://user:pass@localhost:5432/test")
	unsetTestEnv(t, "S3_ENDPOINT")
	setTestEnv(t, "NODE_ENV", "production")

	router := setupTestRouter()
	router.Use(ValidateSecrets(DefaultSecretsValidatorConfig()))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestValidateSecrets_ProductionMode(t *testing.T) {
	setAllValidRequiredVars(t)
	setTestEnv(t, "NODE_ENV", "production")
	unsetTestEnv(t, "DATABASE_URL")

	router := setupTestRouter()
	router.Use(ValidateSecrets(DefaultSecretsValidatorConfig()))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestValidateSecrets_MissingJWTSecret(t *testing.T) {
	setAllValidRequiredVars(t)
	unsetTestEnv(t, "JWT_SECRET_KEY")
	unsetTestEnv(t, "JWT_SECRET")
	setTestEnv(t, "NODE_ENV", "production")

	router := setupTestRouter()
	router.Use(ValidateSecrets(DefaultSecretsValidatorConfig()))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestValidateSecrets_WeakJWTSecret(t *testing.T) {
	setAllValidRequiredVars(t)
	unsetTestEnv(t, "JWT_SECRET_KEY")
	setTestEnv(t, "JWT_SECRET", "too-short")
	setTestEnv(t, "NODE_ENV", "production")

	router := setupTestRouter()
	router.Use(ValidateSecrets(DefaultSecretsValidatorConfig()))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestValidateSecrets_JWTSecretFallbackName(t *testing.T) {
	setAllValidRequiredVars(t)
	// JWT_SECRET_KEY (set by setAllValidRequiredVars) takes priority over
	// JWT_SECRET, matching config.go's getEnv fallback chain — clear it so
	// this test actually exercises the JWT_SECRET fallback path.
	unsetTestEnv(t, "JWT_SECRET_KEY")
	setTestEnv(t, "JWT_SECRET", "jwt_signing_secret_actual_value_here_and_long_enough")
	setTestEnv(t, "NODE_ENV", "production")

	router := setupTestRouter()
	router.Use(ValidateSecrets(DefaultSecretsValidatorConfig()))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestValidateSecrets_CustomConfigDoesNotRequireJWTSecret(t *testing.T) {
	unsetTestEnv(t, "JWT_SECRET_KEY")
	unsetTestEnv(t, "JWT_SECRET")
	setTestEnv(t, "CUSTOM_SECRET", "a_valid_custom_secret_value_here")
	setTestEnv(t, "NODE_ENV", "production")

	cfg := SecretsValidatorConfig{
		RequiredVars: []string{"CUSTOM_SECRET"},
		SkipInDev:    false,
		// RequireJWTSecret intentionally left false.
	}

	router := setupTestRouter()
	router.Use(ValidateSecrets(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestValidateSecrets_EmptyRequiredVars(t *testing.T) {
	setTestEnv(t, "NODE_ENV", "production")

	cfg := SecretsValidatorConfig{
		RequiredVars: []string{},
		SkipInDev:    false,
	}

	router := setupTestRouter()
	router.Use(ValidateSecrets(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
