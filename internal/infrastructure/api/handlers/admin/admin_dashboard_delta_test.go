package admin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ─────────────────────────────────────────────
//  Delta / percentage-change semantics
// ─────────────────────────────────────────────

func TestDashboardDelta(t *testing.T) {
	tests := []struct {
		name          string
		current       float64
		previous      float64
		compare       bool
		wantPrevious  interface{}
		wantDelta     interface{}
		wantPercent   interface{}
		wantDirection string
	}{
		{"comparison disabled", 100, 50, false, nil, nil, nil, "flat"},
		{"growth", 150, 100, true, 100.0, 50.0, 50.0, "up"},
		{"decline", 50, 100, true, 100.0, -50.0, -50.0, "down"},
		{"unchanged", 100, 100, true, 100.0, 0.0, 0.0, "flat"},
		// Division-by-zero guard: a jump from nothing is reported as +100%,
		// never as Infinity or NaN.
		{"previous zero with growth", 25, 0, true, 0.0, 25.0, 100.0, "up"},
		{"both zero", 0, 0, true, 0.0, 0.0, 0.0, "flat"},
		// A negative baseline must not invert the sign of the percentage.
		{"negative previous", 50, -100, true, -100.0, 150.0, 150.0, "up"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous, delta, percentage, direction := dashboardDelta(tt.current, tt.previous, tt.compare)
			assert.Equal(t, tt.wantPrevious, previous)
			assert.Equal(t, tt.wantDelta, delta)
			assert.Equal(t, tt.wantPercent, percentage)
			assert.Equal(t, tt.wantDirection, direction)
		})
	}
}

func TestDashboardDeltaNeverProducesNaNOrInfinity(t *testing.T) {
	// Guards the UI contract: percentages are always finite numbers.
	for _, pair := range [][2]float64{{0, 0}, {1, 0}, {0, 1}, {-1, 0}, {1e12, 1e-12}} {
		_, _, percentage, _ := dashboardDelta(pair[0], pair[1], true)
		value, ok := percentage.(float64)
		if assert.True(t, ok, "percentage should be a float64") {
			assert.False(t, isNaN(value), "percentage must not be NaN for %v", pair)
			assert.False(t, isInf(value), "percentage must not be Infinity for %v", pair)
		}
	}
}

func isNaN(f float64) bool { return f != f }
func isInf(f float64) bool { return f > 1e308 || f < -1e308 }
