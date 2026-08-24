package admin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ─────────────────────────────────────────────
//  Completion rate
// ─────────────────────────────────────────────

func TestDashboardCatalogStatsCompletionRate(t *testing.T) {
	tests := []struct {
		name      string
		stats     dashboardCatalogStats
		wantRatio float64
	}{
		{"no enrollments", dashboardCatalogStats{}, 0},
		{"half completed", dashboardCatalogStats{LmsEnrollments: 200, CompletedEnrollments: 100}, 50},
		{"all completed", dashboardCatalogStats{LmsEnrollments: 10, CompletedEnrollments: 10}, 100},
		{"none completed", dashboardCatalogStats{LmsEnrollments: 10}, 0},
		// The legacy "Enrollment" table must not act as the denominator; if it
		// did, this case would report 500% instead of 50%.
		{
			"legacy enrollment count is ignored",
			dashboardCatalogStats{TotalEnrollments: 20, LmsEnrollments: 200, CompletedEnrollments: 100},
			50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.InDelta(t, tt.wantRatio, tt.stats.CompletionRate(), 0.0001)
		})
	}
}

func TestCompletionRateNeverExceedsHundred(t *testing.T) {
	stats := dashboardCatalogStats{LmsEnrollments: 1000, CompletedEnrollments: 1000}
	assert.LessOrEqual(t, stats.CompletionRate(), 100.0)
}
