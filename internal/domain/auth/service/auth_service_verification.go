package authservice

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// generateSixDigitCode returns a uniformly random 6-digit numeric code
// ("000000"-"999999") using a CSPRNG. It replaces a previous ad-hoc
// bit-packing scheme (byte0<<16 | byte1<<8 | byte2%1000000) whose modulo
// bound only the last byte (Go's % has higher precedence than |), so the
// combined 24-bit value was never actually reduced mod 1,000,000 — codes
// were instead truncated to their first 6 characters, producing a biased,
// non-uniform code space instead of the intended 1-in-1,000,000 code.
func generateSixDigitCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
