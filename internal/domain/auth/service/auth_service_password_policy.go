package authservice

import (
	"errors"
	"strings"
	"unicode"
)

// ─────────────────────────────────────────────
//  Password Policy
// ─────────────────────────────────────────────

// validatePasswordPolicy enforces password complexity requirements.
// Rules: min 8 chars, at least 1 uppercase, 1 lowercase, 1 digit, 1 special char.
// Also rejects commonly breached passwords.
func validatePasswordPolicy(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}
	if len(password) > 128 {
		return errors.New("password must not exceed 128 characters")
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if !hasDigit {
		return errors.New("password must contain at least one digit")
	}
	if !hasSpecial {
		return errors.New("password must contain at least one special character (!@#$%^&*...)")
	}

	// Reject common weak passwords
	lower := strings.ToLower(password)
	commonPasswords := []string{
		"password", "12345678", "qwerty12", "admin123", "letmein1",
		"welcome1", "password1", "123456789", "1234567890",
	}
	for _, weak := range commonPasswords {
		if lower == weak {
			return errors.New("this password is too common, please choose a stronger one")
		}
	}

	return nil
}
