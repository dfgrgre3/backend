package main

import (
	"fmt"
	"time"

	"thanawy-backend/internal/infrastructure/config"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type tokenClaims struct {
	Role string `json:"role"`
	JTI  string `json:"jti"`
	jwt.RegisteredClaims
}

func main() {
	cfg := config.Load()
	jti := uuid.New().String()

	claims := tokenClaims{
		Role: "USER",
		JTI:  jti,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "c2b6f178-dc29-4592-805c-3f41a8b11111", // Dummy valid UUID.
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        jti,
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecretKey))
	if err != nil {
		panic(fmt.Errorf("sign token: %w", err))
	}

	fmt.Println(token)
}
