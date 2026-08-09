package admin

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
)

// admin_dashboard_v1_common.go holds the shared plumbing for the
// /api/admin/dashboard/* endpoint family: permission gating, filter parsing and
// validation, and pagination. The aggregate legacy endpoint
// (GET /api/admin/dashboard) is untouched and remains the compatibility surface
// for the current admin UI.

// ─────────────────────────────────────────────
//  Limits
// ─────────────────────────────────────────────

const (
	// dashboardMaxRangeDays caps a single query window so an admin cannot ask
	// for an unbounded aggregation.
	dashboardMaxRangeDays = 366
	// dashboardMaxSeriesPoints caps time-series cardinality.
	dashboardMaxSeriesPoints = 400
	dashboardDefaultPageSize = 20
	dashboardMaxPageSize     = 100
	dashboardMaxQueryLen     = 200
	dashboardMaxTopLimit     = 50
	dashboardExportTTL       = 24 * time.Hour
)

// ─────────────────────────────────────────────
//  Permission helpers
// ─────────────────────────────────────────────

// dashboardGrants returns the effective grants resolved by the auth middleware.
func dashboardGrants(c *gin.Context) []string {
	raw, _ := c.Get("permissions")
	grants, _ := raw.([]string)
	return grants
}

// dashboardCan reports whether the caller holds the given dashboard permission.
// It reuses the same matcher the middleware uses so a `dashboard:manage` or
// `admin:bypass` grant behaves consistently at widget granularity.
func dashboardCan(c *gin.Context, permission string) bool {
	for _, grant := range dashboardGrants(c) {
		if models.PermissionGrantMatches(grant, permission) {
			return true
		}
	}
	return false
}

// dashboardRequire writes a 403 and reports false when the permission is absent.
func dashboardRequire(c *gin.Context, permission string) bool {
	if dashboardCan(c, permission) {
		return true
	}
	api_response.Error(c, http.StatusForbidden, fmt.Sprintf("Permission '%s' required", permission))
	return false
}

// ─────────────────────────────────────────────
//  Filter parsing
// ─────────────────────────────────────────────

// dashboardFilters is the normalized query state shared by the read endpoints.
type dashboardFilters struct {
	StartDate      time.Time
	EndDate        time.Time
	Granularity    string
	Compare        bool
	UserSegment    string
	CourseCategory string
	Currency       string
	// TenantID is accepted for forward compatibility with a multi-tenant
	// deployment. The current schema has no tenant column, so it is validated
	// and then ignored rather than silently changing query results.
	TenantID string
}

var dashboardGranularities = map[string]bool{"day": true, "week": true, "month": true}

// dashboardAllowedDateRanges are the preset windows the UI may offer.
var dashboardAllowedDateRanges = []struct {
	Code  string
	Label string
	Days  int
}{
	{"today", "اليوم", 1},
	{"last_7_days", "آخر 7 أيام", 7},
	{"last_30_days", "آخر 30 يومًا", 30},
	{"last_90_days", "آخر 90 يومًا", 90},
	{"last_12_months", "آخر 12 شهرًا", 365},
}

// parseDashboardFilters validates and normalizes the shared query parameters.
// It returns false after writing the error response when input is invalid.
func parseDashboardFilters(c *gin.Context) (dashboardFilters, bool) {
	var f dashboardFilters
	now := time.Now()

	startRaw := strings.TrimSpace(c.Query("startDate"))
	endRaw := strings.TrimSpace(c.Query("endDate"))

	switch {
	case startRaw == "" && endRaw == "":
		f.EndDate = now
		f.StartDate = now.AddDate(0, 0, -30)
	case startRaw == "" || endRaw == "":
		api_response.Error(c, http.StatusBadRequest, "startDate and endDate must be provided together")
		return f, false
	default:
		start, ok := parseDashboardDate(startRaw)
		if !ok {
			api_response.Error(c, http.StatusBadRequest, "startDate must be a valid date (YYYY-MM-DD or RFC3339)")
			return f, false
		}
		end, ok := parseDashboardDate(endRaw)
		if !ok {
			api_response.Error(c, http.StatusBadRequest, "endDate must be a valid date (YYYY-MM-DD or RFC3339)")
			return f, false
		}
		if end.Before(start) {
			api_response.Error(c, http.StatusBadRequest, "startDate must be before or equal to endDate")
			return f, false
		}
		if end.Sub(start) > time.Duration(dashboardMaxRangeDays)*24*time.Hour {
			api_response.Error(c, http.StatusBadRequest,
				fmt.Sprintf("date range must not exceed %d days", dashboardMaxRangeDays))
			return f, false
		}
		f.StartDate, f.EndDate = start, end
	}

	if g := strings.ToLower(strings.TrimSpace(c.Query("granularity"))); g != "" {
		if !dashboardGranularities[g] {
			api_response.Error(c, http.StatusBadRequest, "granularity must be one of day, week, month")
			return f, false
		}
		f.Granularity = g
	} else {
		f.Granularity = defaultGranularityFor(f.StartDate, f.EndDate)
	}

	compareRaw := strings.TrimSpace(firstNonEmpty(c.Query("compareWithPrevious"), c.Query("comparisonEnabled")))
	if compareRaw != "" {
		compare, err := strconv.ParseBool(compareRaw)
		if err != nil {
			api_response.Error(c, http.StatusBadRequest, "compareWithPrevious must be a boolean")
			return f, false
		}
		f.Compare = compare
	}

	if cur := strings.ToUpper(strings.TrimSpace(c.Query("currency"))); cur != "" {
		if len(cur) != 3 || !isAlpha(cur) {
			api_response.Error(c, http.StatusBadRequest, "currency must be a 3-letter ISO code")
			return f, false
		}
		f.Currency = cur
	}

	if tenant := strings.TrimSpace(c.Query("tenantId")); tenant != "" {
		if len(tenant) > 64 {
			api_response.Error(c, http.StatusBadRequest, "tenantId is invalid")
			return f, false
		}
		f.TenantID = tenant
	}

	f.UserSegment = strings.TrimSpace(c.Query("userSegment"))
	f.CourseCategory = strings.TrimSpace(firstNonEmpty(c.Query("courseCategory"), c.Query("category")))

	if q := strings.TrimSpace(c.Query("query")); len(q) > dashboardMaxQueryLen {
		api_response.Error(c, http.StatusBadRequest,
			fmt.Sprintf("query must not exceed %d characters", dashboardMaxQueryLen))
		return f, false
	}

	return f, true
}

// parseDashboardDate accepts either a plain date or a full RFC3339 timestamp.
func parseDashboardDate(raw string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339, dateFormat} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

// defaultGranularityFor keeps the point count within a renderable range.
func defaultGranularityFor(start, end time.Time) string {
	days := end.Sub(start).Hours() / 24
	switch {
	case days <= 31:
		return "day"
	case days <= 120:
		return "week"
	default:
		return "month"
	}
}

// previousPeriod mirrors the current window immediately before itself so
// period-over-period deltas compare equal-length spans.
func (f dashboardFilters) previousPeriod() (time.Time, time.Time) {
	span := f.EndDate.Sub(f.StartDate)
	return f.StartDate.Add(-span), f.StartDate
}

func isAlpha(s string) bool {
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// ─────────────────────────────────────────────
//  Pagination & sorting
// ─────────────────────────────────────────────

type dashboardPageParams struct {
	Page      int
	PageSize  int
	Offset    int
	SortBy    string
	Direction string
	Query     string
}

// parseDashboardPage validates paging and sorting against an explicit allowlist
// so a caller cannot inject an arbitrary ORDER BY column.
func parseDashboardPage(c *gin.Context, allowedSort map[string]string, defaultSort string) (dashboardPageParams, bool) {
	p := dashboardPageParams{Page: 1, PageSize: dashboardDefaultPageSize, Direction: "desc"}

	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		page, err := strconv.Atoi(raw)
		if err != nil || page < 1 {
			api_response.Error(c, http.StatusBadRequest, "page must be a positive integer")
			return p, false
		}
		p.Page = page
	}

	if raw := strings.TrimSpace(c.Query("pageSize")); raw != "" {
		size, err := strconv.Atoi(raw)
		if err != nil || size < 1 {
			api_response.Error(c, http.StatusBadRequest, "pageSize must be a positive integer")
			return p, false
		}
		if size > dashboardMaxPageSize {
			api_response.Error(c, http.StatusBadRequest,
				fmt.Sprintf("pageSize must not exceed %d", dashboardMaxPageSize))
			return p, false
		}
		p.PageSize = size
	}

	p.Offset = (p.Page - 1) * p.PageSize

	p.SortBy = defaultSort
	if raw := strings.TrimSpace(c.Query("sortBy")); raw != "" {
		if _, ok := allowedSort[raw]; !ok {
			api_response.Error(c, http.StatusBadRequest,
				fmt.Sprintf("sortBy must be one of %s", strings.Join(sortedKeys(allowedSort), ", ")))
			return p, false
		}
		p.SortBy = raw
	}

	if raw := strings.ToLower(strings.TrimSpace(c.Query("sortDirection"))); raw != "" {
		if raw != "asc" && raw != "desc" {
			api_response.Error(c, http.StatusBadRequest, "sortDirection must be asc or desc")
			return p, false
		}
		p.Direction = raw
	}

	p.Query = strings.TrimSpace(c.Query("query"))
	if len(p.Query) > dashboardMaxQueryLen {
		api_response.Error(c, http.StatusBadRequest,
			fmt.Sprintf("query must not exceed %d characters", dashboardMaxQueryLen))
		return p, false
	}

	return p, true
}

// orderClause resolves the validated sort key to its physical column.
func (p dashboardPageParams) orderClause(allowedSort map[string]string) string {
	column, ok := allowedSort[p.SortBy]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s %s", column, strings.ToUpper(p.Direction))
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// dashboardListResponse is the shared paginated envelope for the list endpoints.
func dashboardListResponse(items interface{}, total int64, p dashboardPageParams, extra gin.H) gin.H {
	payload := gin.H{
		"items":      items,
		"totalCount": total,
		"page":       p.Page,
		"pageSize":   p.PageSize,
		"hasMore":    int64(p.Offset+p.PageSize) < total,
	}
	for key, value := range extra {
		payload[key] = value
	}
	return payload
}

// ─────────────────────────────────────────────
//  Metric shaping
// ─────────────────────────────────────────────

// dashboardDelta derives the comparison fields for a metric card.
func dashboardDelta(current, previous float64, compare bool) (interface{}, interface{}, interface{}, string) {
	if !compare {
		return nil, nil, nil, "flat"
	}
	delta := current - previous
	var percentage float64
	switch {
	case previous != 0:
		percentage = delta / math.Abs(previous) * 100
	case current > 0:
		percentage = 100
	}
	direction := "flat"
	switch {
	case delta > 0:
		direction = "up"
	case delta < 0:
		direction = "down"
	}
	return previous, delta, math.Round(percentage*100) / 100, direction
}
