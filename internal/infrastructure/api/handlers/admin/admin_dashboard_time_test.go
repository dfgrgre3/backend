package admin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ─────────────────────────────────────────────
//  Time bucketing & Series expressions
// ─────────────────────────────────────────────

func TestTruncateToGranularity(t *testing.T) {
	// Wednesday, 2025-03-12T13:45:30Z
	subject := time.Date(2025, 3, 12, 13, 45, 30, 0, time.UTC)

	tests := []struct {
		granularity string
		want        time.Time
	}{
		{"day", time.Date(2025, 3, 12, 0, 0, 0, 0, time.UTC)},
		// Weeks align to Monday.
		{"week", time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)},
		{"month", time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.granularity, func(t *testing.T) {
			assert.Equal(t, tt.want, truncateToGranularity(subject, tt.granularity))
		})
	}
}

func TestTruncateToGranularityWeekOnMonday(t *testing.T) {
	monday := time.Date(2025, 3, 10, 9, 0, 0, 0, time.UTC)
	assert.Equal(t, time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC), truncateToGranularity(monday, "week"))

	sunday := time.Date(2025, 3, 16, 23, 59, 0, 0, time.UTC)
	assert.Equal(t, time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC), truncateToGranularity(sunday, "week"))
}

func TestDashboardBucketsAreContiguousAndClamped(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC)

	buckets := dashboardBuckets(start, end, "day")
	assert.Len(t, buckets, 4)

	// No gaps and no overlaps between consecutive buckets.
	for i := 1; i < len(buckets); i++ {
		assert.Equal(t, buckets[i-1].End, buckets[i].Start)
	}
	// The window is never widened beyond the requested range.
	assert.Equal(t, start, buckets[0].Start)
	assert.Equal(t, end, buckets[len(buckets)-1].End)
}

func TestDashboardBucketsClampPartialFinalPeriod(t *testing.T) {
	// A month-granular range ending mid-month must not report a full month.
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 2, 10, 0, 0, 0, 0, time.UTC)

	buckets := dashboardBuckets(start, end, "month")
	assert.Len(t, buckets, 2)
	assert.Equal(t, end, buckets[1].End, "final bucket must be clamped to endDate")
}

func TestDashboardBucketsCrossYearBoundary(t *testing.T) {
	start := time.Date(2024, 12, 30, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	buckets := dashboardBuckets(start, end, "day")
	assert.Len(t, buckets, 3)

	// Bucket keys must stay distinct across the year boundary. Keying on
	// day-of-month alone previously merged these into one bucket.
	seen := map[string]bool{}
	for _, b := range buckets {
		key := truncateToGranularity(b.Start, "day").Format("2006-01-02")
		assert.False(t, seen[key], "duplicate bucket key %s", key)
		seen[key] = true
	}
}

func TestDashboardBucketsLeapDay(t *testing.T) {
	start := time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)

	buckets := dashboardBuckets(start, end, "day")
	assert.Len(t, buckets, 2, "29 Feb must be its own bucket in a leap year")
	assert.Equal(t, 29, buckets[1].Start.Day())
}

func TestDashboardBucketsRespectMaxPoints(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(3, 0, 0)

	buckets := dashboardBuckets(start, end, "day")
	assert.LessOrEqual(t, len(buckets), dashboardMaxSeriesPoints+1,
		"bucket generation must be bounded so a chart cannot exhaust the server")
}

func TestAdvanceGranularity(t *testing.T) {
	base := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC), advanceGranularity(base, "day"))
	assert.Equal(t, time.Date(2025, 2, 7, 0, 0, 0, 0, time.UTC), advanceGranularity(base, "week"))
	// Go normalizes 31 Feb to 3 Mar; documented here so the behaviour is not
	// mistaken for a bug when reading month-granular series.
	assert.Equal(t, time.Date(2025, 3, 3, 0, 0, 0, 0, time.UTC), advanceGranularity(base, "month"))
}

func TestDefaultGranularityFor(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	assert.Equal(t, "day", defaultGranularityFor(start, start.AddDate(0, 0, 20)))
	assert.Equal(t, "week", defaultGranularityFor(start, start.AddDate(0, 0, 60)))
	assert.Equal(t, "month", defaultGranularityFor(start, start.AddDate(0, 0, 300)))
}

func TestPreviousPeriodMirrorsWindowLength(t *testing.T) {
	f := dashboardFilters{
		StartDate: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2025, 3, 31, 0, 0, 0, 0, time.UTC),
	}
	prevStart, prevEnd := f.previousPeriod()

	assert.Equal(t, f.StartDate, prevEnd, "previous window must end where the current one starts")
	assert.Equal(t, f.EndDate.Sub(f.StartDate), prevEnd.Sub(prevStart),
		"compared windows must be the same length")
}

func TestDashboardSeriesBucketExpr(t *testing.T) {
	assert.Equal(t,
		"TO_CHAR(date_trunc('day', created_at), 'YYYY-MM-DD')",
		dashboardSeriesBucketExpr("created_at", "day"))
	assert.Equal(t,
		"TO_CHAR(date_trunc('week', created_at), 'YYYY-MM-DD')",
		dashboardSeriesBucketExpr("created_at", "week"))
	assert.Equal(t,
		"TO_CHAR(date_trunc('month', created_at), 'YYYY-MM-DD')",
		dashboardSeriesBucketExpr("created_at", "month"))
}

func TestDashboardSeriesBucketExprRejectsUnknownGranularity(t *testing.T) {
	// Granularity is validated upstream, but the expression builder must still
	// fall back to a safe literal rather than interpolating attacker input.
	expr := dashboardSeriesBucketExpr("created_at", "'); DROP TABLE \"User\"; --")
	assert.Equal(t, "TO_CHAR(date_trunc('day', created_at), 'YYYY-MM-DD')", expr)
	assert.NotContains(t, expr, "DROP TABLE")
}
