package admin

import (
	"context"
	"time"

	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

// buildAdminDashboardPayload aggregates every widget section of the admin
// dashboard in one payload. It is cached under cacheKey and coalesced by
// singleflight so the UI's parallel widget requests share a single computation.
func buildAdminDashboardPayload(cacheKey string, timeFilter string) (map[string]interface{}, error) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekAgo := now.AddDate(0, 0, -7)
	monthAgo := now.AddDate(0, -1, 0)
	yearAgo := now.AddDate(-1, 0, 0)
	periodStart := dashboardPeriodStart(now, timeFilter)

	readDB := db.ReadDB()
	if readDB == nil {
		readDB = db.DB
	}
	conn := db.DB

	// Core platform counters.
	core, err := loadDashboardCoreStats(readDB, todayStart, weekAgo)
	if err != nil {
		return nil, err
	}

	// The remaining counter groups are independent, so they run concurrently.
	// Errors are propagated instead of swallowed: a failed aggregate used to be
	// indistinguishable from a genuine zero on the dashboard.
	var (
		audience   dashboardAudienceStats
		catalog    dashboardCatalogStats
		revenue    dashboardRevenueStats
		operations dashboardOperationsStats
	)
	group, _ := errgroup.WithContext(context.Background())
	// Each goroutine gets its own *gorm.DB session: the base readDB is shared
	// across all of them, and starting concurrent query chains directly off
	// a shared *gorm.DB races on its internal Statement/clause maps (observed
	// as "fatal error: concurrent map writes" under load). NewDB:true resets
	// the Statement instead of cloning it — a plain Session{} clone carries
	// over whatever .Model()/.Table() the base statement last had, which leaks
	// across goroutines (observed as queries against a "dashboardCoreStats"
	// table that doesn't exist).
	group.Go(func() (err error) {
		audience, err = loadDashboardAudienceStats(readDB.Session(&gorm.Session{NewDB: true}), todayStart, weekAgo, monthAgo)
		return err
	})
	group.Go(func() (err error) {
		catalog, err = loadDashboardCatalogStats(readDB.Session(&gorm.Session{NewDB: true}))
		return err
	})
	group.Go(func() (err error) {
		// monthlyRevenue and yearlyRevenue now use real month and year windows.
		revenue, err = loadDashboardRevenueStats(readDB.Session(&gorm.Session{NewDB: true}), todayStart, monthAgo, yearAgo)
		return err
	})
	group.Go(func() (err error) {
		operations, err = loadDashboardOperationsStats(readDB.Session(&gorm.Session{NewDB: true}), periodStart)
		return err
	})
	if err := group.Wait(); err != nil {
		return nil, err
	}

	recent := loadDashboardRecentItems(conn, readDB)

	recentActivityItems := buildDashboardRecentActivity(recent.RecentAssignments)
	upcomingEvents := buildDashboardUpcomingEvents(recent.UpcomingExams)
	topSellingCourses := buildDashboardTopCourses(conn)
	charts := buildDashboardCharts(readDB, now)
	trends := computeDashboardTrends(todayStart, monthAgo, yearAgo)

	growthRate := float64(0)
	if core.TotalUsers > 0 {
		growthRate = float64(core.NewUsersThisWeek) / float64(core.TotalUsers) * 100
	}

	teacherGrowthRate := float64(0)
	if audience.TotalTeachers > 0 {
		teacherGrowthRate = float64(audience.NewTeachersThisWeek) / float64(audience.TotalTeachers) * 100
	}

	goals := buildDashboardGoals(core.TotalUsers, core.NewUsersThisWeek, core.StudyMinutes, revenue.Monthly, catalog.TotalEnrollments)
	criticalKPIs := buildDashboardCriticalKPIs(
		audience.ActiveStudents, core.TotalUsers, catalog.PublishedCourses,
		core.TotalSubjects, catalog.CompletionRate(), revenue.Daily,
	)
	systemAlerts := buildDashboardSystemAlerts(
		catalog.ReviewCourses, operations.OpenTickets,
		int64(len(recent.SecurityAlerts)), int64(len(recent.LiveClasses)),
	)

	responsePayload := gin.H{
		"stats": gin.H{
			"totalUsers":             core.TotalUsers,
			"activeStudents":         audience.ActiveStudents,
			"totalTeachers":          audience.TotalTeachers,
			"totalSubjects":          core.TotalSubjects,
			"publishedCourses":       catalog.PublishedCourses,
			"reviewCourses":          catalog.ReviewCourses,
			"draftCourses":           catalog.DraftCourses,
			"archivedCourses":        catalog.ArchivedCourses,
			"totalExams":             core.TotalExams,
			"totalResources":         core.TotalResources,
			"activeChallenges":       core.ActiveChallenges,
			"newUsersToday":          core.NewUsersToday,
			"newUsersThisWeek":       core.NewUsersThisWeek,
			"dailyRevenue":           revenue.Daily,
			"monthlyRevenue":         revenue.Monthly,
			"newSubscriptions":       operations.NewSubscriptions,
			"cancelledSubscriptions": operations.CancelledSubscriptions,
			"pendingOrders":          operations.PendingOrders,
			"openTickets":            operations.OpenTickets,
			"moderationQueue":        catalog.ReviewCourses,
			"pendingApprovals":       operations.PendingOrders,
			"completionRate":         catalog.CompletionRate(),
			"totalEnrollments":       catalog.TotalEnrollments,
			"totalLessons":           catalog.TotalLessons,
		},
		"trends": gin.H{
			"userGrowth": trends.UserGrowthTrend,
			"studyTime":  trends.StudyTimeTrend,
		},
		"charts": gin.H{
			"userGrowth": charts.UserGrowth,
			"activity":   charts.Activity,
		},
		"activity": gin.H{
			"tasksCompleted":     core.CompletedTasks,
			"examsTaken":         core.ExamsTaken,
			"achievementsEarned": core.AchievementsEarned,
			"studyMinutes":       core.StudyMinutes,
		},
		"recentActivity": recentActivityItems,
		"upcomingEvents": upcomingEvents,
		"revenue": gin.H{
			"dailyRevenue":   revenue.Daily,
			"monthlyRevenue": revenue.Monthly,
			"yearlyRevenue":  revenue.Yearly,
			"pendingRevenue": revenue.Pending,
			"dailyTrend":     trends.DailyRevenueTrend,
			"monthlyTrend":   trends.MonthlyRevenueTrend,
			"yearlyTrend":    trends.YearlyRevenueTrend,
		},
		"users": gin.H{
			"totalUsers":        core.TotalUsers,
			"activeStudents":    audience.ActiveStudents,
			"newUsersToday":     core.NewUsersToday,
			"newUsersThisWeek":  core.NewUsersThisWeek,
			"newUsersThisMonth": audience.NewUsersThisMonth,
			"studentGrowthRate": growthRate,
			"recentStudents":    recent.RecentUsers,
		},
		"teachers": gin.H{
			"totalTeachers":       audience.TotalTeachers,
			"activeTeachers":      audience.TotalTeachers,
			"newTeachersToday":    audience.NewTeachersToday,
			"newTeachersThisWeek": audience.NewTeachersThisWeek,
			"teacherGrowthRate":   teacherGrowthRate,
			"recentTeachers":      recent.RecentTeachers,
		},
		"courses": gin.H{
			"totalCourses":      core.TotalSubjects,
			"publishedCourses":  catalog.PublishedCourses,
			"draftCourses":      catalog.DraftCourses,
			"reviewCourses":     catalog.ReviewCourses,
			"archivedCourses":   catalog.ArchivedCourses,
			"totalLessons":      catalog.TotalLessons,
			"totalEnrollments":  catalog.TotalEnrollments,
			"activeEnrollments": catalog.TotalEnrollments,
			"recentCourses":     recent.RecentCourses,
		},
		"payments": gin.H{
			"recentOrders":   recent.RecentOrders,
			"recentPayments": recent.RecentPayments,
		},
		"exams": gin.H{
			"exams": buildDashboardExams(recent.RecentExams),
		},
		"assignments": gin.H{
			"assignments": buildDashboardAssignments(recent.RecentAssignments),
		},
		"announcements": gin.H{
			"announcements": buildDashboardAnnouncements(recent.Announcements),
		},
		"live": gin.H{
			"classes": buildDashboardLiveClasses(recent.LiveClasses),
		},
		"security": gin.H{
			"alerts": buildDashboardSecurityAlerts(recent.SecurityAlerts),
		},
		"goals":             goals,
		"criticalKPIs":      criticalKPIs,
		"systemAlerts":      systemAlerts,
		"topSellingCourses": topSellingCourses,
		"timeFilter":        timeFilter,
	}

	cacheAdminDashboard(cacheKey, responsePayload)

	return responsePayload, nil
}
