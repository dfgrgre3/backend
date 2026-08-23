package middleware

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

type SecretsValidatorConfig struct {
	RequiredVars []string
	SkipInDev    bool
	// RequireJWTSecret additionally validates the JWT signing secret
	// (JWT_SECRET_KEY or JWT_SECRET — see validateJWTSecret). It's a separate
	// flag rather than a RequiredVars entry because the two names are
	// interchangeable aliases (config.go accepts either), which a flat
	// RequiredVars list can't express. Scoped to an opt-in flag, not applied
	// unconditionally to every SecretsValidatorConfig, so callers building a
	// minimal custom config for an unrelated secret aren't forced to also
	// provision a JWT secret.
	RequireJWTSecret bool
}

func DefaultSecretsValidatorConfig() SecretsValidatorConfig {
	return SecretsValidatorConfig{
		RequiredVars: []string{
			"DATABASE_URL",
			"S3_ENDPOINT",
			"S3_ACCESS_KEY",
			"S3_SECRET_KEY",
			"S3_BUCKET",
		},
		SkipInDev:        true,
		RequireJWTSecret: true,
	}
}

func ValidateSecrets(cfg SecretsValidatorConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.SkipInDev && os.Getenv("NODE_ENV") == "development" {
			c.Next()
			return
		}

		var missing []string
		var placeholder []string

		for _, key := range cfg.RequiredVars {
			val := os.Getenv(key)
			if val == "" {
				missing = append(missing, key)
				continue
			}
			if isPlaceholderValue(key, val) {
				placeholder = append(placeholder, key)
			}
		}

		if cfg.RequireJWTSecret {
			validateJWTSecret(&missing, &placeholder)
		}

		if len(missing) > 0 || len(placeholder) > 0 {
			log.Printf("[SECURITY] Missing secrets: %v, Placeholders: %v", missing, placeholder)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error":       "Service configuration incomplete",
				"missing":     missing,
				"placeholder": placeholder,
			})
			return
		}

		c.Next()
	}
}

func isPlaceholderValue(key, val string) bool {
	placeholders := []string{
		"your-",
		"change_me",
		"placeholder",
		"example",
		"dummy",
	}
	lower := strings.ToLower(val)
	for _, p := range placeholders {
		if strings.HasPrefix(lower, p) || strings.Contains(lower, p) {
			return true
		}
	}
	if strings.HasPrefix(lower, "test-") || strings.HasPrefix(lower, "test_") {
		return true
	}
	if (key == "JWT_SECRET" || key == "JWT_SECRET_KEY") && isWeakSecretLength(val) {
		return true
	}
	return false
}

// isWeakSecretLength flags a signing secret too short to resist brute-force —
// same 32-char floor config.go's own comments assume for JWT_SECRET.
func isWeakSecretLength(val string) bool {
	return len(val) < 32
}

// validateJWTSecret checks the JWT signing secret regardless of which of the
// two accepted env var names is set (see config.go's getEnv fallback chain),
// appending to missing/placeholder using whichever name actually supplied
// the value.
func validateJWTSecret(missing, placeholder *[]string) {
	key := "JWT_SECRET_KEY"
	val := os.Getenv(key)
	if val == "" {
		key = "JWT_SECRET"
		val = os.Getenv(key)
	}

	if val == "" {
		*missing = append(*missing, "JWT_SECRET_KEY or JWT_SECRET")
		return
	}
	if isPlaceholderValue(key, val) {
		*placeholder = append(*placeholder, key)
	}
}
