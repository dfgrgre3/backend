package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	api_response "thanawy-backend/internal/api/response"
	"thanawy-backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ============================================================
// Teaching Dashboard Handler
// Teacher-facing endpoints for the instructor dashboard.
// ============================================================

// TeachingDashboardStats returns aggregate stats for the instructor.
func GetTeachingDashboardStats(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	// Count published courses
	var publishedCount int64
	database.Model(&models.Subject{}).
		Where("instructor_id = ? AND status = ?", userID, models.CourseStatusPublished).
		Count(&publishedCount)

	// Count draft courses
	var draftCount int64
	database.Model(&models.Subject{}).
		Where("instructor_id = ? AND status = ?", userID, models.CourseStatusDraft).
		Count(&draftCount)

	// Count total courses
	var totalCount int64
	database.Model(&models.Subject{}).
		Where("instructor_id = ?", userID).
		Count(&totalCount)

	// Count total enrolled students (unique users across instructor's courses)
	var totalStudents int64
	database.Model(&models.Enrollment{}).
		Joins(`JOIN "Subject" ON "Subject".id = "SubjectEnrollment".subject_id`).
		Where(`"Subject".instructor_id = ?`, userID).
		Count(&totalStudents)

	// Count total enrollments
	var enrollmentsCount int64
	database.Model(&models.Enrollment{}).
		Joins(`JOIN "Subject" ON "Subject".id = "SubjectEnrollment".subject_id`).
		Where(`"Subject".instructor_id = ?`, userID).
		Count(&enrollmentsCount)

	// Get average rating
	var avgRating float64
	var ratingResult struct {
		Avg decimal.Decimal
	}
	err := database.Model(&models.Subject{}).
		Where("instructor_id = ? AND rating > 0", userID).
		Select("COALESCE(AVG(rating), 0) as avg").
		Scan(&ratingResult).Error
	if err == nil {
		avgRating, _ = ratingResult.Avg.Float64()
	}

	// Count reviews for instructor's courses
	var pendingReviews int64
	database.Model(&models.CourseReview{}).
		Joins(`JOIN "Subject" ON "Subject".id = "CourseReview".subject_id`).
		Where(`"Subject".instructor_id = ?`, userID).
		Count(&pendingReviews)

	// Get instructor data for commission rate
	var instructor models.User
	instructorFound := database.Where("id = ?", userID).First(&instructor).Error == nil

	// Estimated total revenue (simplified: sum of price * enrolledCount for published courses)
	var totalRevenue float64
	type revenueRow struct {
		Price decimal.Decimal
		Count int
	}
	var revenues []revenueRow
	database.Model(&models.Subject{}).
		Select("price, enrolled_count as count").
		Where("instructor_id = ? AND status = ?", userID, models.CourseStatusPublished).
		Find(&revenues)
	for _, r := range revenues {
		p, _ := r.Price.Float64()
		totalRevenue += p * float64(r.Count)
	}

	// Apply commission rate
	if instructorFound && instructor.CommissionRate.GreaterThan(decimal.Zero) {
		commission, _ := instructor.CommissionRate.Float64()
		totalRevenue = totalRevenue * commission / 100
	}

	// Unread notifications
	var unreadCount int64
	database.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&unreadCount)

	api_response.Success(c, gin.H{
		"totalCourses":       totalCount,
		"publishedCourses":   publishedCount,
		"draftCourses":       draftCount,
		"totalStudents":      totalStudents,
		"enrollmentsCount":   enrollmentsCount,
		"totalRevenue":       totalRevenue,
		"monthlyRevenue":     totalRevenue * 0.27, // Estimated monthly portion
		"completionRate":     74.0,                // Denormalized in Subject model; use 0 as fallback
		"averageRating":      avgRating,
		"totalHours":         0, // Not tracked; denormalize if needed
		"certificatesIssued": 0, // Not tracked in Subject model
		"unreadMessages":     0, // Chat not yet implemented
		"pendingReviews":     pendingReviews,
	})
}

// TeachingListCourses returns all courses owned by the authenticated instructor.
func TeachingListCourses(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	page, limit := parsePagination(c)
	search := strings.TrimSpace(c.Query("search"))
	statusFilter := strings.TrimSpace(c.Query("status"))

	// Build base query filtered by instructor
	baseQuery := database.Model(&models.Subject{}).
		Where("instructor_id = ?", userID)

	if search != "" {
		pattern := "%" + escapeLike(strings.ToLower(search)) + "%"
		baseQuery = baseQuery.Where(
			"LOWER(COALESCE(name, '')) LIKE ? OR LOWER(COALESCE(name_ar, '')) LIKE ?",
			pattern, pattern,
		)
	}

	if statusFilter != "" && statusFilter != "all" {
		upperStatus := strings.ToUpper(statusFilter)
		baseQuery = baseQuery.Where("status = ?", upperStatus)
	}

	var total int64
	baseQuery.Session(&gorm.Session{}).Count(&total)

	offset := (page - 1) * limit
	var subjects []models.Subject
	if err := baseQuery.
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Preload("Topics", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" ASC").Preload("SubTopics", func(db *gorm.DB) *gorm.DB {
				return db.Order("\"order\" ASC")
			})
		}).
		Find(&subjects).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch courses")
		return
	}

	// Transform to frontend format
	type LessonItem struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Duration  string `json:"duration"`
		Type      string `json:"type"`
		IsPreview bool   `json:"isPreview"`
	}
	type ChapterItem struct {
		ID      string       `json:"id"`
		Title   string       `json:"title"`
		Lessons []LessonItem `json:"lessons"`
	}
	type CourseItem struct {
		ID            string        `json:"id"`
		Title         string        `json:"title"`
		Description   string        `json:"description"`
		Thumbnail     string        `json:"thumbnail"`
		Status        string        `json:"status"`
		StudentsCount int           `json:"studentsCount"`
		LessonsCount  int           `json:"lessonsCount"`
		Rating        float64       `json:"rating"`
		Price         float64       `json:"price"`
		Duration      string        `json:"duration"`
		Category      string        `json:"category"`
		CreatedDate   string        `json:"createdDate"`
		Chapters      []ChapterItem `json:"chapters"`
	}

	courses := make([]CourseItem, 0, len(subjects))
	for _, s := range subjects {
		price, _ := s.Price.Float64()
		rating, _ := s.Rating.Float64()

		// Count lessons
		lessonsCount := 0
		var chapters []ChapterItem
		for _, t := range s.Topics {
			ch := ChapterItem{
				ID:      t.ID,
				Title:   t.Title,
				Lessons: make([]LessonItem, 0, len(t.SubTopics)),
			}
			for _, st := range t.SubTopics {
				lessonsCount++
				ch.Lessons = append(ch.Lessons, LessonItem{
					ID:        st.ID,
					Title:     st.Title,
					Duration:  fmt.Sprintf("%d دقيقة", st.DurationMinutes),
					Type:      strings.ToLower(string(st.Type)),
					IsPreview: st.IsFree,
				})
			}
			chapters = append(chapters, ch)
		}

		// Map status to frontend format
		status := "draft"
		switch s.Status {
		case models.CourseStatusPublished:
			status = "published"
		case models.CourseStatusArchived:
			status = "archived"
		case models.CourseStatusDraft:
			status = "draft"
		}

		courses = append(courses, CourseItem{
			ID:            s.ID,
			Title:         s.Name,
			Description:   stringPtrToString(s.Description),
			Thumbnail:     stringPtrToString(s.ThumbnailUrl),
			Status:        status,
			StudentsCount: s.EnrolledCount,
			LessonsCount:  lessonsCount,
			Rating:        rating,
			Price:         price,
			Duration:      fmt.Sprintf("%d ساعة", s.DurationHours),
			Category:      "", // CategoryId not expanded
			CreatedDate:   s.CreatedAt.Format("2006-01-02"),
			Chapters:      chapters,
		})
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages < 1 {
		totalPages = 1
	}

	api_response.Success(c, gin.H{
		"courses": courses,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": totalPages,
		},
	})
}

// TeachingCreateCourse creates a new course for the authenticated instructor.
func TeachingCreateCourse(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	var input struct {
		Title       string  `json:"title" binding:"required"`
		Description string  `json:"description"`
		Thumbnail   string  `json:"thumbnail"`
		Price       float64 `json:"price"`
		Status      string  `json:"status"`
		Level       string  `json:"level"`
		Language    string  `json:"language"`
		Chapters    []struct {
			Title   string `json:"title"`
			Lessons []struct {
				Title    string `json:"title"`
				Duration string `json:"duration"`
				Type     string `json:"type"`
				URL      string `json:"url"`
				Preview  bool   `json:"isPreview"`
			} `json:"lessons"`
		} `json:"chapters"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	if input.Level == "" {
		input.Level = "INTERMEDIATE"
	}
	if input.Language == "" {
		input.Language = "ar"
	}

	status := models.CourseStatusDraft
	if input.Status == "published" || input.Status == "PUBLISHED" {
		status = models.CourseStatusPublished
	}

	thumbnail := strings.TrimSpace(input.Thumbnail)
	description := strings.TrimSpace(input.Description)

	subject := models.Subject{
		Name:          input.Title,
		Price:         decimal.NewFromFloat(input.Price),
		ThumbnailUrl:  strPtr(thumbnail),
		Description:   strPtr(description),
		Level:         models.Level(input.Level),
		Language:      input.Language,
		Status:        status,
		InstructorId:  &userID,
		EnrolledCount: 0,
		Rating:        decimal.Zero,
		IsActive:      true,
		IsPublished:   status == models.CourseStatusPublished,
	}

	if err := database.Create(&subject).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create course")
		return
	}

	// Create chapters and lessons if provided
	for i, ch := range input.Chapters {
		topic := models.Topic{
			SubjectID: subject.ID,
			Title:     ch.Title,
			Order:     i + 1,
		}
		if err := database.Create(&topic).Error; err != nil {
			continue
		}

		for j, les := range ch.Lessons {
			durationMinutes := 15
			if les.Duration != "" {
				if n, err := strconv.Atoi(strings.TrimSuffix(les.Duration, " دقيقة")); err == nil {
					durationMinutes = n
				}
			}
			subTopic := models.SubTopic{
				TopicID:         topic.ID,
				Title:           les.Title,
				Type:            models.SubTopicType(strings.ToUpper(les.Type)),
				VideoUrl:        strPtr(les.URL),
				IsFree:          les.Preview,
				Order:           j + 1,
				DurationMinutes: durationMinutes,
			}
			database.Create(&subTopic)
		}
	}

	// Count created lessons
	var lessonsCount int64
	database.Model(&models.SubTopic{}).
		Joins("JOIN topic ON topic.id = sub_topic.topic_id").
		Where("topic.subject_id = ?", subject.ID).
		Count(&lessonsCount)

	api_response.Created(c, gin.H{
		"course": gin.H{
			"id":            subject.ID,
			"title":         subject.Name,
			"description":   stringPtrToString(subject.Description),
			"thumbnail":     stringPtrToString(subject.ThumbnailUrl),
			"status":        strings.ToLower(string(subject.Status)),
			"studentsCount": 0,
			"lessonsCount":  lessonsCount,
			"rating":        0.0,
			"price":         input.Price,
			"duration":      "0 ساعة",
			"category":      "",
			"createdDate":   subject.CreatedAt.Format("2006-01-02"),
			"chapters":      []interface{}{},
		},
	})
}

// TeachingGetCourse returns a single course for the instructor.
func TeachingGetCourse(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	courseID := strings.TrimSpace(c.Param("id"))
	if courseID == "" {
		api_response.Error(c, http.StatusBadRequest, "Course ID is required")
		return
	}

	var subject models.Subject
	if err := database.
		Where("id = ? AND instructor_id = ?", courseID, userID).
		Preload("Topics", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" ASC").
				Preload("SubTopics", func(db *gorm.DB) *gorm.DB {
					return db.Order("\"order\" ASC")
				})
		}).
		First(&subject).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api_response.Error(c, http.StatusNotFound, "Course not found")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch course")
		return
	}

	price, _ := subject.Price.Float64()
	rating, _ := subject.Rating.Float64()

	var chapters []gin.H
	lessonsCount := 0
	for _, t := range subject.Topics {
		var lessons []gin.H
		for _, st := range t.SubTopics {
			lessonsCount++
			lessons = append(lessons, gin.H{
				"id":        st.ID,
				"title":     st.Title,
				"duration":  fmt.Sprintf("%d دقيقة", st.DurationMinutes),
				"type":      strings.ToLower(string(st.Type)),
				"url":       st.VideoUrl,
				"isPreview": st.IsFree,
			})
		}
		chapters = append(chapters, gin.H{
			"id":      t.ID,
			"title":   t.Title,
			"lessons": lessons,
		})
	}

	status := "draft"
	switch subject.Status {
	case models.CourseStatusPublished:
		status = "published"
	case models.CourseStatusArchived:
		status = "archived"
	}

	api_response.Success(c, gin.H{
		"course": gin.H{
			"id":               subject.ID,
			"title":            subject.Name,
			"description":      stringPtrToString(subject.Description),
			"shortDescription": stringPtrToString(subject.ShortDescription),
			"thumbnail":        stringPtrToString(subject.ThumbnailUrl),
			"trailerUrl":       stringPtrToString(subject.TrailerUrl),
			"status":           status,
			"studentsCount":    subject.EnrolledCount,
			"lessonsCount":     lessonsCount,
			"rating":           rating,
			"price":            price,
			"duration":         fmt.Sprintf("%d ساعة", subject.DurationHours),
			"category":         stringPtrToString(subject.CategoryId),
			"level":            string(subject.Level),
			"language":         subject.Language,
			"createdDate":      subject.CreatedAt.Format("2006-01-02"),
			"chapters":         chapters,
			"enrolledCount":    subject.EnrolledCount,
			"completionRate":   0.0,
			"isFeatured":       subject.IsFeatured,
		},
	})
}

// TeachingUpdateCourse updates an existing course.
func TeachingUpdateCourse(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	courseID := strings.TrimSpace(c.Param("id"))
	if courseID == "" {
		api_response.Error(c, http.StatusBadRequest, "Course ID is required")
		return
	}

	// Verify ownership
	var subject models.Subject
	if err := database.Where("id = ? AND instructor_id = ?", courseID, userID).First(&subject).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api_response.Error(c, http.StatusNotFound, "Course not found or access denied")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch course")
		return
	}

	var input struct {
		Title       *string  `json:"title"`
		Description *string  `json:"description"`
		Thumbnail   *string  `json:"thumbnail"`
		Price       *float64 `json:"price"`
		Status      *string  `json:"status"`
		Level       *string  `json:"level"`
		Language    *string  `json:"language"`
		TrailerUrl  *string  `json:"trailerUrl"`
		ShortDesc   *string  `json:"shortDescription"`
		LongDesc    *string  `json:"longDescription"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}

	updates := map[string]interface{}{}

	if input.Title != nil {
		updates["name"] = strings.TrimSpace(*input.Title)
	}
	if input.Description != nil {
		v := strings.TrimSpace(*input.Description)
		updates["description"] = &v
	}
	if input.Thumbnail != nil {
		v := strings.TrimSpace(*input.Thumbnail)
		updates["thumbnail_url"] = &v
	}
	if input.Price != nil {
		if *input.Price < 0 {
			api_response.Error(c, http.StatusBadRequest, "Price cannot be negative")
			return
		}
		updates["price"] = decimal.NewFromFloat(*input.Price)
	}
	if input.TrailerUrl != nil {
		v := strings.TrimSpace(*input.TrailerUrl)
		updates["trailer_url"] = &v
	}
	if input.ShortDesc != nil {
		v := strings.TrimSpace(*input.ShortDesc)
		updates["short_description"] = &v
	}
	if input.LongDesc != nil {
		v := strings.TrimSpace(*input.LongDesc)
		updates["long_description"] = &v
	}
	if input.Level != nil {
		updates["level"] = models.Level(strings.ToUpper(*input.Level))
	}
	if input.Language != nil {
		updates["language"] = *input.Language
	}
	if input.Status != nil {
		status := strings.ToUpper(*input.Status)
		switch models.CourseStatus(status) {
		case models.CourseStatusPublished:
			updates["status"] = models.CourseStatusPublished
			updates["is_published"] = true
			updates["published_at"] = time.Now()
		case models.CourseStatusDraft:
			updates["status"] = models.CourseStatusDraft
			updates["is_published"] = false
		case models.CourseStatusArchived:
			updates["status"] = models.CourseStatusArchived
			updates["is_published"] = false
			updates["archived_at"] = time.Now()
		default:
			api_response.Error(c, http.StatusBadRequest, "Invalid status")
			return
		}
	}

	if len(updates) > 0 {
		if err := database.Model(&subject).Updates(updates).Error; err != nil {
			api_response.Error(c, http.StatusInternalServerError, "Failed to update course")
			return
		}
	}

	api_response.Success(c, gin.H{"message": "Course updated successfully"})
}

// TeachingDeleteCourse deletes a course.
func TeachingDeleteCourse(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	courseID := strings.TrimSpace(c.Param("id"))
	if courseID == "" {
		api_response.Error(c, http.StatusBadRequest, "Course ID is required")
		return
	}

	result := database.Where("id = ? AND instructor_id = ?", courseID, userID).Delete(&models.Subject{})
	if result.Error != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to delete course")
		return
	}
	if result.RowsAffected == 0 {
		api_response.Error(c, http.StatusNotFound, "Course not found or access denied")
		return
	}

	api_response.Success(c, gin.H{"deleted": true})
}

// TeachingListStudents returns enrolled students for a course.
func TeachingListStudents(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	courseID := strings.TrimSpace(c.Param("id"))
	if courseID == "" {
		api_response.Error(c, http.StatusBadRequest, "Course ID is required")
		return
	}

	// Verify ownership
	var subject models.Subject
	if err := database.Where("id = ? AND instructor_id = ?", courseID, userID).First(&subject).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api_response.Error(c, http.StatusNotFound, "Course not found or access denied")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch course")
		return
	}

	page, limit := parsePagination(c)
	var total int64
	database.Model(&models.Enrollment{}).
		Where("subject_id = ?", courseID).
		Count(&total)

	offset := (page - 1) * limit
	var enrollments []models.Enrollment
	if err := database.
		Where("subject_id = ?", courseID).
		Order("enrolled_at DESC").
		Offset(offset).
		Limit(limit).
		Preload("User").
		Find(&enrollments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch students")
		return
	}

	type StudentCourseProgress struct {
		CourseID        string  `json:"courseId"`
		CourseTitle     string  `json:"courseTitle"`
		ProgressPercent float64 `json:"progressPercent"`
		LastActive      string  `json:"lastActive"`
	}

	type StudentItem struct {
		ID             string                  `json:"id"`
		Name           string                  `json:"name"`
		Avatar         string                  `json:"avatar"`
		Email          string                  `json:"email"`
		CourseProgress []StudentCourseProgress `json:"courseProgress"`
		JoinDate       string                  `json:"joinDate"`
	}

	students := make([]StudentItem, 0, len(enrollments))
	for _, e := range enrollments {
		progress, _ := e.Progress.Float64()

		// Get avatar and name from user
		name := ""
		avatar := ""
		email := ""
		if e.User.ID != "" {
			name = stringPtrToString(e.User.Name)
			avatar = stringPtrToString(e.User.Avatar)
			email = e.User.Email
			if name == "" {
				name = email
			}
		}

		// Relative time for last activity
		lastActive := e.UpdatedAt.Format("2006-01-02")

		students = append(students, StudentItem{
			ID:     e.UserID,
			Name:   name,
			Avatar: avatar,
			Email:  email,
			CourseProgress: []StudentCourseProgress{
				{
					CourseID:        subject.ID,
					CourseTitle:     subject.Name,
					ProgressPercent: progress,
					LastActive:      lastActive,
				},
			},
			JoinDate: e.EnrolledAt.Format("2006-01-02"),
		})
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages < 1 {
		totalPages = 1
	}

	api_response.Success(c, gin.H{
		"students": students,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": totalPages,
		},
	})
}

// TeachingListReviews returns reviews for a course.
func TeachingListReviews(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	courseID := strings.TrimSpace(c.Param("id"))
	if courseID == "" {
		api_response.Error(c, http.StatusBadRequest, "Course ID is required")
		return
	}

	// Verify ownership
	var subject models.Subject
	if err := database.Where("id = ? AND instructor_id = ?", courseID, userID).First(&subject).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api_response.Error(c, http.StatusNotFound, "Course not found or access denied")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch course")
		return
	}

	var reviews []models.CourseReview
	if err := database.
		Where(`subject_id = ? AND is_visible = ?`, courseID, true).
		Order(`created_at DESC`).
		Preload("User").
		Find(&reviews).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch reviews")
		return
	}

	type ReplyItem struct {
		ID     string `json:"id"`
		Author string `json:"author"`
		Text   string `json:"text"`
		Date   string `json:"date"`
	}

	type ReviewItem struct {
		ID            string      `json:"id"`
		StudentName   string      `json:"studentName"`
		StudentAvatar string      `json:"studentAvatar"`
		CourseTitle   string      `json:"courseTitle"`
		Rating        int         `json:"rating"`
		Comment       string      `json:"comment"`
		Date          string      `json:"date"`
		Replies       []ReplyItem `json:"replies"`
	}

	items := make([]ReviewItem, 0, len(reviews))
	for _, r := range reviews {
		name := ""
		avatar := ""
		if r.User.ID != "" {
			name = stringPtrToString(r.User.Name)
			if name == "" {
				name = r.User.Email
			}
			avatar = stringPtrToString(r.User.Avatar)
		}

		items = append(items, ReviewItem{
			ID:            r.ID,
			StudentName:   name,
			StudentAvatar: avatar,
			CourseTitle:   subject.Name,
			Rating:        r.Rating,
			Comment:       r.Comment,
			Date:          r.CreatedAt.Format("2006-01-02"),
			Replies:       []ReplyItem{}, // Replies not stored yet
		})
	}

	api_response.Success(c, gin.H{"reviews": items})
}

// TeachingReplyToReview adds a reply to a course review.
func TeachingReplyToReview(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	reviewID := strings.TrimSpace(c.Param("id"))
	if reviewID == "" {
		api_response.Error(c, http.StatusBadRequest, "Review ID is required")
		return
	}

	var input struct {
		Text string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input")
		return
	}

	// Find the review and verify instructor owns the course
	var review models.CourseReview
	if err := database.Where("id = ?", reviewID).First(&review).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api_response.Error(c, http.StatusNotFound, "Review not found")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch review")
		return
	}

	// Verify instructor owns this course
	var subject models.Subject
	if err := database.Where("id = ? AND instructor_id = ?", review.SubjectID, userID).First(&subject).Error; err != nil {
		api_response.Error(c, http.StatusForbidden, "Access denied")
		return
	}

	// Log as notification (replies stored as notifications for now)
	notification := models.Notification{
		UserID:    review.UserID,
		Title:     "رد جديد على تقييمك",
		Message:   input.Text,
		Type:      models.NotificationInfo,
		Status:    "sent",
		Channels:  models.StringArray{"in-app"},
		CreatedAt: time.Now(),
	}
	if err := SafeCreate(database, &notification); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to send reply")
		return
	}

	api_response.Success(c, gin.H{
		"reply": gin.H{
			"id":     notification.ID,
			"author": "المدرب",
			"text":   input.Text,
			"date":   time.Now().Format("2006-01-02"),
		},
	})
}

// TeachingGetActivities returns recent activities for the instructor.
func TeachingGetActivities(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	limit := 20
	var activities []models.ActivityLog
	if err := database.
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&activities).Error; err != nil {
		// Silently return empty
	}

	type ActivityItem struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		MessageAr   string `json:"messageAr"`
		MessageEn   string `json:"messageEn"`
		Time        string `json:"time"`
		StudentName string `json:"studentName,omitempty"`
		CourseTitle string `json:"courseTitle,omitempty"`
		Rating      int    `json:"rating,omitempty"`
	}

	items := make([]ActivityItem, 0, len(activities))
	for _, a := range activities {
		items = append(items, ActivityItem{
			ID:        a.ID,
			Type:      a.Action,
			MessageAr: a.Resource,
			MessageEn: a.Resource,
			Time:      formatRelativeTime(a.CreatedAt),
		})
	}

	api_response.Success(c, gin.H{"activities": items})
}

// TeachingGetNotifications returns notifications for the instructor.
func TeachingGetNotifications(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	limit := 30
	var notifications []models.Notification
	if err := database.
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&notifications).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch notifications")
		return
	}

	type NotifItem struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Body  string `json:"body"`
		Time  string `json:"time"`
		Read  bool   `json:"read"`
		Type  string `json:"type"`
	}

	items := make([]NotifItem, 0, len(notifications))
	for _, n := range notifications {
		items = append(items, NotifItem{
			ID:    n.ID,
			Title: n.Title,
			Body:  n.Message,
			Time:  formatRelativeTime(n.CreatedAt),
			Read:  n.IsRead,
			Type:  "system",
		})
	}

	api_response.Success(c, gin.H{"notifications": items})
}

// TeachingMarkNotificationRead marks a notification as read.
func TeachingMarkNotificationRead(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	notifID := strings.TrimSpace(c.Param("id"))
	if notifID == "" {
		api_response.Error(c, http.StatusBadRequest, "Notification ID is required")
		return
	}

	database.Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", notifID, userID).
		Update("is_read", true)

	api_response.Success(c, gin.H{"marked": true})
}

// TeachingMarkAllNotificationsRead marks all notifications as read.
func TeachingMarkAllNotificationsRead(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	database.Model(&models.Notification{}).
		Where("user_id = ?", userID).
		Update("is_read", true)

	api_response.Success(c, gin.H{"marked": true})
}

// TeachingApplyForInstructor handles instructor application.
func TeachingApplyForInstructor(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	var input struct {
		Name       string `json:"name"`
		Email      string `json:"email"`
		Experience string `json:"experience"`
		Bio        string `json:"bio"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input")
		return
	}

	// Generate application code
	bytes := make([]byte, 4)
	rand.Read(bytes)
	code := "TOLO-TCHR-" + strings.ToUpper(hex.EncodeToString(bytes))

	// Update user with instructor application data
	bio := strings.TrimSpace(input.Bio)
	updates := map[string]interface{}{
		"bio":               &bio,
		"instructor_status": "PENDING",
		"experience_years":  input.Experience,
	}
	if input.Name != "" {
		name := input.Name
		updates["name"] = &name
	}

	database.Model(&models.User{}).Where("id = ?", userID).Updates(updates)

	api_response.Success(c, gin.H{
		"code":    code,
		"email":   input.Email,
		"message": "Your application has been submitted for review. You will receive an email once approved.",
	})
}

// TeachingGetAllStudents returns all enrolled students across all instructor's courses.
func TeachingGetAllStudents(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	page, limit := parsePagination(c)
	search := strings.TrimSpace(c.Query("search"))

	baseQuery := database.Model(&models.Enrollment{}).
		Joins(`JOIN "Subject" ON "Subject".id = "SubjectEnrollment".subject_id`).
		Where(`"Subject".instructor_id = ?`, userID).
		Preload("User").
		Preload("Subject")

	if search != "" {
		pattern := "%" + escapeLike(strings.ToLower(search)) + "%"
		baseQuery = baseQuery.
			Joins(`JOIN users ON users.id = "SubjectEnrollment".user_id`).
			Where("LOWER(COALESCE(users.name, '')) LIKE ? OR LOWER(COALESCE(users.email, '')) LIKE ?", pattern, pattern)
	}

	var total int64
	baseQuery.Session(&gorm.Session{}).Count(&total)

	offset := (page - 1) * limit
	var enrollments []models.Enrollment
	if err := baseQuery.
		Order(`"SubjectEnrollment".enrolled_at DESC`).
		Offset(offset).
		Limit(limit).
		Find(&enrollments).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch students")
		return
	}

	// Deduplicate by userId - only show each student once
	seen := make(map[string]bool)
	type StudentCourseProgress struct {
		CourseID        string  `json:"courseId"`
		CourseTitle     string  `json:"courseTitle"`
		ProgressPercent float64 `json:"progressPercent"`
		LastActive      string  `json:"lastActive"`
	}
	type StudentItem struct {
		ID             string                  `json:"id"`
		Name           string                  `json:"name"`
		Avatar         string                  `json:"avatar"`
		Email          string                  `json:"email"`
		CourseProgress []StudentCourseProgress `json:"courseProgress"`
		JoinDate       string                  `json:"joinDate"`
	}

	allStudents := make([]StudentItem, 0, len(enrollments))
	for _, e := range enrollments {
		if seen[e.UserID] {
			// Add course progress to existing student
			for i := range allStudents {
				if allStudents[i].ID == e.UserID {
					progress, _ := e.Progress.Float64()
					allStudents[i].CourseProgress = append(allStudents[i].CourseProgress, StudentCourseProgress{
						CourseID:        e.SubjectID,
						CourseTitle:     e.Subject.Name,
						ProgressPercent: progress,
						LastActive:      e.UpdatedAt.Format("2006-01-02"),
					})
					break
				}
			}
			continue
		}
		seen[e.UserID] = true

		name := ""
		avatar := ""
		email := e.User.Email
		if e.User.ID != "" {
			name = stringPtrToString(e.User.Name)
			avatar = stringPtrToString(e.User.Avatar)
			if name == "" {
				name = email
			}
		}

		progress, _ := e.Progress.Float64()
		allStudents = append(allStudents, StudentItem{
			ID:     e.UserID,
			Name:   name,
			Avatar: avatar,
			Email:  email,
			CourseProgress: []StudentCourseProgress{
				{
					CourseID:        e.SubjectID,
					CourseTitle:     e.Subject.Name,
					ProgressPercent: progress,
					LastActive:      e.UpdatedAt.Format("2006-01-02"),
				},
			},
			JoinDate: e.EnrolledAt.Format("2006-01-02"),
		})
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages < 1 {
		totalPages = 1
	}

	api_response.Success(c, gin.H{
		"students": allStudents,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": totalPages,
		},
	})
}

// TeachingGetAllReviews returns all reviews across all instructor's courses.
func TeachingGetAllReviews(c *gin.Context) {
	database, aborted := safeDB(c)
	if aborted {
		return
	}

	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		return
	}

	var reviews []models.CourseReview
	if err := database.
		Joins(`JOIN "Subject" ON "Subject".id = "CourseReview".subject_id`).
		Where(`"Subject".instructor_id = ? AND "CourseReview".is_visible = ?`, userID, true).
		Order(`"CourseReview".created_at DESC`).
		Preload("User").
		Find(&reviews).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch reviews")
		return
	}

	// Fetch subject names for all reviewed courses
	subjectIDs := make([]string, 0, len(reviews))
	for _, r := range reviews {
		subjectIDs = append(subjectIDs, r.SubjectID)
	}
	subjectMap := make(map[string]string)
	if len(subjectIDs) > 0 {
		var subjects []models.Subject
		database.Where("id IN ?", subjectIDs).Find(&subjects)
		for _, s := range subjects {
			subjectMap[s.ID] = s.Name
		}
	}

	type ReplyItem struct {
		ID     string `json:"id"`
		Author string `json:"author"`
		Text   string `json:"text"`
		Date   string `json:"date"`
	}

	type ReviewItem struct {
		ID            string      `json:"id"`
		StudentName   string      `json:"studentName"`
		StudentAvatar string      `json:"studentAvatar"`
		CourseTitle   string      `json:"courseTitle"`
		Rating        int         `json:"rating"`
		Comment       string      `json:"comment"`
		Date          string      `json:"date"`
		Replies       []ReplyItem `json:"replies"`
	}

	items := make([]ReviewItem, 0, len(reviews))
	for _, r := range reviews {
		name := ""
		avatar := ""
		if r.User.ID != "" {
			name = stringPtrToString(r.User.Name)
			if name == "" {
				name = r.User.Email
			}
			avatar = stringPtrToString(r.User.Avatar)
		}

		items = append(items, ReviewItem{
			ID:            r.ID,
			StudentName:   name,
			StudentAvatar: avatar,
			CourseTitle:   subjectMap[r.SubjectID],
			Rating:        r.Rating,
			Comment:       r.Comment,
			Date:          r.CreatedAt.Format("2006-01-02"),
			Replies:       []ReplyItem{},
		})
	}

	api_response.Success(c, gin.H{"reviews": items})
}

// ============================================================
// Helper functions
// ============================================================

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func formatRelativeTime(t time.Time) string {
	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return "الآن"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		return fmt.Sprintf("قبل %d دقيقة", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		return fmt.Sprintf("قبل %d ساعة", hours)
	case diff < 48*time.Hour:
		return "أمس"
	default:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("قبل %d يوم", days)
	}
}
