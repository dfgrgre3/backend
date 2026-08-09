package admin

import (
	"sync"
	"time"
)

// dashboardTrendResult groups the growth/trend percentages the UI displays.
type dashboardTrendResult struct {
	UserGrowthTrend     float64
	StudyTimeTrend      float64
	DailyRevenueTrend   float64
	MonthlyRevenueTrend float64
	YearlyRevenueTrend  float64
}

// computeDashboardTrends resolves all trend percentages concurrently.
// The windows mirror loadDashboardRevenueStats exactly so each trend describes
// the figure it is displayed next to.
func computeDashboardTrends(todayStart, monthAgo, yearAgo time.Time) dashboardTrendResult {
	var result dashboardTrendResult

	var wg sync.WaitGroup
	wg.Add(5)
	go func() { defer wg.Done(); result.UserGrowthTrend = calculateUserGrowthTrend() }()
	go func() { defer wg.Done(); result.StudyTimeTrend = calculateStudyTimeTrend() }()
	go func() {
		defer wg.Done()
		result.DailyRevenueTrend = calculateRevenueTrend(todayStart, todayStart.AddDate(0, 0, -1))
	}()
	go func() {
		defer wg.Done()
		result.MonthlyRevenueTrend = calculateRevenueTrend(monthAgo, monthAgo.AddDate(0, -1, 0))
	}()
	go func() {
		defer wg.Done()
		result.YearlyRevenueTrend = calculateRevenueTrend(yearAgo, yearAgo.AddDate(-1, 0, 0))
	}()
	wg.Wait()

	return result
}
