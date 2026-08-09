package authservice

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"
	"time"
)

type MFAService interface {
	GenerateTOTPSecret(email string) (secret string, qrCodeURL string, err error)
	ValidateTOTP(secret string, code string) bool
	GenerateBackupCodes() []string
}

type mfaService struct{}

func NewMFAService() MFAService {
	return &mfaService{}
}

func (s *mfaService) GenerateTOTPSecret(email string) (secret string, qrCodeURL string, err error) {
	// Generate 20-byte random secret
	randomBytes := make([]byte, 20)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", err
	}

	// Base32 encode the secret
	secret = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)

	// Format issuer and email for the QR code URL
	issuer := "ToloThanawy"
	qrCodeURL = fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		issuer, email, secret, issuer)

	return secret, qrCodeURL, nil
}

func (s *mfaService) ValidateTOTP(secret string, code string) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}

	// Clean up secret padding if any
	secret = strings.ToUpper(strings.TrimSpace(secret))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		// Try decoding with standard padding if no-padding fails
		key, err = base32.StdEncoding.DecodeString(secret)
		if err != nil {
			return false
		}
	}

	// Get current time step (30 seconds)
	epochSeconds := time.Now().Unix()
	currentStep := epochSeconds / 30

	// Check current time step, previous one, and next one (allow clock skew of 30 seconds)
	for i := int64(-1); i <= 1; i++ {
		step := currentStep + i
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(step))

		// Compute HMAC-SHA1
		mac := hmac.New(sha1.New, key)
		mac.Write(buf)
		sum := mac.Sum(nil)

		// Dynamic truncation
		offset := sum[len(sum)-1] & 0x0f
		binaryVal := binary.BigEndian.Uint32(sum[offset : offset+4])
		binaryVal = binaryVal & 0x7fffffff
		otp := binaryVal % 1000000

		otpStr := fmt.Sprintf("%06d", otp)
		if otpStr == code {
			return true
		}
	}

	return false
}

func (s *mfaService) GenerateBackupCodes() []string {
	codes := make([]string, 8)
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // Easily readable characters
	for i := 0; i < 8; i++ {
		var codeBuilder strings.Builder
		for j := 0; j < 8; j++ {
			n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
			codeBuilder.WriteByte(chars[n.Int64()])
		}
		// Format as XXXX-XXXX
		val := codeBuilder.String()
		codes[i] = val[:4] + "-" + val[4:]
	}
	return codes
}
