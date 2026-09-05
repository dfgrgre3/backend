package bootstrap

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

// =============================================================================
// Environment variable helpers
// =============================================================================
//
// getEnvBool / getEnvInt — local helpers used by other bootstrap files.
// We deliberately do NOT import internal/infrastructure/config here to keep
// this package free of any infra-layer dependency (it lives in the very
// same composition root and runs before the rest of the app is wired up).
//
// The `ValidateProductionConfig` function at the bottom of this file is the
// new fail-fast guard for insecure production defaults — see the SECURITY
// block in ANALYSIS_REPORT.md §10 / §12.7.
// =============================================================================

func getEnvBool(key string, defaultVal bool) bool {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseBool(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

func getEnvInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil || val <= 0 {
		return defaultVal
	}
	return val
}

// =============================================================================
// Production fail-fast validation
// =============================================================================
//
// `ValidateProductionConfig` is called once at startup, immediately after
// `config.Load()` returns. In non-production environments it is a no-op
// (returns nil); in `APP_ENV=production` (or any *_production override) it
// aborts the process with `log.Fatal` on every insecure default that the
// analysis report flagged as CRITICAL or HIGH:
//
//   1. JWT secret is the docker-compose placeholder ("change_me_in_production")
//      OR is shorter than 32 bytes
//   2. POSTGRES_PASSWORD is the docker-compose placeholder
//      OR is shorter than 16 characters
//   3. COOKIE_SECURE is not explicitly "true"
//   4. Trusted proxies is unset, contains 0.0.0.0/0 or ::/0, OR contains
//      the * wildcard — any of which would let an attacker spoof
//      X-Forwarded-For and bypass the rate limiter / IP-based audit
//   5. REDIS_PASSWORD (when DISABLE_REDIS != "true") is the dev default
//      ("devpassword") OR is shorter than 16 characters
//   6. MINIO_ROOT_PASSWORD is the docker-compose placeholder OR is shorter
//      than 16 characters
//
// We do NOT reimplement config parsing here — we only *re-read* the raw env
// values. The intent is to catch the dangerous fallbacks baked into
// docker-compose.yml before they reach production traffic.
//
// Why not refactor config.Load() to do this?
//   - config.Load() lives in internal/infrastructure/config; this file
//     lives in internal/bootstrap. Keeping the fail-fast here makes the
//     "what we refuse to start with" rule visible at the composition root.
//   - A future test suite can exercise ValidateProductionConfig directly
//     without spinning up the full DI container.

var (
	// insecurePlaceholderTokens are the placeholder strings we refuse to see
	// in production. They are checked case-insensitively as substrings so
	// variants like "CHANGE_ME_IN_PRODUCTION!" still match.
	insecurePlaceholderTokens = []string{
		"change_me",
		"changeme",
		"devpassword",
		"minioadmin",
		"strong_password_placeholder",
	}

	// jwtMinLength mirrors the existing min-length check in config.Load()
	// (see internal/infrastructure/config/config.go:148). Keep in sync.
	jwtMinLength = 32

	// passwordMinLength is the minimum length we accept for any
	// infrastructure password in production.
	passwordMinLength = 16
)

// isInsecureSecret reports whether `value` matches a known placeholder or
// is too short to be safe.
func isInsecureSecret(value, name string, minLen int) error {
	if value == "" {
		return fmt.Errorf("%s is not set", name)
	}
	if len(value) < minLen {
		return fmt.Errorf("%s is too short (got %d chars, minimum %d)", name, len(value), minLen)
	}
	lower := strings.ToLower(value)
	for _, tok := range insecurePlaceholderTokens {
		if strings.Contains(lower, tok) {
			return fmt.Errorf("%s still uses the placeholder value %q — set a real secret in production", name, tok)
		}
	}
	return nil
}

// hasInsecureTrustedProxy reports whether the TRUSTED_PROXIES env contains
// any wildcard that would let an attacker spoof the source IP.
func hasInsecureTrustedProxy(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("TRUSTED_PROXIES is empty in production — X-Forwarded-For would be ignored and the rate limiter / IP-based audit can be bypassed by setting the header from any client")
	}
	for _, part := range strings.Split(raw, ",") {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}
		if entry == "*" {
			return fmt.Errorf("TRUSTED_PROXIES contains %q — every client would be treated as a trusted proxy", entry)
		}
		if entry == "0.0.0.0/0" || entry == "::/0" {
			return fmt.Errorf("TRUSTED_PROXIES contains %q — every client (including the public internet) would be treated as a trusted proxy", entry)
		}
		// Also catch single-host wildcards like "0.0.0.0" or "::"
		if entry == "0.0.0.0" || entry == "::" {
			return fmt.Errorf("TRUSTED_PROXIES contains %q — this effectively trusts every client", entry)
		}
		// And the worst case: a malformed CIDR that Gin would silently
		// fall back to trust-everything for.
		if _, _, err := net.ParseCIDR(entry); err != nil && net.ParseIP(entry) == nil {
			return fmt.Errorf("TRUSTED_PROXIES entry %q is not a valid IP or CIDR", entry)
		}
	}
	return nil
}

// IsProductionEnv reports whether the current APP_ENV should trigger the
// fail-fast checks. Exported so tests can drive it directly.
func IsProductionEnv() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	return env == "production" || env == "prod"
}

// ValidateProductionConfig enforces secure defaults in production. Returns
// nil for any non-production env, otherwise returns a joined error covering
// every insecure default that was detected. `run.go` is expected to
// `log.Fatal` on a non-nil result.
func ValidateProductionConfig() error {
	if !IsProductionEnv() {
		return nil
	}

	var errs []string

	// 1. JWT secret
	if err := isInsecureSecret(os.Getenv("JWT_SECRET"), "JWT_SECRET", jwtMinLength); err != nil {
		errs = append(errs, err.Error())
	}
	// JWT_SECRET_KEY is the alternate name used elsewhere in the codebase;
	// require the same minimum if JWT_SECRET is empty.
	if os.Getenv("JWT_SECRET") == "" {
		if err := isInsecureSecret(os.Getenv("JWT_SECRET_KEY"), "JWT_SECRET_KEY", jwtMinLength); err != nil {
			errs = append(errs, err.Error())
		}
	}

	// 2. Postgres password
	if err := isInsecureSecret(os.Getenv("POSTGRES_PASSWORD"), "POSTGRES_PASSWORD", passwordMinLength); err != nil {
		errs = append(errs, err.Error())
	}

	// 3. Cookies must be marked Secure in production (else the session
	// cookies travel over plain HTTP if a reverse proxy mis-routes a
	// request). config.Load() already defaults this to true for prod, but
	// we re-validate so a manual override of COOKIE_SECURE=false is loud.
	if !getEnvBool("COOKIE_SECURE", false) {
		errs = append(errs, "COOKIE_SECURE must be true in production — auth cookies must not travel over plain HTTP")
	}

	// 4. Trusted proxies
	if err := hasInsecureTrustedProxy(os.Getenv("TRUSTED_PROXIES")); err != nil {
		errs = append(errs, err.Error())
	}

	// 5. Redis password (only when Redis is the active session/lockout
	// store — which the codebase documents as a hard security dependency).
	if !getEnvBool("DISABLE_REDIS", false) {
		if err := isInsecureSecret(os.Getenv("REDIS_PASSWORD"), "REDIS_PASSWORD", passwordMinLength); err != nil {
			errs = append(errs, err.Error())
		}
	}

	// 6. MinIO root password
	if err := isInsecureSecret(os.Getenv("MINIO_ROOT_PASSWORD"), "MINIO_ROOT_PASSWORD", passwordMinLength); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("insecure production configuration detected:\n  - %s", strings.Join(errs, "\n  - "))
}

// MustValidateProductionConfig is a thin wrapper that calls
// ValidateProductionConfig and logs+fatal-exits on any failure. Safe to
// call from `Run()` after `config.Load()` returns.
func MustValidateProductionConfig() {
	if err := ValidateProductionConfig(); err != nil {
		log.Fatalf("[STARTUP] %v", err)
	}
}
