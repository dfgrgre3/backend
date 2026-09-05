package bootstrap

import (
	"strings"
	"testing"
)

// =============================================================================
// Tests for ValidateProductionConfig
// =============================================================================
//
// These exercise the fail-fast guard added in commit "Add production fail-fast
// for insecure defaults" (see ANALYSIS_REPORT.md §10 / §13 for the rationale).
//
// Each test sets the relevant env vars on the process, then calls the
// exported IsProductionEnv / ValidateProductionConfig helpers. We restore
// the original env at the end of every test with t.Setenv (Go 1.17+) which
// also takes care of restoring values on test cleanup.
// =============================================================================

func withProductionEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "production")
}

func withDevelopmentEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "development")
}

func setGoodSecrets(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "abcdefghijklmnopqrstuvwxyz1234567890") // 34 chars
	t.Setenv("POSTGRES_PASSWORD", "supersecret-pg-pw-2024")
	t.Setenv("REDIS_PASSWORD", "supersecret-redis-pw-2024")
	t.Setenv("MINIO_ROOT_PASSWORD", "supersecret-minio-pw-2024")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8,192.168.0.0/16")
}

// TestValidate_DevelopmentIsAlwaysAllowed confirms the guard is a no-op
// outside production. The dev workflow must keep working with weak defaults.
func TestValidate_DevelopmentIsAlwaysAllowed(t *testing.T) {
	withDevelopmentEnv(t)
	t.Setenv("JWT_SECRET", "x")
	t.Setenv("COOKIE_SECURE", "false")
	// no TRUSTED_PROXIES

	if err := ValidateProductionConfig(); err != nil {
		t.Fatalf("expected dev env to bypass validation, got: %v", err)
	}
}

// TestValidate_AcceptsStrongProductionDefaults is the happy path: every
// required env is set with a non-placeholder, sufficiently long value.
func TestValidate_AcceptsStrongProductionDefaults(t *testing.T) {
	withProductionEnv(t)
	setGoodSecrets(t)

	if err := ValidateProductionConfig(); err != nil {
		t.Fatalf("expected strong production env to be accepted, got: %v", err)
	}
}

// TestValidate_RejectsDefaultJWTSecret pins the most common dev-to-prod
// leak: someone forgot to override JWT_SECRET and the docker-compose
// placeholder is still in the environment.
func TestValidate_RejectsDefaultJWTSecret(t *testing.T) {
	withProductionEnv(t)
	setGoodSecrets(t)
	t.Setenv("JWT_SECRET", "change_me_in_production")

	err := ValidateProductionConfig()
	if err == nil {
		t.Fatal("expected validation to fail for the JWT placeholder")
	}
	if !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Errorf("expected error to mention JWT_SECRET, got: %v", err)
	}
}

// TestValidate_RejectsShortJWTSecret ensures the 32-byte minimum is
// enforced even when the value is "real" (just too short).
func TestValidate_RejectsShortJWTSecret(t *testing.T) {
	withProductionEnv(t)
	setGoodSecrets(t)
	t.Setenv("JWT_SECRET", "tooshort")

	err := ValidateProductionConfig()
	if err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("expected JWT_SECRET length error, got: %v", err)
	}
}

// TestValidate_RejectsDefaultPostgresPassword catches the same leak class
// for the database password.
func TestValidate_RejectsDefaultPostgresPassword(t *testing.T) {
	withProductionEnv(t)
	setGoodSecrets(t)
	t.Setenv("POSTGRES_PASSWORD", "change_me_strong_password")

	err := ValidateProductionConfig()
	if err == nil || !strings.Contains(err.Error(), "POSTGRES_PASSWORD") {
		t.Fatalf("expected POSTGRES_PASSWORD placeholder error, got: %v", err)
	}
}

// TestValidate_RejectsInsecureCookie covers the COOKIE_SECURE=false path:
// session cookies would otherwise travel over plain HTTP.
func TestValidate_RejectsInsecureCookie(t *testing.T) {
	withProductionEnv(t)
	setGoodSecrets(t)
	t.Setenv("COOKIE_SECURE", "false")

	err := ValidateProductionConfig()
	if err == nil || !strings.Contains(err.Error(), "COOKIE_SECURE") {
		t.Fatalf("expected COOKIE_SECURE error, got: %v", err)
	}
}

// TestValidate_RejectsWildcardTrustedProxy ensures the single most
// dangerous mis-configuration (TRUSTED_PROXIES=0.0.0.0/0) is caught.
func TestValidate_RejectsWildcardTrustedProxy(t *testing.T) {
	withProductionEnv(t)
	setGoodSecrets(t)
	t.Setenv("TRUSTED_PROXIES", "0.0.0.0/0")

	err := ValidateProductionConfig()
	if err == nil || !strings.Contains(err.Error(), "TRUSTED_PROXIES") {
		t.Fatalf("expected TRUSTED_PROXIES error, got: %v", err)
	}
}

// TestValidate_RejectsAsteriskTrustedProxy covers the "*" variant that
// Gin's SetTrustedProxies also treats as "trust everyone".
func TestValidate_RejectsAsteriskTrustedProxy(t *testing.T) {
	withProductionEnv(t)
	setGoodSecrets(t)
	t.Setenv("TRUSTED_PROXIES", "*")

	err := ValidateProductionConfig()
	if err == nil || !strings.Contains(err.Error(), "TRUSTED_PROXIES") {
		t.Fatalf("expected TRUSTED_PROXIES error, got: %v", err)
	}
}

// TestValidate_RejectsEmptyTrustedProxy covers the silent-fail case: the
// env var is unset, which would otherwise let Gin fall back to trusting
// the immediate peer.
func TestValidate_RejectsEmptyTrustedProxy(t *testing.T) {
	withProductionEnv(t)
	setGoodSecrets(t)
	t.Setenv("TRUSTED_PROXIES", "")

	err := ValidateProductionConfig()
	if err == nil || !strings.Contains(err.Error(), "TRUSTED_PROXIES") {
		t.Fatalf("expected TRUSTED_PROXIES empty error, got: %v", err)
	}
}

// TestValidate_AllowsValidCIDRs is the negative of the previous three
// tests: well-formed RFC1918 ranges should be accepted.
func TestValidate_AllowsValidCIDRs(t *testing.T) {
	withProductionEnv(t)
	setGoodSecrets(t)
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,::1/128")

	if err := ValidateProductionConfig(); err != nil {
		t.Fatalf("expected valid CIDR list to be accepted, got: %v", err)
	}
}

// TestValidate_RejectsRedisPasswordWhenRedisEnabled covers the case
// where Redis is the active session/lockout store (the default) but
// someone left the dev placeholder.
func TestValidate_RejectsRedisPasswordWhenRedisEnabled(t *testing.T) {
	withProductionEnv(t)
	setGoodSecrets(t)
	t.Setenv("DISABLE_REDIS", "false")
	t.Setenv("REDIS_PASSWORD", "devpassword")

	err := ValidateProductionConfig()
	if err == nil || !strings.Contains(err.Error(), "REDIS_PASSWORD") {
		t.Fatalf("expected REDIS_PASSWORD error, got: %v", err)
	}
}

// TestValidate_SkipsRedisPasswordWhenRedisDisabled confirms that
// intentionally turning Redis off (DISABLE_REDIS=true) does not also
// require a Redis password to be set.
func TestValidate_SkipsRedisPasswordWhenRedisDisabled(t *testing.T) {
	withProductionEnv(t)
	setGoodSecrets(t)
	t.Setenv("DISABLE_REDIS", "true")
	// no REDIS_PASSWORD at all
	t.Setenv("REDIS_PASSWORD", "")

	if err := ValidateProductionConfig(); err != nil {
		t.Fatalf("expected Redis-disabled env to be accepted, got: %v", err)
	}
}

// TestValidate_RejectsMinIOPassword covers the storage layer.
func TestValidate_RejectsMinIOPassword(t *testing.T) {
	withProductionEnv(t)
	setGoodSecrets(t)
	t.Setenv("MINIO_ROOT_PASSWORD", "minioadmin")

	err := ValidateProductionConfig()
	if err == nil || !strings.Contains(err.Error(), "MINIO_ROOT_PASSWORD") {
		t.Fatalf("expected MINIO_ROOT_PASSWORD error, got: %v", err)
	}
}

// TestIsProductionEnv double-checks the env-name matcher covers the
// common aliases the project uses.
func TestIsProductionEnv(t *testing.T) {
	cases := map[string]bool{
		"production": true,
		"prod":       true,
		"PRODUCTION": true,
		"Prod":       true,
		"dev":        false,
		"develop":    false,
		"":           false,
	}
	for env, want := range cases {
		t.Setenv("APP_ENV", env)
		if got := IsProductionEnv(); got != want {
			t.Errorf("APP_ENV=%q: IsProductionEnv() = %v, want %v", env, got, want)
		}
	}
}
