package services

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"thanawy-backend/internal/config"
	"thanawy-backend/internal/db"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
)

type TokenService struct{}

type TokenClaims struct {
	Email string `json:"email"`
	Role  string `json:"role"`
	JTI   string `json:"jti"` // Added for session tracking
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	JTI          string `json:"jti"`
}

type jwkKey struct {
	Kty string   `json:"kty"`
	Kid string   `json:"kid"`
	Use string   `json:"use"`
	Alg string   `json:"alg"`
	N   string   `json:"n"`
	E   string   `json:"e"`
	X5c []string `json:"x5c"`
}

type jwksKeys struct {
	Keys []jwkKey `json:"keys"`
}

var (
	jwkCache      = make(map[string]*rsa.PublicKey)
	jwkCacheMu    sync.RWMutex
	lastFetchTime time.Time
	fetchMu       sync.Mutex

	clerkPEMKeyVal string
	clerkPEMPubKey *rsa.PublicKey
	clerkPEMMu     sync.RWMutex

	jtiBlacklistCache = cache.New(15*time.Minute, 30*time.Minute)
)

func getClerkPEMPublicKey(pemPublicKey string) (*rsa.PublicKey, error) {
	clerkPEMMu.RLock()
	if clerkPEMPubKey != nil && clerkPEMKeyVal == pemPublicKey {
		pubKey := clerkPEMPubKey
		clerkPEMMu.RUnlock()
		return pubKey, nil
	}
	clerkPEMMu.RUnlock()

	clerkPEMMu.Lock()
	defer clerkPEMMu.Unlock()

	// Recheck under write lock
	if clerkPEMPubKey != nil && clerkPEMKeyVal == pemPublicKey {
		return clerkPEMPubKey, nil
	}

	pemStr := strings.ReplaceAll(pemPublicKey, "\\n", "\n")
	pubKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(pemStr))
	if err != nil {
		return nil, err
	}

	clerkPEMPubKey = pubKey
	clerkPEMKeyVal = pemPublicKey
	return pubKey, nil
}


func parseJWKToRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, err
	}

	var eVal int
	for _, b := range eBytes {
		eVal = (eVal << 8) | int(b)
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: eVal,
	}, nil
}

func fetchAndCacheJWKS(jwksURL string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(jwksURL)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS from %s: %w", jwksURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch JWKS from %s, status: %d", jwksURL, resp.StatusCode)
	}

	var jwks jwksKeys
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("failed to decode JWKS: %w", err)
	}

	jwkCacheMu.Lock()
	defer jwkCacheMu.Unlock()

	for _, key := range jwks.Keys {
		if key.Kty == "RSA" && key.N != "" && key.E != "" {
			pubKey, err := parseJWKToRSAPublicKey(key.N, key.E)
			if err == nil {
				jwkCache[key.Kid] = pubKey
			}
		}
	}

	return nil
}

func (s *TokenService) GenerateTokenPair(userId, role, email string) (*TokenPair, error) {
	cfg := config.Load()
	jti := uuid.New().String()

	// Access Token (Short-lived: 15 minutes).
	// Email is included so middleware can hydrate user_email directly from the
	// token without a DB lookup on every request.
	accessClaims := TokenClaims{
		Email: email,
		Role:  role,
		JTI:   jti,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userId,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        jti,
		},
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		return nil, err
	}

	refreshSecret := cfg.JWTRefreshSecret
	if refreshSecret == "" {
		refreshSecret = cfg.JWTSecret
	}

	// Refresh Token (Long-lived: 30 days).
	// The refresh token only carries subject + expiry; email/role are re-fetched
	// from the DB on every token rotation to reflect any permission changes.
	// Signed with JWT_REFRESH_SECRET to isolate access-token compromise.
	refreshClaims := jwt.RegisteredClaims{
		Subject:   userId,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ID:        jti,
	}
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(refreshSecret))
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		JTI:          jti,
	}, nil
}

// GenerateAccessToken is intentionally removed.
// Always use GenerateTokenPair so the JTI is tied to a persisted UserSession.

func (s *TokenService) ValidateToken(tokenString string) (*TokenClaims, error) {
	cfg := config.Load()
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		alg, ok := token.Header["alg"].(string)
		if !ok {
			return nil, fmt.Errorf("missing signing algorithm")
		}

		// If alg is RS256, verify signature using Clerk JWKS or PEM Public Key
		if alg == "RS256" {
			kid, _ := token.Header["kid"].(string)

			// Try cache first if kid is available
			if kid != "" {
				jwkCacheMu.RLock()
				pubKey, exists := jwkCache[kid]
				jwkCacheMu.RUnlock()
				if exists {
					return pubKey, nil
				}
			}

			// If kid not in cache and JWKS URL is configured, try fetching JWKS
			if cfg.ClerkJWKSURL != "" {
				fetchMu.Lock()
				jwkCacheMu.RLock()
				pubKey, exists := jwkCache[kid]
				jwkCacheMu.RUnlock()
				if exists {
					fetchMu.Unlock()
					return pubKey, nil
				}

				// Throttle fetches: don't fetch more than once every 10 seconds
				if time.Since(lastFetchTime) > 10*time.Second {
					if err := fetchAndCacheJWKS(cfg.ClerkJWKSURL); err == nil {
						lastFetchTime = time.Now()
					}
				}
				fetchMu.Unlock()

				// Check cache again after fetch
				if kid != "" {
					jwkCacheMu.RLock()
					pubKey, exists = jwkCache[kid]
					jwkCacheMu.RUnlock()
					if exists {
						return pubKey, nil
					}
				}
			}

			// Fallback to CLERK_PEM_PUBLIC_KEY
			if cfg.ClerkPEMPublicKey == "" {
				return nil, fmt.Errorf("clerk public key is not configured and JWKS validation failed")
			}
			publicKey, err := getClerkPEMPublicKey(cfg.ClerkPEMPublicKey)
			if err != nil {
				return nil, fmt.Errorf("failed to parse Clerk public key: %w", err)
			}
			return publicKey, nil
		}

		if alg == "HS256" {
			if cfg.Environment == "production" {
				return nil, fmt.Errorf("HS256 signing method is disabled in production")
			}
			return []byte(cfg.JWTSecret), nil
		}

		return nil, fmt.Errorf("unexpected signing method: %v. Only Clerk RS256 or HS256 tokens are supported", alg)
	// WithLeeway allows up to 30 seconds of clock skew between servers.
	// Intentionally kept small to minimise the window for replaying near-expired tokens.
	}, jwt.WithLeeway(30*time.Second))

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		// Validate Issuer (iss) and Audience (aud) claims for RS256 Clerk tokens
		if token.Method.Alg() == "RS256" {
			if cfg.ClerkIssuerURL != "" && !strings.HasPrefix(claims.Issuer, cfg.ClerkIssuerURL) {
				return nil, fmt.Errorf("invalid token issuer: expected prefix %s, got %s", cfg.ClerkIssuerURL, claims.Issuer)
			}
			if cfg.ClerkClientID != "" {
				matched := false
				for _, aud := range claims.Audience {
					if aud == cfg.ClerkClientID {
						matched = true
						break
					}
				}
				if !matched {
					return nil, fmt.Errorf("invalid token audience: expected %s", cfg.ClerkClientID)
				}
			}
		}
		return claims, nil
	}

	return nil, jwt.ErrSignatureInvalid
}

func (s *TokenService) ValidateRefreshToken(tokenString string) (*jwt.RegisteredClaims, error) {
	cfg := config.Load()
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		alg, ok := token.Header["alg"].(string)
		if !ok {
			return nil, fmt.Errorf("missing signing algorithm")
		}
		if alg != "HS256" {
			return nil, fmt.Errorf("unexpected signing method for refresh token: %v", alg)
		}
		refreshSecret := cfg.JWTRefreshSecret
		if refreshSecret == "" {
			refreshSecret = cfg.JWTSecret
		}
		return []byte(refreshSecret), nil
	}, jwt.WithLeeway(30*time.Second))

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrSignatureInvalid
}

func (s *TokenService) BlacklistJTI(jti string, duration time.Duration) {
	if jti == "" {
		return
	}
	if duration <= 0 {
		duration = 15 * time.Minute
	}
	jtiBlacklistCache.Set(jti, true, duration)

	if db.Redis != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			blacklistKey := fmt.Sprintf("jti_blacklist:%s", jti)
			db.Redis.Set(ctx, blacklistKey, "1", duration)
		}()
	}
}

func (s *TokenService) IsJTIBlacklisted(jti string) bool {
	if jti == "" {
		return false
	}
	if _, found := jtiBlacklistCache.Get(jti); found {
		return true
	}
	if db.Redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		isBlacklisted, err := db.Redis.Exists(ctx, fmt.Sprintf("jti_blacklist:%s", jti)).Result()
		cancel()
		if err == nil && isBlacklisted > 0 {
			jtiBlacklistCache.Set(jti, true, 15*time.Minute)
			return true
		}
	}
	return false
}
