package authservice

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/config"
)

func TestAccessTokenUsesRS256KeysInProduction(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}

	service := &authTokenService{cfg: &config.Config{
		Environment:   "production",
		JWTPrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})),
		JWTPublicKey:  string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})),
		JWTIssuerURL:  "https://issuer.example.test",
	}}

	pair, err := service.GenerateTokenPair(&models.User{
		ID:     "user-1",
		Email:  "user@example.test",
		Role:   models.RoleStudent,
		Status: models.StatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}

	claims, err := service.ValidateAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != "user-1" {
		t.Fatalf("expected user-1, got %q", claims.UserID)
	}
}
