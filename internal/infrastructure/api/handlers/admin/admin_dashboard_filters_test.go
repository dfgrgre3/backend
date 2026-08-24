package admin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

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
