package admin

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/storage"
	"time"

	"gorm.io/gorm"
)

// dashboardExportScopes هي النطاقات القابلة للتصدير مقابل الإذن اللازم لكل منها.
var dashboardExportScopes = map[string]string{
	"summary":          models.PermDashboardViewKPIs,
	"pendingActions":   models.PermDashboardViewPendingItems,
	"alerts":           models.PermDashboardViewAlerts,
	"recentActivities": models.PermDashboardViewRecentActivity,
	"topCourses":       models.PermDashboardViewTopCourses,
	"systemHealth":     models.PermDashboardViewSystemHealth,
}

// dashboardExportFormats هي الصيغ المدعومة فعليًا.
var dashboardExportFormats = map[string]bool{"csv": true}

// dashboardExportMaxRangeDays يحدّ نطاق التصدير لتفادي استهلاك ذاكرة مفرط.
const dashboardExportMaxRangeDays = 366

// markDashboardExportFailed يسجّل سبب الفشل على المهمة.
func markDashboardExportFailed(conn *gorm.DB, jobID, code string) {
	conn.Model(&models.DashboardExportJob{}).Where("id = ?", jobID).
		Updates(map[string]interface{}{
			"status":     models.DashboardExportStatusFailed,
			"error_code": code,
			"updated_at": time.Now(),
		})
}

// storeDashboardExport يكتب ملف CSV إلى التخزين ويعيد رابطه.
func storeDashboardExport(ctx context.Context, jobID, scope string, rows [][]string) (string, error) {
	if storage.GlobalStorage == nil {
		return "", fmt.Errorf("storage is not configured")
	}

	var buffer bytes.Buffer
	// BOM حتى تفتح Excel النص العربي بترميز صحيح.
	buffer.WriteString("\ufeff")
	writer := csv.NewWriter(&buffer)
	if err := writer.WriteAll(rows); err != nil {
		return "", err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	filename := fmt.Sprintf("exports/dashboard/%s-%s.csv", scope, jobID)
	payload := buffer.Bytes()
	return storage.GlobalStorage.Upload(ctx, filename,
		bytes.NewReader(payload), int64(len(payload)), "text/csv; charset=utf-8")
}
