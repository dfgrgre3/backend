package protected

import (
	"fmt"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
)

func AdminUsersAnalytics(c *gin.Context) {
	now := time.Now()
	sixMonthsAgo := now.AddDate(0, -6, 0)
	startOfSixMonths := time.Date(sixMonthsAgo.Year(), sixMonthsAgo.Month(), 1, 0, 0, 0, 0, sixMonthsAgo.Location())

	// User growth over time (monthly)
	type monthlyCount struct {
		Year  int
		Month int
		Count int64
	}
	var userGrowth []monthlyCount
	db.DB.Model(&models.User{}).
		Select("EXTRACT(YEAR FROM created_at) as year, EXTRACT(MONTH FROM created_at) as month, COUNT(*) as count").
		Where("created_at >= ? AND deleted_at IS NULL", startOfSixMonths).
		Group("EXTRACT(YEAR FROM created_at), EXTRACT(MONTH FROM created_at)").
		Order("year ASC, month ASC").
		Scan(&userGrowth)

	growthChart := make([]gin.H, 0, 6)
	for i := 5; i >= 0; i-- {
		d := now.AddDate(0, -i, 0)
		month := int(d.Month())
		year := d.Year()
		count := int64(0)
		for _, m := range userGrowth {
			if m.Year == year && m.Month == month {
				count = m.Count
				break
			}
		}
		monthNames := map[int]string{
			1: "يناير", 2: "فبراير", 3: "مارس", 4: "أبريل",
			5: "مايو", 6: "يونيو", 7: "يوليو", 8: "أغسطس",
			9: "سبتمبر", 10: "أكتوبر", 11: "نوفمبر", 12: "ديسمبر",
		}
		growthChart = append(growthChart, gin.H{
			"name":  monthNames[month],
			"users": count,
		})
	}

	// Users by role (pie chart data)
	type roleCount struct {
		Role  string
		Count int64
	}
	var roleCounts []roleCount
	db.DB.Model(&models.User{}).
		Select("role, COUNT(*) as count").
		Where("deleted_at IS NULL").
		Group("role").
		Scan(&roleCounts)

	roleLabels := map[string]string{
		"STUDENT":     "طلاب",
		"TEACHER":     "معلمون",
		"MODERATOR":   "مشرفون",
		"ADMIN":       "مدراء",
		"SUPER_ADMIN": "مدراء",
		"PARENT":      "أولياء أمور",
		"SUPPORT":     "دعم فني",
	}
	roleChart := make([]gin.H, 0, len(roleCounts))
	for _, rc := range roleCounts {
		label := roleLabels[rc.Role]
		if label == "" {
			label = rc.Role
		}
		roleChart = append(roleChart, gin.H{
			"name":  label,
			"value": rc.Count,
		})
	}

	// Users by country
	type countryCount struct {
		Country string
		Count   int64
	}
	var countryCounts []countryCount
	db.DB.Model(&models.User{}).
		Select("COALESCE(country, 'أخرى') as country, COUNT(*) as count").
		Where("deleted_at IS NULL").
		Group("country").
		Order("count DESC").
		Limit(5).
		Scan(&countryCounts)

	countryChart := make([]gin.H, 0, len(countryCounts))
	for _, cc := range countryCounts {
		if cc.Country == "" || cc.Country == "أخرى" {
			countryChart = append(countryChart, gin.H{"name": "أخرى", "users": cc.Count})
		} else {
			countryChart = append(countryChart, gin.H{"name": cc.Country, "users": cc.Count})
		}
	}

	// Login activity (last 7 days)
	type dailyActivity struct {
		Day   string
		Count int64
	}
	var loginActivity []dailyActivity
	sevenDaysAgo := now.AddDate(0, 0, -6)
	startOfSevenDays := time.Date(sevenDaysAgo.Year(), sevenDaysAgo.Month(), sevenDaysAgo.Day(), 0, 0, 0, 0, sevenDaysAgo.Location())

	db.DB.Model(&models.SecurityLog{}).
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as day, COUNT(*) as count").
		Where("created_at >= ? AND event_type IN ('LOGIN_SUCCESS','LOGIN_ATTEMPT')", startOfSevenDays).
		Group("TO_CHAR(created_at, 'YYYY-MM-DD')").
		Scan(&loginActivity)

	loginMap := make(map[string]int64)
	for _, d := range loginActivity {
		loginMap[d.Day] = d.Count
	}

	dayNames := []string{"السبت", "الأحد", "الاثنين", "الثلاثاء", "الأربعاء", "الخميس", "الجمعة"}
	loginChart := make([]gin.H, 0, 7)
	nowDay := int(now.Weekday())
	for i := 6; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		dayKey := d.Format("2006-01-02")
		dayIdx := (nowDay - i + 7) % 7
		loginChart = append(loginChart, gin.H{
			"name":   dayNames[dayIdx],
			"logins": loginMap[dayKey],
		})
	}

	// Registration trend (last 4 weeks)
	var weeklyRegistrations []dailyActivity
	fourWeeksAgo := now.AddDate(0, 0, -27)
	db.DB.Model(&models.User{}).
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as day, COUNT(*) as count").
		Where("created_at >= ? AND deleted_at IS NULL", fourWeeksAgo).
		Group("TO_CHAR(created_at, 'YYYY-MM-DD')").
		Scan(&weeklyRegistrations)

	regMap := make(map[string]int64)
	for _, d := range weeklyRegistrations {
		regMap[d.Day] = d.Count
	}

	registrationChart := make([]gin.H, 0, 4)
	for w := 0; w < 4; w++ {
		weekStart := now.AddDate(0, 0, -(3-w)*7-6)
		weekEnd := weekStart.AddDate(0, 0, 6)
		total := int64(0)
		for day, count := range regMap {
			t, _ := time.Parse("2006-01-02", day)
			if (t.Equal(weekStart) || t.After(weekStart)) && (t.Equal(weekEnd) || t.Before(weekEnd)) {
				total += count
			}
		}
		registrationChart = append(registrationChart, gin.H{
			"name":          fmt.Sprintf("الأسبوع %d", w+1),
			"registrations": total,
		})
	}

	api_response.Success(c, gin.H{
		"growth":        growthChart,
		"roles":         roleChart,
		"countries":     countryChart,
		"loginActivity": loginChart,
		"registrations": registrationChart,
	})
}

func AdminUsersFilterOptions(c *gin.Context) {
	// Fetch available teachers for filter
	var teachers []models.User
	db.DB.Model(&models.User{}).
		Select("id, COALESCE(name, username, email) as name").
		Where("deleted_at IS NULL AND (role = ? OR role = ?)", models.RoleTeacher, models.RoleAdmin).
		Limit(50).
		Find(&teachers)

	teacherOptions := make([]gin.H, 0, len(teachers))
	for _, t := range teachers {
		teacherOptions = append(teacherOptions, gin.H{
			"id":   t.ID,
			"name": t.GetName(),
		})
	}

	// Fetch available courses/subjects for filter
	var subjects []models.Subject
	db.DB.Model(&models.Subject{}).
		Select("id, name").
		Where("deleted_at IS NULL").
		Limit(50).
		Find(&subjects)

	courseOptions := make([]gin.H, 0, len(subjects))
	for _, s := range subjects {
		courseOptions = append(courseOptions, gin.H{
			"id":   s.ID,
			"name": s.Name,
		})
	}

	// Fetch categories
	type category struct {
		ID   string
		Name string
	}
	var categories []category
	db.DB.Model(&models.Category{}).
		Select("id, name").
		Where("deleted_at IS NULL").
		Limit(20).
		Find(&categories)

	categoryOptions := make([]gin.H, 0, len(categories))
	for _, cat := range categories {
		categoryOptions = append(categoryOptions, gin.H{
			"id":   cat.ID,
			"name": cat.Name,
		})
	}

	api_response.Success(c, gin.H{
		"teachers":   teacherOptions,
		"courses":    courseOptions,
		"categories": categoryOptions,
	})
}

// ─────────────────────────────────────────────
// Parent Management Handlers
// ─────────────────────────────────────────────

// GetParentStudents returns students linked to a parent
