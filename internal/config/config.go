package config

import (
	"fmt"
	"log"
	"os"
	"strings"
	"thanawy-backend/internal/models"

	"github.com/google/uuid"
)

var GlobalConfig *Config

type Config struct {
	DatabaseURL          string
	DatabaseWriteURL     string
	DatabaseReadReplicas []string
	JWTSecret          string
	JWTRefreshSecret   string
	Environment        string
	BCryptCost         int

	// Storage Configuration
	StorageType string // "s3" (Cloudflare R2 / AWS S3 / MinIO) or "local"
	S3          struct {
		Endpoint  string
		AccessKey string
		SecretKey string
		Bucket    string
		Region    string
		UseSSL    bool
		PublicURL string
	}
	LocalStorage struct {
		BaseDir   string
		PublicURL string
	}
	ClerkWebhookSecret string
	ClerkPEMPublicKey  string
	ClerkSecretKey     string
	ClerkJWKSURL       string
	InternalIPRanges   []string
	ImpersonationSecret string

	// HTTP Server Timeouts
	HTTPReadTimeout  string
	HTTPWriteTimeout string
	HTTPIdleTimeout  string

	// Trust Proxy
	TrustProxy     bool
	TrustedProxies []string

	// Rate Limiting
	RateLimitRequests int
	RateLimitWindow   string

	// Cookie Security
	CookieSecure   bool
	CookieSameSite string

	// App Env
	AppEnv string

	// Sentry DSN
	SentryDSN string

	// App Version
	AppVersion string
}

func Load() *Config {
	dbURL := getEnv("DATABASE_URL", "")
	jwtSecret := getEnv("JWT_SECRET", "")
	jwtRefreshSecret := getEnv("JWT_REFRESH_SECRET", "")
	environment := getEnv("NODE_ENV", "development")

	if environment == "production" {
		if jwtSecret == "" || jwtSecret == "default_secret" || jwtSecret == "dev_only_secret_change_in_production" {
			log.Fatal("FATAL: JWT_SECRET MUST be set to a secure, unique value in production environments.")
		}
		if len(jwtSecret) < 32 {
			log.Fatal("FATAL: JWT_SECRET must be at least 32 characters long for production security.")
		}
		if jwtRefreshSecret == "" {
			log.Fatal("FATAL: JWT_REFRESH_SECRET must be set to a secure, unique value in production environments.")
		}
		if len(jwtRefreshSecret) < 32 {
			log.Fatal("FATAL: JWT_REFRESH_SECRET must be at least 32 characters long for production security.")
		}
	} else {
		if jwtSecret == "" {
			log.Println("WARNING: JWT_SECRET is not set. Using insecure default for development only.")
			jwtSecret = "dev_only_secret_change_in_production_" + generateRandomString(16)
		}
		if jwtRefreshSecret == "" {
			log.Println("WARNING: JWT_REFRESH_SECRET is not set. Using fallback to JWT_SECRET for development only.")
			jwtRefreshSecret = jwtSecret + "_refresh"
		}
	}

	c := &Config{
		DatabaseURL:          dbURL,
		DatabaseWriteURL:     getEnv("DATABASE_WRITE_DSN", ""),
		DatabaseReadReplicas: parseReplicas(getEnv("DATABASE_REPLICAS", "")),
		JWTSecret:            jwtSecret,
		JWTRefreshSecret:     jwtRefreshSecret,
		Environment:          environment,
		BCryptCost:           getEnvInt("BCRYPT_COST", 10),
		StorageType:          getEnv("STORAGE_TYPE", "s3"),
	}

	// Local Storage Config
	c.LocalStorage.BaseDir = getEnv("LOCAL_STORAGE_BASE_DIR", "uploads")
	c.LocalStorage.PublicURL = getEnv("LOCAL_STORAGE_PUBLIC_URL", "http://localhost:8082/uploads")

	// S3 Storage Config
	c.S3.Endpoint = getEnv("S3_ENDPOINT", "")
	c.S3.AccessKey = getEnv("S3_ACCESS_KEY", "")
	c.S3.SecretKey = getEnv("S3_SECRET_KEY", "")
	c.S3.Bucket = getEnv("S3_BUCKET", "")
	c.S3.Region = getEnv("S3_REGION", "us-east-1")
	c.S3.UseSSL = getEnv("S3_USE_SSL", "true") == "true"
	c.S3.PublicURL = getEnv("S3_PUBLIC_URL", "")

	// Check if S3 credentials are placeholders or empty, and fallback to local
	isS3Placeholder := c.S3.AccessKey == "" ||
		strings.Contains(strings.ToLower(c.S3.AccessKey), "placeholder") ||
		strings.Contains(strings.ToLower(c.S3.AccessKey), "your-")

	if c.StorageType == "s3" && isS3Placeholder {
		if environment == "production" {
			// CRITICAL: Refuse to start in production without valid cloud storage.
			// LocalStorage on a container/pod is ephemeral — all uploads would be lost
			// on every container restart, redeployment, or autoscaling event.
			log.Fatal("FATAL: S3_ACCESS_KEY is not set or is a placeholder in production. " +
				"LocalStorage is NOT allowed in production (stateless cloud architecture). " +
				"Set S3_ENDPOINT, S3_ACCESS_KEY, S3_SECRET_KEY, S3_BUCKET env vars.")
		}
		log.Println("WARNING: S3 credentials are placeholders or empty. Falling back to local storage (dev only).")
		c.StorageType = "local"
	}

	// Extra safety: explicitly block local storage in production even if STORAGE_TYPE is set directly.
	if c.StorageType == "local" && environment == "production" {
		log.Fatal("FATAL: STORAGE_TYPE=local is not allowed in production. " +
			"All uploads must use cloud storage (S3/Supabase) to ensure data durability.")
	}

	c.ClerkWebhookSecret = getEnv("CLERK_WEBHOOK_SECRET", "")
	c.ClerkPEMPublicKey = getEnv("CLERK_PEM_PUBLIC_KEY", "")
	c.ClerkSecretKey = getEnv("CLERK_SECRET_KEY", "")
	c.ClerkJWKSURL = getEnv("CLERK_JWKS_URL", "")
	c.ImpersonationSecret = getEnv("IMPERSONATION_SECRET", "")

	// Stability & Production configurations
	c.HTTPReadTimeout = getEnv("HTTP_READ_TIMEOUT", "10s")
	c.HTTPWriteTimeout = getEnv("HTTP_WRITE_TIMEOUT", "30s")
	c.HTTPIdleTimeout = getEnv("HTTP_IDLE_TIMEOUT", "120s")

	c.TrustProxy = getEnv("TRUST_PROXY", "true") == "true"
	trustedProxiesRaw := getEnv("TRUSTED_PROXIES", "")
	if trustedProxiesRaw != "" {
		parts := strings.Split(trustedProxiesRaw, ",")
		c.TrustedProxies = make([]string, 0, len(parts))
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				c.TrustedProxies = append(c.TrustedProxies, trimmed)
			}
		}
	}

	c.RateLimitRequests = getEnvInt("RATE_LIMIT_REQUESTS", 200)
	c.RateLimitWindow = getEnv("RATE_LIMIT_WINDOW", "1m")

	c.CookieSecure = getEnv("COOKIE_SECURE", "false") == "true"
	c.CookieSameSite = getEnv("COOKIE_SAME_SITE", "lax")

	c.AppEnv = getEnv("APP_ENV", environment)

	c.SentryDSN = getEnv("SENTRY_DSN", "")
	c.AppVersion = getEnv("APP_VERSION", "1.0.0")

	// IP Whitelist Config
	// Standard RFC 1918 and loopback ranges used as defaults
	defaultRanges := []string{
		"127.0.0.1/8",    // NOSONAR IPv4 Loopback
		"10.0.0.0/8",     // NOSONAR RFC 1918
		"172.16.0.0/12",  // NOSONAR RFC 1918
		"192.168.0.0/16", // NOSONAR RFC 1918
		"::1/128",        // NOSONAR IPv6 Loopback
	}
	models.DefaultInternalIPRanges = defaultRanges

	internalIPsRaw := getEnv("INTERNAL_IP_RANGES", "")
	if internalIPsRaw != "" {
		c.InternalIPRanges = strings.Split(internalIPsRaw, ",")
	} else {
		c.InternalIPRanges = defaultRanges
	}

	return c
}

// LoadSafe returns a Config without calling log.Fatal.
// Instead, it returns the configuration and an error if JWT_SECRET is invalid.
// This is used by the Vercel serverless handler to avoid crashing on cold start.
func LoadSafe() (*Config, error) {
	dbURL := getEnv("DATABASE_URL", "")
	jwtSecret := getEnv("JWT_SECRET", "")
	environment := getEnv("NODE_ENV", "development")

	if environment == "production" {
		if jwtSecret == "" || jwtSecret == "default_secret" || jwtSecret == "dev_only_secret_change_in_production" {
			return nil, fmt.Errorf("JWT_SECRET must be set to a secure, unique value in production environments")
		}
		if len(jwtSecret) < 32 {
			return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters long for production security")
		}
	} else if jwtSecret == "" {
		log.Println("WARNING: JWT_SECRET is not set. Using insecure default for development only.")
		jwtSecret = "dev_only_secret_change_in_production_" + generateRandomString(16)
	}

	c := &Config{
		DatabaseURL:          dbURL,
		DatabaseWriteURL:     getEnv("DATABASE_WRITE_DSN", ""),
		DatabaseReadReplicas: parseReplicas(getEnv("DATABASE_REPLICAS", "")),
		JWTSecret:            jwtSecret,
		Environment:          environment,
		StorageType:          getEnv("STORAGE_TYPE", "s3"),
	}

	// Local Storage Config
	c.LocalStorage.BaseDir = getEnv("LOCAL_STORAGE_BASE_DIR", "uploads")
	c.LocalStorage.PublicURL = getEnv("LOCAL_STORAGE_PUBLIC_URL", "http://localhost:8082/uploads")

	c.S3.Endpoint = getEnv("S3_ENDPOINT", "")
	c.S3.AccessKey = getEnv("S3_ACCESS_KEY", "")
	c.S3.SecretKey = getEnv("S3_SECRET_KEY", "")
	c.S3.Bucket = getEnv("S3_BUCKET", "")
	c.S3.Region = getEnv("S3_REGION", "us-east-1")
	c.S3.UseSSL = getEnv("S3_USE_SSL", "true") == "true"
	c.S3.PublicURL = getEnv("S3_PUBLIC_URL", "")

	// Check if S3 credentials are placeholders or empty, and fallback to local
	isS3Placeholder := c.S3.AccessKey == "" ||
		strings.Contains(strings.ToLower(c.S3.AccessKey), "placeholder") ||
		strings.Contains(strings.ToLower(c.S3.AccessKey), "your-")

	if c.StorageType == "s3" && isS3Placeholder {
		if environment == "production" {
			return nil, fmt.Errorf("FATAL: S3_ACCESS_KEY is not set or is a placeholder in production. " +
				"LocalStorage is NOT allowed in production (stateless cloud architecture). " +
				"Set S3_ENDPOINT, S3_ACCESS_KEY, S3_SECRET_KEY, S3_BUCKET env vars")
		}
		log.Println("WARNING: S3 credentials are placeholders or empty. Falling back to local storage (dev only).")
		c.StorageType = "local"
	}

	// Extra safety: explicitly block local storage in production even if STORAGE_TYPE is set directly.
	if c.StorageType == "local" && environment == "production" {
		return nil, fmt.Errorf("FATAL: STORAGE_TYPE=local is not allowed in production. " +
			"All uploads must use cloud storage (S3/Supabase) to ensure data durability")
	}

	c.ClerkWebhookSecret = getEnv("CLERK_WEBHOOK_SECRET", "")
	c.ClerkPEMPublicKey = getEnv("CLERK_PEM_PUBLIC_KEY", "")
	c.ClerkSecretKey = getEnv("CLERK_SECRET_KEY", "")
	c.ClerkJWKSURL = getEnv("CLERK_JWKS_URL", "")
	c.ImpersonationSecret = getEnv("IMPERSONATION_SECRET", "")

	// Stability & Production configurations
	c.HTTPReadTimeout = getEnv("HTTP_READ_TIMEOUT", "10s")
	c.HTTPWriteTimeout = getEnv("HTTP_WRITE_TIMEOUT", "30s")
	c.HTTPIdleTimeout = getEnv("HTTP_IDLE_TIMEOUT", "120s")

	c.TrustProxy = getEnv("TRUST_PROXY", "true") == "true"
	trustedProxiesSafeRaw := getEnv("TRUSTED_PROXIES", "")
	if trustedProxiesSafeRaw != "" {
		parts := strings.Split(trustedProxiesSafeRaw, ",")
		c.TrustedProxies = make([]string, 0, len(parts))
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				c.TrustedProxies = append(c.TrustedProxies, trimmed)
			}
		}
	}

	c.RateLimitRequests = getEnvInt("RATE_LIMIT_REQUESTS", 200)
	c.RateLimitWindow = getEnv("RATE_LIMIT_WINDOW", "1m")

	c.CookieSecure = getEnv("COOKIE_SECURE", "false") == "true"
	c.CookieSameSite = getEnv("COOKIE_SAME_SITE", "lax")

	c.AppEnv = getEnv("APP_ENV", environment)

	c.SentryDSN = getEnv("SENTRY_DSN", "")
	c.AppVersion = getEnv("APP_VERSION", "1.0.0")

	defaultRanges := []string{
		"127.0.0.1/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"::1/128",
	}
	models.DefaultInternalIPRanges = defaultRanges

	internalIPsRaw := getEnv("INTERNAL_IP_RANGES", "")
	if internalIPsRaw != "" {
		c.InternalIPRanges = strings.Split(internalIPsRaw, ",")
	} else {
		c.InternalIPRanges = defaultRanges
	}

	return c, nil
}

func getEnvInt(key string, defaultVal int) int {
	valStr := getEnv(key, "")
	if valStr == "" {
		return defaultVal
	}
	importStr := strings.TrimSpace(valStr)
	var val int
	_, err := fmt.Sscanf(importStr, "%d", &val)
	if err != nil {
		return defaultVal
	}
	return val
}

// generateRandomString generates a random string for dev secrets
func generateRandomString(n int) string {
	result := uuid.New().String()
	if len(result) > n {
		return result[:n]
	}
	return result
}

func parseReplicas(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}