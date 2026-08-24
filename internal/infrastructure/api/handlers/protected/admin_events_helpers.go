package protected

import (
	"fmt"
	"time"
)

// whereIDEquals is the GORM query for fetching by primary key.
const whereIDEquals = "id = ?"

// parseFlexibleDate parses dates in multiple ISO 8601 formats.
func parseFlexibleDate(dateStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}
