package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	models "thanawy-backend/internal/domain/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

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
