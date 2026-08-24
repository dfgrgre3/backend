package admin

import (
	"testing"

	models "thanawy-backend/internal/domain/common"

	"github.com/stretchr/testify/assert"
)

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
