package authservice

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"
	"time"

	"thanawy-backend/internal/infrastructure/config"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type CustomClaims struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
	JTI          string
}

type AuthTokenService interface {
	GenerateTokenPair(user *models.User) (*TokenPair, error)
	ValidateAccessToken(tokenString string) (*CustomClaims, error)
	GenerateRefreshToken() (string, error)
	IsJTIBlacklisted(jti string) bool
	BlacklistJTI(jti string, ttl time.Duration) error
	GenerateAccessTokenForSession(user *models.User, sessionID string) (string, error)
}

type authTokenService struct {
	cfg *config.Config
}

// Package-level JTI blacklist shared across all AuthTokenService instances.
// This ensures that a token blacklisted by one handler is recognized by the
// auth middleware and any other consumer, regardless of which
// NewAuthTokenService() call they obtained.
var globalJTIBlacklist sync.Map

func NewAuthTokenService() AuthTokenService {
	return &authTokenService{cfg: config.GlobalConfig}
}

func (s *authTokenService) IsJTIBlacklisted(jti string) bool {
	if jti == "" {
		return false
	}
	if v, ok := globalJTIBlacklist.Load(jti); ok {
		if expiresAt, ok := v.(time.Time); ok {
			if time.Now().Before(expiresAt) {
				return true
			}
			globalJTIBlacklist.Delete(jti)
		}
	}
	if cache.Redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		exists, err := cache.Redis.Exists(ctx, fmt.Sprintf("blacklist_jti:%s", jti)).Result()
		if err == nil && exists > 0 {
			return true
		}
	}
	return false
}

func (s *authTokenService) BlacklistJTI(jti string, ttl time.Duration) error {
	if jti == "" {
		return nil
	}
	globalJTIBlacklist.Store(jti, time.Now().Add(ttl))
	if cache.Redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		return cache.Redis.Set(ctx, fmt.Sprintf("blacklist_jti:%s", jti), "true", ttl).Err()
	}
	return nil
}

func (s *authTokenService) GenerateTokenPair(user *models.User) (*TokenPair, error) {
	if s.cfg == nil {
		s.cfg = config.Load()
	}
	if user == nil {
		return nil, errors.New("user is required")
	}

	now := time.Now()
	expiresAt := now.Add(15 * time.Minute)
	jti := uuid.NewString()
	claims := CustomClaims{
		UserID: user.ID,
		Role:   string(user.Role),
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   user.ID,
			Issuer:    s.cfg.JWTIssuerURL,
			Audience:  jwt.ClaimStrings{"thanawy-api"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	accessToken, err := s.signAccessToken(claims)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(time.Until(expiresAt).Seconds()),
		JTI:          jti,
	}, nil
}

func (s *authTokenService) GenerateAccessTokenForSession(user *models.User, sessionID string) (string, error) {
	if s.cfg == nil {
		s.cfg = config.Load()
	}
	if user == nil {
		return "", errors.New("user is required")
	}

	now := time.Now()
	expiresAt := now.Add(15 * time.Minute)
	claims := CustomClaims{
		UserID: user.ID,
		Role:   string(user.Role),
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        sessionID,
			Subject:   user.ID,
			Issuer:    s.cfg.JWTIssuerURL,
			Audience:  jwt.ClaimStrings{"thanawy-api"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	return s.signAccessToken(claims)
}

func (s *authTokenService) signAccessToken(claims CustomClaims) (string, error) {
	if strings.TrimSpace(s.cfg.JWTPrivateKey) != "" {
		privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(strings.ReplaceAll(s.cfg.JWTPrivateKey, `\n`, "\n")))
		if err != nil {
			return "", fmt.Errorf("invalid JWT_PRIVATE_KEY: %w", err)
		}
		return jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(privateKey)
	}

	if s.cfg.Environment == "production" {
		return "", errors.New("JWT_PRIVATE_KEY is required in production")
	}
	if s.cfg.JWTSecretKey == "" {
		return "", errors.New("JWT secret key is not configured")
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JWTSecretKey))
}

func (s *authTokenService) ValidateAccessToken(tokenString string) (*CustomClaims, error) {
	if s.cfg == nil {
		s.cfg = config.Load()
	}
	var claims *CustomClaims
	var token *jwt.Token
	var err error
	if strings.TrimSpace(s.cfg.JWTPublicKey) != "" {
		publicKey, parseErr := jwt.ParseRSAPublicKeyFromPEM([]byte(strings.ReplaceAll(s.cfg.JWTPublicKey, `\n`, "\n")))
		if parseErr != nil {
			return nil, fmt.Errorf("invalid JWT_PUBLIC_KEY: %w", parseErr)
		}
		claims = &CustomClaims{}
		token, err = jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if token.Method == nil || token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return publicKey, nil
		}, jwt.WithIssuer(s.cfg.JWTIssuerURL), jwt.WithAudience("thanawy-api"), jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
	} else if s.cfg.Environment != "production" && s.cfg.JWTSecretKey != "" {
		claims = &CustomClaims{}
		token, err = jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if token.Method == nil || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(s.cfg.JWTSecretKey), nil
		}, jwt.WithAudience("thanawy-api"), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	} else {
		return nil, errors.New("JWT_PUBLIC_KEY is required for access-token validation")
	}
	if err != nil || token == nil || !token.Valid {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("invalid token")
	}
	if claims.ExpiresAt == nil {
		return nil, errors.New("token missing exp claim")
	}
	if claims.UserID == "" {
		claims.UserID = claims.Subject
	}

	if claims != nil && s.IsJTIBlacklisted(claims.ID) {
		return nil, errors.New("token has been blacklisted/revoked")
	}

	return claims, nil
}

func (s *authTokenService) GenerateRefreshToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
