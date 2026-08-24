// Package shared contains constants and helper functions used by more than one
// handler package (admin, protected, ...).
package shared

import "strconv"

// StringOrEmpty safely dereferences a *string pointer, returning "" if nil.
func StringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// FirstNonEmpty returns the first non-empty string from the given values.
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// ParsePositiveInt parses value as a positive int, returning fallback otherwise.
func ParsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

// CalculateTotalPages returns the number of pages for the given total and limit.
func CalculateTotalPages(total int64, limit int) int64 {
	if limit <= 0 {
		return 1
	}
	pages := total / int64(limit)
	if total%int64(limit) != 0 {
		pages++
	}
	if pages == 0 {
		return 1
	}
	return pages
}
