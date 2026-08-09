package db

import (
	"io"

	"thanawy-backend/internal/infrastructure/database/fileguard"
)

// Compatibility facade over the database/fileguard package.
// New code should import database/fileguard directly.

// ValidateMagicNumber validates file contents against an explicit allow-list.
func ValidateMagicNumber(r io.ReadSeeker, allowedMimes []string) (bool, string, error) {
	return fileguard.ValidateMagicNumber(r, allowedMimes)
}
