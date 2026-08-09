package admin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	models "thanawy-backend/internal/domain/common"

	"github.com/gin-gonic/gin"
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

// ─────────────────────────────────────────────
//  Time bucketing
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

// ─────────────────────────────────────────────
//  Series bucket expression (SQL injection surface)
// ─────────────────────────────────────────────

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

// ─────────────────────────────────────────────
//  Metric registry integrity
// ─────────────────────────────────────────────

func TestDashboardMetricSpecsHaveRequiredFields(t *testing.T) {
	specs := dashboardMetricSpecs()
	assert.NotEmpty(t, specs)

	seen := map[string]bool{}
	for _, spec := range specs {
		t.Run(spec.Key, func(t *testing.T) {
			assert.NotEmpty(t, spec.Key, "metric key is required")
			assert.NotEmpty(t, spec.Title, "metric title is required")
			// Every metric must declare a visibility gate, otherwise it would
			// be readable by any authenticated admin.
			assert.NotEmpty(t, spec.Permission, "metric %s has no permission gate", spec.Key)
			assert.NotNil(t, spec.Compute, "metric %s has no compute function", spec.Key)

			assert.False(t, seen[spec.Key], "duplicate metric key %s", spec.Key)
			seen[spec.Key] = true
		})
	}
}

func TestDashboardMetricSpecsUseKnownPermissions(t *testing.T) {
	// A typo in a permission constant would silently make a metric unreachable,
	// so every gate is checked against the declared dashboard permissions.
	known := map[string]bool{
		models.PermDashboardViewKPIs:             true,
		models.PermDashboardViewLearningMetrics:  true,
		models.PermDashboardViewFinancialMetrics: true,
		models.PermDashboardViewSupportMetrics:   true,
		models.PermDashboardViewContentMetrics:   true,
		models.PermDashboardViewSystemHealth:     true,
		models.PermDashboardViewPendingItems:     true,
		models.PermDashboardViewAlerts:           true,
		models.PermDashboardViewTopCourses:       true,
		models.PermDashboardViewRecentActivity:   true,
	}

	for _, spec := range dashboardMetricSpecs() {
		assert.True(t, known[spec.Permission],
			"metric %s uses unknown permission %q", spec.Key, spec.Permission)
	}
}

func TestTimeSeriesCapableMetricsDeclareAResolver(t *testing.T) {
	// Every metric must either resolve a batched series or be explicitly marked
	// as point-in-time. Without this, the time-series endpoint silently falls
	// back to one query per bucket (the N+1 that was removed).
	for _, spec := range dashboardMetricSpecs() {
		t.Run(spec.Key, func(t *testing.T) {
			assert.True(t, spec.PointInTime || spec.Series != nil,
				"metric %s must declare Series or be marked PointInTime", spec.Key)
		})
	}
}

// ─────────────────────────────────────────────
//  Filter parsing & validation
// ─────────────────────────────────────────────

func newFilterContext(query string) *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/summary?"+query, nil)
	return c
}

func TestParseDashboardFiltersDefaults(t *testing.T) {
	filters, ok := parseDashboardFilters(newFilterContext(""))
	assert.True(t, ok)
	assert.False(t, filters.Compare)
	assert.NotEmpty(t, filters.Granularity)
	assert.True(t, filters.StartDate.Before(filters.EndDate))
}

func TestParseDashboardFiltersRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"start without end", "startDate=2025-01-01"},
		{"end without start", "endDate=2025-01-31"},
		{"unparsable date", "startDate=not-a-date&endDate=2025-01-31"},
		{"inverted range", "startDate=2025-03-01&endDate=2025-01-01"},
		{"range beyond cap", "startDate=2020-01-01&endDate=2025-01-01"},
		{"unknown granularity", "granularity=hourly"},
		{"non-boolean compare", "compareWithPrevious=perhaps"},
		{"bad currency", "currency=EGPP"},
		{"numeric currency", "currency=123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := parseDashboardFilters(newFilterContext(tt.query))
			assert.False(t, ok, "expected %s to be rejected", tt.name)
		})
	}
}

func TestParseDashboardFiltersAcceptsValidRange(t *testing.T) {
	filters, ok := parseDashboardFilters(
		newFilterContext("startDate=2025-01-01&endDate=2025-01-31&granularity=day&compareWithPrevious=true"))

	assert.True(t, ok)
	assert.True(t, filters.Compare)
	assert.Equal(t, "day", filters.Granularity)
	assert.Equal(t, 2025, filters.StartDate.Year())
}

func TestParseDashboardFiltersAcceptsSameDayRange(t *testing.T) {
	_, ok := parseDashboardFilters(newFilterContext("startDate=2025-01-15&endDate=2025-01-15"))
	assert.True(t, ok, "a single-day window is valid")
}

// ─────────────────────────────────────────────
//  Pagination & sorting (ORDER BY injection surface)
// ─────────────────────────────────────────────

func TestParseDashboardPageRejectsUnknownSortKey(t *testing.T) {
	// An unvalidated sortBy would be interpolated straight into ORDER BY.
	injections := []string{
		"created_at DESC--",
		"id",
		"(SELECT 1)",
		"createdAt, priority",
		"createdAt DROP TABLE",
	}

	for _, payload := range injections {
		t.Run(payload, func(t *testing.T) {
			_, ok := parseDashboardPage(
				newFilterContext("sortBy="+url.QueryEscape(payload)), pendingActionSortKeys, "createdAt")
			assert.False(t, ok, "expected sortBy=%q to be rejected", payload)
		})
	}
}

func TestParseDashboardPageRejectsInvalidPaging(t *testing.T) {
	tests := []string{
		"page=0",
		"page=-1",
		"page=abc",
		"pageSize=0",
		"pageSize=-5",
		"pageSize=100000",
		"sortDirection=sideways",
	}

	for _, query := range tests {
		t.Run(query, func(t *testing.T) {
			_, ok := parseDashboardPage(newFilterContext(query), pendingActionSortKeys, "createdAt")
			assert.False(t, ok, "expected %s to be rejected", query)
		})
	}
}

func TestParseDashboardPageDefaultsAndOffset(t *testing.T) {
	page, ok := parseDashboardPage(newFilterContext("page=3&pageSize=20"), pendingActionSortKeys, "createdAt")
	assert.True(t, ok)
	assert.Equal(t, 3, page.Page)
	assert.Equal(t, 20, page.PageSize)
	assert.Equal(t, 40, page.Offset)
	assert.Equal(t, "desc", page.Direction)
}

func TestOrderClauseOnlyEmitsAllowlistedColumns(t *testing.T) {
	page := dashboardPageParams{SortBy: "priority", Direction: "asc"}
	assert.Equal(t, "priority ASC", page.orderClause(pendingActionSortKeys))

	rogue := dashboardPageParams{SortBy: "id; DROP TABLE \"User\"", Direction: "asc"}
	assert.Empty(t, rogue.orderClause(pendingActionSortKeys),
		"a non-allowlisted sort key must produce no ORDER BY fragment")
}

func TestDashboardListResponseHasMore(t *testing.T) {
	page := dashboardPageParams{Page: 1, PageSize: 10, Offset: 0}

	assert.True(t, dashboardListResponse([]string{}, 25, page, nil)["hasMore"].(bool))
	assert.False(t, dashboardListResponse([]string{}, 10, page, nil)["hasMore"].(bool))
	assert.False(t, dashboardListResponse([]string{}, 0, page, nil)["hasMore"].(bool))
}

// ─────────────────────────────────────────────
//  Permission gating
// ─────────────────────────────────────────────

func contextWithGrants(grants ...string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("permissions", grants)
	return c
}

func TestDashboardCanMatchesExactGrant(t *testing.T) {
	c := contextWithGrants(models.PermDashboardViewKPIs)

	assert.True(t, dashboardCan(c, models.PermDashboardViewKPIs))
	// Holding one dashboard permission must not imply the others — especially
	// not the financial one.
	assert.False(t, dashboardCan(c, models.PermDashboardViewFinancialMetrics))
	assert.False(t, dashboardCan(c, models.PermDashboardViewSensitive))
}

func TestDashboardCanWithNoGrants(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.False(t, dashboardCan(c, models.PermDashboardAccess))
}

func TestDashboardRequireWritesForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("permissions", []string{"dashboard:view_kpis"})

	allowed := dashboardRequire(c, "dashboard:view_financial_metrics")

	assert.False(t, allowed)
	assert.Equal(t, http.StatusForbidden, recorder.Code)
	// The denial message must not leak which grants the caller does hold.
	assert.NotContains(t, recorder.Body.String(), "dashboard:view_kpis")
}

func TestDashboardRequireAllowsHolder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("permissions", []string{"dashboard:view_financial_metrics"})

	assert.True(t, dashboardRequire(c, models.PermDashboardViewFinancialMetrics))
	assert.NotEqual(t, http.StatusForbidden, recorder.Code)
}

func TestExportScopesAreAllPermissionGated(t *testing.T) {
	assert.NotEmpty(t, dashboardExportScopes)
	for scope, permission := range dashboardExportScopes {
		assert.NotEmpty(t, permission, "export scope %q has no permission gate", scope)
	}
}

func TestExportFormatsAreRestricted(t *testing.T) {
	// Only formats the backend can actually produce may be accepted; anything
	// else would create a job that never completes.
	assert.True(t, dashboardExportFormats["csv"])
	assert.False(t, dashboardExportFormats["xlsx"])
	assert.False(t, dashboardExportFormats["pdf"])
}

// ─────────────────────────────────────────────
//  Payload filtering — server-side data minimization
// ─────────────────────────────────────────────

// fullDashboardPayload mirrors the shape produced by buildAdminDashboardPayload.
func fullDashboardPayload() map[string]interface{} {
	return map[string]interface{}{
		"stats": map[string]interface{}{
			"totalUsers":       1200,
			"activeStudents":   950,
			"totalTeachers":    86,
			"dailyRevenue":     12500,
			"monthlyRevenue":   360000,
			"publishedCourses": 64,
			"archivedCourses":  18,
			"totalExams":       221,
		},
		"revenue":        map[string]interface{}{"dailyRevenue": 12500},
		"payments":       []interface{}{map[string]interface{}{"id": "pay-1"}},
		"users":          map[string]interface{}{"totalUsers": 1200},
		"teachers":       map[string]interface{}{"totalTeachers": 86},
		"courses":        map[string]interface{}{"publishedCourses": 64},
		"recentActivity": []interface{}{map[string]interface{}{"id": "act-1"}},
		"security":       []interface{}{map[string]interface{}{"id": "sec-1"}},
		"systemAlerts":   []interface{}{map[string]interface{}{"id": "alert-1"}},
		"exams":          []interface{}{},
		"assignments":    []interface{}{},
		"live":           []interface{}{},
		"charts":         map[string]interface{}{},
	}
}

func filterFor(t *testing.T, grants ...string) map[string]interface{} {
	t.Helper()
	filtered, ok := filterDashboardPayload(contextWithGrants(grants...), fullDashboardPayload()).(map[string]interface{})
	if !ok {
		t.Fatalf("filterDashboardPayload did not return a map")
	}
	return filtered
}

func TestFilterDashboardPayloadStripsFinancialDataWithoutPermission(t *testing.T) {
	filtered := filterFor(t, models.PermDashboardViewKPIs)

	// The blocks must be absent from the response, not merely hidden by the UI.
	assert.NotContains(t, filtered, "revenue")
	assert.NotContains(t, filtered, "payments")

	stats := filtered["stats"].(map[string]interface{})
	assert.NotContains(t, stats, "dailyRevenue")
	assert.NotContains(t, stats, "monthlyRevenue")

	// The permission the caller does hold is still served.
	assert.Contains(t, stats, "totalUsers")
}

func TestFilterDashboardPayloadStripsAudienceDataWithoutKPIPermission(t *testing.T) {
	filtered := filterFor(t, models.PermDashboardViewFinancialMetrics)

	assert.NotContains(t, filtered, "users")
	assert.NotContains(t, filtered, "teachers")

	stats := filtered["stats"].(map[string]interface{})
	assert.NotContains(t, stats, "totalUsers")
	assert.NotContains(t, stats, "activeStudents")

	assert.Contains(t, filtered, "revenue")
	assert.Contains(t, stats, "dailyRevenue")
}

func TestFilterDashboardPayloadStripsContentDataWithoutPermission(t *testing.T) {
	filtered := filterFor(t, models.PermDashboardViewKPIs)

	for _, key := range []string{"courses", "exams", "assignments", "live"} {
		assert.NotContains(t, filtered, key)
	}

	stats := filtered["stats"].(map[string]interface{})
	assert.NotContains(t, stats, "publishedCourses")
	assert.NotContains(t, stats, "archivedCourses")
	assert.NotContains(t, stats, "totalExams")
}

func TestFilterDashboardPayloadStripsActivityAndAlertsWithoutPermission(t *testing.T) {
	filtered := filterFor(t, models.PermDashboardViewKPIs)

	assert.NotContains(t, filtered, "recentActivity")
	assert.NotContains(t, filtered, "systemAlerts")
	assert.NotContains(t, filtered, "security")
}

func TestFilterDashboardPayloadWithNoGrantsLeaksNothing(t *testing.T) {
	// The strongest case: an authenticated admin holding no dashboard grants
	// must receive none of the gated blocks.
	filtered := filterFor(t)

	for _, key := range []string{
		"revenue", "payments", "users", "teachers", "courses",
		"recentActivity", "security", "systemAlerts", "exams", "assignments", "live",
	} {
		assert.NotContains(t, filtered, key, "block %q leaked to a caller with no grants", key)
	}

	stats := filtered["stats"].(map[string]interface{})
	for _, key := range []string{
		"totalUsers", "activeStudents", "totalTeachers",
		"dailyRevenue", "monthlyRevenue", "publishedCourses", "totalExams",
	} {
		assert.NotContains(t, stats, key, "stat %q leaked to a caller with no grants", key)
	}
}

func TestFilterDashboardPayloadKeepsEverythingForFullyGrantedAdmin(t *testing.T) {
	filtered := filterFor(t,
		models.PermDashboardViewKPIs,
		models.PermDashboardViewFinancialMetrics,
		models.PermDashboardViewContentMetrics,
		models.PermDashboardViewRecentActivity,
		models.PermDashboardViewAlerts,
		models.PermDashboardViewSystemHealth,
	)

	for _, key := range []string{
		"revenue", "payments", "users", "teachers", "courses",
		"recentActivity", "security", "systemAlerts",
	} {
		assert.Contains(t, filtered, key)
	}
}

func TestFilterDashboardPayloadDoesNotMutateTheCachedPayload(t *testing.T) {
	// The payload is shared through the Redis cache across callers, so filtering
	// must copy rather than delete in place — otherwise the first unprivileged
	// request would permanently strip data for everyone else.
	original := fullDashboardPayload()

	filtered := filterDashboardPayload(contextWithGrants(), original).(map[string]interface{})
	assert.NotContains(t, filtered, "revenue")

	assert.Contains(t, original, "revenue", "the cached payload must not be mutated")
	originalStats := original["stats"].(map[string]interface{})
	assert.Contains(t, originalStats, "dailyRevenue", "cached stats must not be mutated")
}
