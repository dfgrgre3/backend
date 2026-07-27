package handlers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseHealthRange(t *testing.T) {
	tests := []struct {
		value    string
		duration time.Duration
		interval time.Duration
	}{
		{"15m", 15 * time.Minute, time.Minute},
		{"1h", time.Hour, 5 * time.Minute},
		{"6h", 6 * time.Hour, 15 * time.Minute},
		{"24h", 24 * time.Hour, time.Hour},
		{"7d", 7 * 24 * time.Hour, 6 * time.Hour},
	}
	for _, test := range tests {
		selected, ok := parseHealthRange(test.value)
		assert.True(t, ok)
		assert.Equal(t, test.duration, selected.Duration)
		assert.Equal(t, test.interval, selected.Interval)
	}
	_, ok := parseHealthRange("30d")
	assert.False(t, ok)
}

func TestHealthSeverityMappings(t *testing.T) {
	assert.Equal(t, "low", examIssueSeverity(1))
	assert.Equal(t, "medium", examIssueSeverity(5))
	assert.Equal(t, "high", examIssueSeverity(20))
	assert.Equal(t, "low", threatLevel(0))
	assert.Equal(t, "medium", threatLevel(1))
	assert.Equal(t, "high", threatLevel(10))
	assert.Equal(t, "critical", threatLevel(20))
}
