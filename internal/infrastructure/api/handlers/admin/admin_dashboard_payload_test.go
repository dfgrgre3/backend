package admin

import (
	"testing"

	models "thanawy-backend/internal/domain/common"

	"github.com/stretchr/testify/assert"
)

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
