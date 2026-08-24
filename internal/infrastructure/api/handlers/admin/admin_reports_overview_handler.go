package admin

import (
	"net/http"
	"time"
	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func GetAdminReportsOverview(c *gin.Context) {
	var (
		totalUsers, usersToday, usersWeek, usersMonth int64
		totalSubjects, activeSubjects                 int64
		totalNotifications, totalStudySessions         int64
		totalBooks, booksThisMonth                    int64
		totalReviews, reviewsThisMonth                int64
	)

	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)
	weekAgo := now.AddDate(0, 0, -7)
	monthAgo := now.AddDate(0, -1, 0)

	db.DB.Model(&models.User{}).Count(&totalUsers)
	db.DB.Model(&models.User{}).Where(createdAtGte, dayAgo).Count(&usersToday)
	db.DB.Model(&models.User{}).Where(createdAtGte, weekAgo).Count(&usersWeek)
	db.DB.Model(&models.User{}).Where(createdAtGte, monthAgo).Count(&usersMonth)
	db.DB.Model(&models.Subject{}).Count(&totalSubjects)
	db.DB.Model(&models.Subject{}).Where(isActiveQuery, true).Count(&activeSubjects)
	db.DB.Model(&models.Notification{}).Count(&totalNotifications)
	db.DB.Model(&models.StudySession{}).Count(&totalStudySessions)
	db.DB.Model(&models.Book{}).Count(&totalBooks)
	db.DB.Model(&models.Book{}).Where(createdAtGte, monthAgo).Count(&booksThisMonth)
	db.DB.Model(&models.CourseReview{}).Count(&totalReviews)
	db.DB.Model(&models.CourseReview{}).Where(createdAtGte, monthAgo).Count(&reviewsThisMonth)

	var subjects []models.Subject
	db.DB.Order("enrolled_count desc").Limit(5).Find(&subjects)
	popularSubjects := make([]gin.H, 0, len(subjects))
	for _, s := range subjects {
		popularSubjects = append(popularSubjects, gin.H{
			"id":            s.ID,
			"title":         firstNonEmpty(stringOrEmpty(s.NameAr), s.Name),
			"enrolledCount": s.EnrolledCount,
			"isPublished":   s.IsPublished,
		})
	}

	var books []models.Book
	db.DB.Order(createdAtDescSort).Limit(5).Find(&books)
	popularBooks := make([]gin.H, 0, len(books))
	for _, b := range books {
		popularBooks = append(popularBooks, gin.H{
			"id":     b.ID,
			"title":  b.Title,
			"author": b.Author,
			"isFree": b.IsFree,
		})
	}

	type trendPoint struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}
	registrationTrend := make([]trendPoint, 0, 7)
	for i := 6; i >= 0; i-- {
		start := time.Date(now.Year(), now.Month(), now.Day()-i, 0, 0, 0, 0, now.Location())
		end := start.Add(24 * time.Hour)
		var count int64
		db.DB.Model(&models.User{}).Where(createdAtRangeQuery, start, end).Count(&count)
		registrationTrend = append(registrationTrend, trendPoint{Date: start.Format(dateFormat), Count: count})
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"users": gin.H{
				"total":        totalUsers,
				"newToday":     usersToday,
				"newThisWeek":  usersWeek,
				"newThisMonth": usersMonth,
			},
			"books": gin.H{
				"total":        totalBooks,
				"newThisMonth": booksThisMonth,
			},
			"subjects": gin.H{
				"total":  totalSubjects,
				"active": activeSubjects,
			},
			"engagement": gin.H{
				"totalReviews":     totalReviews,
				"reviewsThisMonth": reviewsThisMonth,
				"activeSessions":   totalStudySessions,
			},
			"popularBooks":    popularBooks,
			"popularSubjects": popularSubjects,
			"trends": gin.H{
				"userRegistrations": registrationTrend,
			},
		},
	})
}
