package config

import (
	"fmt"
	"log"
	"os"
	"strings"
	models "thanawy-backend/internal/domain/common"
)

type Config struct {
	DatabaseURL          string
	DatabaseWriteURL     string
	DatabaseReadReplicas []string
	Environment          string
	BCryptCost           int

	// JWT Token validation configuration
	JWTSecretKey string
	JWTIssuerURL string
	JWTJWKSURL   string
	JWTClientID  string

	// Storage Configuration
	StorageType string // "s3" (Cloudflare R2 / AWS S3 / MinIO)
	S3          struct {
		Endpoint  string
		AccessKey string
		SecretKey string
		Bucket    string
		Region    string
		UseSSL    bool
		PublicURL string
	}
	InternalIPRanges    []string
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
	CookieDomain   string

	// App Env
	AppEnv string

	// Sentry DSN
	SentryDSN string

	// App Version
	AppVersion string
}

var GlobalConfig *Config

func Load() *Config {
	dbURL := getEnv("DATABASE_URL", "")
	environment := getEnv("NODE_ENV", "development")

	c := &Config{
		DatabaseURL:          dbURL,
		DatabaseWriteURL:     getEnv("DATABASE_WRITE_DSN", ""),
		DatabaseReadReplicas: parseReplicas(getEnv("DATABASE_REPLICAS", "")),
		Environment:          environment,
		BCryptCost:           getEnvInt("BCRYPT_COST", 12),
		StorageType:          getEnv("STORAGE_TYPE", "s3"),
		JWTSecretKey:         getEnv("JWT_SECRET_KEY", getEnv("JWT_SECRET", "")),
		JWTIssuerURL:         getEnv("JWT_ISSUER_URL", ""),
		JWTJWKSURL:           getEnv("JWT_JWKS_URL", ""),
		JWTClientID:          getEnv("JWT_CLIENT_ID", ""),
	}

	// S3 Storage Config
	c.S3.Endpoint = getEnv("S3_ENDPOINT", "")
	c.S3.AccessKey = getEnv("S3_ACCESS_KEY", "")
	c.S3.SecretKey = getEnv("S3_SECRET_KEY", "")
	c.S3.Bucket = getEnv("S3_BUCKET", "")
	c.S3.Region = getEnv("S3_REGION", "us-east-1")
	c.S3.UseSSL = getEnv("S3_USE_SSL", "true") == "true"
	c.S3.PublicURL = getEnv("S3_PUBLIC_URL", "")

	// Check if S3 credentials are placeholders or empty.
	isS3Placeholder := c.S3.AccessKey == "" ||
		strings.Contains(strings.ToLower(c.S3.AccessKey), "placeholder") ||
		strings.Contains(strings.ToLower(c.S3.AccessKey), "your-")

	if c.StorageType == "s3" && isS3Placeholder {
		log.Fatal("FATAL: Cloud storage is required. Set S3_ENDPOINT, S3_ACCESS_KEY, S3_SECRET_KEY, S3_BUCKET env vars.")
	}

	if c.StorageType != "s3" && c.StorageType != "local" {
		log.Fatalf("FATAL: Unsupported storage type %q. Use 's3' or 'local'.", c.StorageType)
	}

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

	c.RateLimitRequests = getEnvInt("RATE_LIMIT_REQUESTS", 500)
	c.RateLimitWindow = getEnv("RATE_LIMIT_WINDOW", "1m")

	// Cookie Security: default to secure in production to prevent
	// session/CSRF cookies from being transmitted over plain HTTP.
	// Explicit COOKIE_SECURE=false can be set for local HTTP development.
	cookieSecureDefault := "false"
	if environment == "production" {
		cookieSecureDefault = "true"
	}
	c.CookieSecure = getEnv("COOKIE_SECURE", cookieSecureDefault) == "true"
	c.CookieSameSite = getEnv("COOKIE_SAME_SITE", "lax")
	c.CookieDomain = getEnv("COOKIE_DOMAIN", "")

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

	GlobalConfig = c
	return c
}

// LoadSafe returns a Config without calling log.Fatal.
func LoadSafe() (*Config, error) {
	dbURL := getEnv("DATABASE_URL", "")
	environment := getEnv("NODE_ENV", "development")

	c := &Config{
		DatabaseURL:          dbURL,
		DatabaseWriteURL:     getEnv("DATABASE_WRITE_DSN", ""),
		DatabaseReadReplicas: parseReplicas(getEnv("DATABASE_REPLICAS", "")),
		Environment:          environment,
		BCryptCost:           getEnvInt("BCRYPT_COST", 12),
		StorageType:          getEnv("STORAGE_TYPE", "s3"),
		JWTSecretKey:         getEnv("JWT_SECRET_KEY", getEnv("JWT_SECRET", "")),
		JWTIssuerURL:         getEnv("JWT_ISSUER_URL", ""),
		JWTJWKSURL:           getEnv("JWT_JWKS_URL", ""),
		JWTClientID:          getEnv("JWT_CLIENT_ID", ""),
	}

	c.S3.Endpoint = getEnv("S3_ENDPOINT", "")
	c.S3.AccessKey = getEnv("S3_ACCESS_KEY", "")
	c.S3.SecretKey = getEnv("S3_SECRET_KEY", "")
	c.S3.Bucket = getEnv("S3_BUCKET", "")
	c.S3.Region = getEnv("S3_REGION", "us-east-1")
	c.S3.UseSSL = getEnv("S3_USE_SSL", "true") == "true"
	c.S3.PublicURL = getEnv("S3_PUBLIC_URL", "")

	// Check if S3 credentials are placeholders or empty.
	isS3Placeholder := c.S3.AccessKey == "" ||
		strings.Contains(strings.ToLower(c.S3.AccessKey), "placeholder") ||
		strings.Contains(strings.ToLower(c.S3.AccessKey), "your-")

	if c.StorageType == "s3" && isS3Placeholder {
		return nil, fmt.Errorf("cloud storage is required: set S3_ENDPOINT, S3_ACCESS_KEY, S3_SECRET_KEY, S3_BUCKET env vars")
	}

	if c.StorageType != "s3" && c.StorageType != "local" {
		return nil, fmt.Errorf("unsupported storage type %q: use 's3' or 'local'", c.StorageType)
	}

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

	c.RateLimitRequests = getEnvInt("RATE_LIMIT_REQUESTS", 500)
	c.RateLimitWindow = getEnv("RATE_LIMIT_WINDOW", "1m")

	// Cookie Security: default to secure in production to prevent
	// session/CSRF cookies from being transmitted over plain HTTP.
	// Explicit COOKIE_SECURE=false can be set for local HTTP development.
	cookieSecureDefault := "false"
	if environment == "production" {
		cookieSecureDefault = "true"
	}
	c.CookieSecure = getEnv("COOKIE_SECURE", cookieSecureDefault) == "true"
	c.CookieSameSite = getEnv("COOKIE_SAME_SITE", "lax")
	c.CookieDomain = getEnv("COOKIE_DOMAIN", "")

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
