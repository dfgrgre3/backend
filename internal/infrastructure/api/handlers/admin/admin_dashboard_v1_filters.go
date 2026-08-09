package admin

import (
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"

	api_response "thanawy-backend/internal/infrastructure/api/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetDashboardFiltersMetadata returns the option lists that drive the dashboard
// filter bar. Options the caller cannot act on are omitted rather than disabled,
// so the UI never renders a control that would fail server-side.
//
// GET /api/admin/dashboard/filters
func GetDashboardFiltersMetadata(c *gin.Context) {
	if !dashboardRequire(c, models.PermDashboardAccess) {
		return
	}
	readDB, abort := safeReadDB(c)
	if abort {
		return
	}

	if tenant := strings.TrimSpace(c.Query("tenantId")); len(tenant) > 64 {
		api_response.Error(c, http.StatusBadRequest, "tenantId is invalid")
		return
	}

	dateRanges := make([]gin.H, 0, len(dashboardAllowedDateRanges))
	for i, r := range dashboardAllowedDateRanges {
		dateRanges = append(dateRanges, gin.H{
			"id":        r.Code,
			"code":      r.Code,
			"label":     r.Label,
			"days":      r.Days,
			"isDefault": i == 2, // last_30_days
		})
	}

	granularities := []gin.H{
		{"id": "day", "code": "day", "label": "يومي", "isDefault": true},
		{"id": "week", "code": "week", "label": "أسبوعي", "isDefault": false},
		{"id": "month", "code": "month", "label": "شهري", "isDefault": false},
	}

	comparisonOptions := []gin.H{
		{"id": "none", "code": "none", "label": "بدون مقارنة", "isDefault": true},
		{"id": "previous_period", "code": "previous_period", "label": "الفترة السابقة", "isDefault": false},
	}

	payload := gin.H{
		"allowedDateRanges":    dateRanges,
		"allowedGranularities": granularities,
		"comparisonOptions":    comparisonOptions,
		"courseCategories":     dashboardCourseCategoryOptions(readDB),
		"courseStatuses":       staticFilterOptions([]string{"DRAFT", "REVIEW", "PUBLISHED", "ARCHIVED"}, "PUBLISHED"),
		"userSegments":         dashboardUserSegmentOptions(),
		"enrollmentStatuses":   staticFilterOptions([]string{"ACTIVE", "COMPLETED", "CANCELLED"}, ""),
		"supportPriorities":    staticFilterOptions([]string{"low", "medium", "high", "urgent"}, ""),
		"pendingItemTypes":     dashboardPendingTypeOptions(c),
		"alertSeverities":      staticFilterOptions(models.DashboardAlertSeverities(), ""),
		"alertStates":          staticFilterOptions(models.DashboardAlertStates(), models.DashboardAlertStateOpen),
		"defaultFilterState": gin.H{
			"dateRange":           "last_30_days",
			"granularity":         "day",
			"compareWithPrevious": false,
		},
	}

	// Payment/subscription option lists are financial metadata.
	if dashboardCan(c, models.PermDashboardViewFinancialMetrics) {
		payload["paymentStatuses"] = staticFilterOptions(
			[]string{string(models.PaymentPending), string(models.PaymentCompleted), string(models.PaymentFailed)}, "")
	}

	if dashboardCan(c, models.PermDashboardApplySavedFilters) {
		payload["savedFilters"] = dashboardSavedFilterOptions(c)
	} else {
		payload["savedFilters"] = []gin.H{}
	}

	api_response.Success(c, payload)
}

// staticFilterOptions shapes a fixed value list into the option contract.
func staticFilterOptions(values []string, defaultValue string) []gin.H {
	options := make([]gin.H, 0, len(values))
	for _, v := range values {
		options = append(options, gin.H{
			"id":        v,
			"code":      v,
			"label":     v,
			"isDefault": v == defaultValue,
		})
	}
	return options
}

// dashboardUserSegmentOptions lists the audience slices the summary supports.
func dashboardUserSegmentOptions() []gin.H {
	segments := []struct{ code, label string }{
		{"all", "كل المستخدمين"},
		{"students", "المتعلمون"},
		{"teachers", "المدربون"},
		{"parents", "أولياء الأمور"},
	}
	options := make([]gin.H, 0, len(segments))
	for _, s := range segments {
		options = append(options, gin.H{
			"id":        s.code,
			"code":      s.code,
			"label":     s.label,
			"isDefault": s.code == "all",
		})
	}
	return options
}

// dashboardPendingTypeOptions only advertises queues the caller can open, so a
// Support user is not offered a content-approval filter they cannot use.
func dashboardPendingTypeOptions(c *gin.Context) []gin.H {
	types := []struct{ code, label, permission string }{
		{"course_review", "دورات بانتظار المراجعة", models.PermSubjectsView},
		{"support_ticket", "تذاكر دعم مفتوحة", models.PermTicketsView},
		{"pending_order", "طلبات اشتراك معلقة", models.PermDashboardViewFinancialMetrics},
	}
	options := make([]gin.H, 0, len(types))
	for _, t := range types {
		if !dashboardCan(c, t.permission) {
			continue
		}
		options = append(options, gin.H{
			"id":          t.code,
			"code":        t.code,
			"label":       t.label,
			"isDefault":   false,
			"permissions": []string{t.permission},
		})
	}
	return options
}

// dashboardCourseCategoryOptions reads the live category catalogue.
func dashboardCourseCategoryOptions(readDB *gorm.DB) []gin.H {
	var categories []models.LmsCategory
	readDB.Where("deleted_at IS NULL").Order("name ASC").Limit(200).Find(&categories)

	options := make([]gin.H, 0, len(categories))
	for _, cat := range categories {
		options = append(options, gin.H{
			"id":        cat.ID.String(),
			"code":      cat.Slug,
			"label":     cat.Name,
			"isDefault": false,
		})
	}
	return options
}

// dashboardSavedFilterOptions returns the caller's own saved filters plus any
// shared ones.
func dashboardSavedFilterOptions(c *gin.Context) []gin.H {
	userID, _ := c.Get("userId")
	id, _ := userID.(string)
	if id == "" {
		return []gin.H{}
	}

	readDB := readDashboardDB()
	if readDB == nil {
		return []gin.H{}
	}

	var filters []models.DashboardSavedFilter
	readDB.Where("deleted_at IS NULL AND (user_id = ? OR visibility = ?)", id, models.DashboardFilterVisibilityShared).
		Order("is_default DESC, created_at DESC").
		Limit(100).
		Find(&filters)

	options := make([]gin.H, 0, len(filters))
	for _, f := range filters {
		options = append(options, gin.H{
			"id":          f.ID,
			"code":        f.ID,
			"label":       f.Name,
			"description": stringOrEmpty(f.Description),
			"filterState": f.FilterState,
			"visibility":  f.Visibility,
			"isDefault":   f.IsDefault,
			"isOwner":     f.UserID == id,
		})
	}
	return options
}
