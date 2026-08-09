package admin

import (
	"net/http"
	"strconv"
	models "thanawy-backend/internal/domain/common"
	"time"

	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
)

func GetAdminReportsOverview(c *gin.Context) {
	var totalUsers int64
	var usersToday int64
	var usersWeek int64
	var usersMonth int64
	var totalSubjects int64
	var activeSubjects int64
	var totalNotifications int64
	var totalStudySessions int64
	var totalBooks int64
	var booksThisMonth int64
	var totalReviews int64
	var reviewsThisMonth int64

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
	for _, subject := range subjects {
		popularSubjects = append(popularSubjects, gin.H{
			"id":            subject.ID,
			"title":         firstNonEmpty(stringOrEmpty(subject.NameAr), subject.Name),
			"enrolledCount": subject.EnrolledCount,
			"isPublished":   subject.IsPublished,
		})
	}

	var books []models.Book
	db.DB.Order(createdAtDescSort).Limit(5).Find(&books)
	popularBooks := make([]gin.H, 0, len(books))
	for _, book := range books {
		popularBooks = append(popularBooks, gin.H{
			"id":     book.ID,
			"title":  book.Title,
			"author": book.Author,
			"isFree": book.IsFree,
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

func GetAdminReportsUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	var total int64
	db.DB.Model(&models.User{}).Count(&total)

	var users []models.User
	if err := db.DB.Order(createdAtDescSort).Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users report"})
		return
	}

	userIDs := make([]string, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}

	examCounts := countsByKey(&models.ExamResult{}, "user_id", userIDs)
	sessionCounts := countsByKey(&models.StudySession{}, "user_id", userIDs)

	items := make([]gin.H, 0, len(users))
	for _, user := range users {
		items = append(items, gin.H{
			"id":            user.ID,
			"name":          user.Name,
			"email":         user.Email,
			"username":      user.Username,
			"role":          user.Role,
			"status":        user.Status,
			"createdAt":     user.CreatedAt,
			"lastLogin":     user.LastLogin,
			"monthlyExams":  examCounts[user.ID],
			"studySessions": sessionCounts[user.ID],
		})
	}

	roleCounts := []gin.H{}
	for _, role := range []models.UserRole{models.RoleAdmin, models.RoleTeacher, models.RoleStudent} {
		var count int64
		db.DB.Model(&models.User{}).Where(queryRole, role).Count(&count)
		roleCounts = append(roleCounts, gin.H{"role": role, "count": count})
	}

	api_response.Success(c, gin.H{
		"items": items,
		"users": items,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalCount": total,
			"totalPages": calculateTotalPages(total, limit),
		},
		"stats": gin.H{
			"totalUsers": total,
			"byRole":     roleCounts,
		},
	})
}

func GetAdminReportsBooks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	var total int64
	db.DB.Model(&models.Book{}).Count(&total)

	var books []models.Book
	if err := db.DB.Order(createdAtDescSort).Offset(offset).Limit(limit).Find(&books).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch books report"})
		return
	}

	items := make([]gin.H, 0, len(books))
	for _, book := range books {
		items = append(items, gin.H{
			"id":          book.ID,
			"title":       book.Title,
			"author":      book.Author,
			"price":       book.Price,
			"isFree":      book.IsFree,
			"subjectId":   book.SubjectID,
			"createdAt":   book.CreatedAt,
			"downloadUrl": book.DownloadUrl,
		})
	}

	api_response.Success(c, gin.H{
		"items": items,
		"books": items,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalCount": total,
			"totalPages": calculateTotalPages(total, limit),
		},
		"stats": gin.H{
			"totalBooks": total,
		},
	})
}
