package admin

import (
	models "thanawy-backend/internal/domain/common"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// dashboardChartData holds the chart series rendered by the dashboard.
type dashboardChartData struct {
	UserGrowth []gin.H
	Activity   []gin.H
}

func buildDashboardCharts(readDB *gorm.DB, now time.Time) dashboardChartData {
	sixMonthsAgo := now.AddDate(0, -5, 0)
	startOfSixMonths := time.Date(sixMonthsAgo.Year(), sixMonthsAgo.Month(), 1, 0, 0, 0, 0, sixMonthsAgo.Location())

	return dashboardChartData{
		UserGrowth: buildUserGrowthChart(readDB, now, startOfSixMonths),
		Activity:   buildActivityChart(readDB, now),
	}
}

// buildUserGrowthChart returns monthly signup counts for the trailing 6 months.
// Buckets are keyed by year *and* month: grouping on the month alone merged the
// same calendar month from different years into a single inflated bucket.
func buildUserGrowthChart(readDB *gorm.DB, now time.Time, startOfSixMonths time.Time) []gin.H {
	type monthlyCount struct {
		Bucket string
		Count  int64
	}
	var userGrowthData []monthlyCount
	readDB.Model(&models.User{}).
		Select("TO_CHAR(created_at, 'YYYY-MM') as bucket, COUNT(*) as count").
		Where("deleted_at IS NULL AND created_at >= ?", startOfSixMonths).
		Group("TO_CHAR(created_at, 'YYYY-MM')").
		Scan(&userGrowthData)

	userGrowthMap := make(map[string]int64, len(userGrowthData))
	for _, m := range userGrowthData {
		userGrowthMap[m.Bucket] = m.Count
	}

	userGrowth := make([]gin.H, 0, 6)
	for i := 5; i >= 0; i-- {
		d := now.AddDate(0, -i, 0)
		userGrowth = append(userGrowth, gin.H{
			"month": int(d.Month()),
			"year":  d.Year(),
			"users": userGrowthMap[d.Format("2006-01")],
		})
	}
	return userGrowth
}

// buildActivityChart returns daily study-session counts for the last 12 weeks.
// The window matches the frontend heatmap grid (84 days) so every cell renders
// real data instead of permanent zeros.
func buildActivityChart(readDB *gorm.DB, now time.Time) []gin.H {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).
		AddDate(0, 0, -(dashboardActivityDays - 1))

	type dailyCount struct {
		Bucket string
		Count  int64
	}
	var activityData []dailyCount
	// Bucket on the full date. A DD/MM key collapses the same day-of-month from
	// two different years into one bucket, which an 84-day window can span.
	readDB.Model(&models.StudySession{}).
		Select("TO_CHAR(start_time, 'YYYY-MM-DD') as bucket, COUNT(*) as count").
		Where("deleted_at IS NULL AND start_time >= ?", start).
		Group("TO_CHAR(start_time, 'YYYY-MM-DD')").
		Scan(&activityData)

	activityMap := make(map[string]int64, len(activityData))
	for _, d := range activityData {
		activityMap[d.Bucket] = d.Count
	}

	activityChart := make([]gin.H, 0, dashboardActivityDays)
	for i := dashboardActivityDays - 1; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		activityChart = append(activityChart, gin.H{
			// `day` keeps its DD/MM display format for the existing heatmap.
			"day":      d.Format("02/01"),
			"date":     d.Format(dateFormat),
			"sessions": activityMap[d.Format("2006-01-02")],
		})
	}
	return activityChart
}
