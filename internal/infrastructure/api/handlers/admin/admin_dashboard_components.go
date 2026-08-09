package admin

import (
	"sync"
	models "thanawy-backend/internal/domain/common"

	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// dashboardRecentItems bundles the recent-rows lists the dashboard renders.
type dashboardRecentItems struct {
	RecentUsers       []models.User
	RecentTeachers    []models.User
	RecentCourses     []models.LmsCourse
	RecentOrders      []models.UserSubscription
	RecentPayments    []models.Payment
	RecentExams       []models.Exam
	RecentAssignments []models.Task
	Announcements     []models.Notification
	LiveClasses       []models.LiveSession
	SecurityAlerts    []models.SecurityLog
	UpcomingExams     []models.Exam
}

func loadDashboardRecentItems(conn *gorm.DB, readDB *gorm.DB) dashboardRecentItems {
	var items dashboardRecentItems

	var wg sync.WaitGroup
	wg.Add(11)

	go func() { defer wg.Done(); conn.Order(createdAtDescSort).Limit(5).Find(&items.RecentUsers) }()
	go func() {
		defer wg.Done()
		conn.Where("deleted_at IS NULL AND role = ?", models.RoleTeacher).
			Order(createdAtDescSort).Limit(5).Find(&items.RecentTeachers)
	}()
	go func() { defer wg.Done(); conn.Order(createdAtDescSort).Limit(5).Find(&items.RecentCourses) }()
	go func() {
		defer wg.Done()
		userSubscriptionScope(conn).Order(createdAtDescSort).Limit(5).Find(&items.RecentOrders)
	}()
	go func() { defer wg.Done(); conn.Order(createdAtDescSort).Limit(5).Find(&items.RecentPayments) }()
	go func() { defer wg.Done(); conn.Order(createdAtDescSort).Limit(5).Find(&items.RecentExams) }()
	go func() { defer wg.Done(); conn.Order(createdAtDescSort).Limit(5).Find(&items.RecentAssignments) }()
	go func() { defer wg.Done(); conn.Order(createdAtDescSort).Limit(5).Find(&items.Announcements) }()
	go func() {
		defer wg.Done()
		conn.Where("deleted_at IS NULL AND status = ?", "LIVE").
			Order(createdAtDescSort).Limit(5).Find(&items.LiveClasses)
	}()
	go func() { defer wg.Done(); conn.Order(createdAtDescSort).Limit(5).Find(&items.SecurityAlerts) }()
	go func() { defer wg.Done(); readDB.Order(createdAtDescSort).Limit(5).Find(&items.UpcomingExams) }()

	wg.Wait()
	return items
}

func buildDashboardRecentActivity(tasks []models.Task) []gin.H {
	taskUserIDs := make([]string, 0, len(tasks))
	for _, t := range tasks {
		taskUserIDs = append(taskUserIDs, t.UserID)
	}

	var taskUsers []models.User
	if len(taskUserIDs) > 0 {
		db.DB.Where(idInQuery, taskUserIDs).Find(&taskUsers)
	}
	userMap := make(map[string]models.User, len(taskUsers))
	for _, u := range taskUsers {
		userMap[u.ID] = u
	}

	items := make([]gin.H, 0, len(tasks))
	for _, t := range tasks {
		user := userMap[t.UserID]
		items = append(items, gin.H{
			"id":          t.ID,
			"userId":      t.UserID,
			"type":        "task",
			"title":       t.Title,
			"description": stringOrEmpty(t.Description),
			"createdAt":   t.CreatedAt,
			"user": gin.H{
				"name":   firstNonEmpty(stringOrEmpty(user.Name), stringOrEmpty(user.Username), user.Email),
				"avatar": user.Avatar,
			},
		})
	}
	return items
}

func buildDashboardUpcomingEvents(exams []models.Exam) []gin.H {
	items := make([]gin.H, 0, len(exams))
	for _, e := range exams {
		items = append(items, gin.H{
			"id":    e.ID,
			"title": e.Title,
			"date":  e.CreatedAt,
			"type":  "exam",
		})
	}
	return items
}

func buildDashboardTopCourses(conn *gorm.DB) []gin.H {
	type topCourseRow struct {
		ID    string `gorm:"column:id"`
		Title string `gorm:"column:title"`
		Sales int64  `gorm:"column:sales"`
	}
	var topCourseRows []topCourseRow
	conn.Model(&models.LmsEnrollment{}).
		Select(`"LmsCourse".id, "LmsCourse".title, COUNT(*) as sales`).
		Joins(`JOIN "LmsCourse" ON "LmsEnrollment".course_id = "LmsCourse".id`).
		Where(`"LmsEnrollment".deleted_at IS NULL AND "LmsCourse".deleted_at IS NULL`).
		Group(`"LmsCourse".id, "LmsCourse".title`).
		Order("sales DESC").
		Limit(5).
		Scan(&topCourseRows)

	var avgPayment float64
	conn.Model(&models.Payment{}).
		Select("COALESCE(AVG(amount), 0)").
		Where("deleted_at IS NULL AND status = ?", models.PaymentCompleted).
		Scan(&avgPayment)

	topSellingCourses := make([]gin.H, 0, len(topCourseRows))
	for _, row := range topCourseRows {
		topSellingCourses = append(topSellingCourses, gin.H{
			"id":      row.ID,
			"title":   row.Title,
			"sales":   row.Sales,
			"revenue": float64(row.Sales) * avgPayment,
		})
	}
	return topSellingCourses
}
