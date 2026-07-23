package handlers

import (
	"net/http"
	"strconv"
	"time"

	api_response "thanawy-backend/internal/api/response"
	"thanawy-backend/internal/cache"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"
	"thanawy-backend/internal/services"

	"github.com/gin-gonic/gin"
)

// =============================================================
// Admin Workflow Queue Handlers
// =============================================================

// GetAdminReviewQueue returns the list of courses pending review
// GET /api/admin/courses/review-queue
func GetAdminReviewQueue(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	notificationSvc := services.NewWorkflowNotificationService()
	items, total, err := notificationSvc.GetReviewQueue(page, limit)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch review queue")
		return
	}

	stats, _ := notificationSvc.GetReviewStats()

	api_response.Success(c, gin.H{
		"queue": items,
		"stats": stats,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetReviewQueueStats returns statistics about the review queue
// GET /api/admin/courses/review-queue/stats
func GetReviewQueueStats(c *gin.Context) {
	notificationSvc := services.NewWorkflowNotificationService()
	stats, err := notificationSvc.GetReviewStats()
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch stats")
		return
	}

	api_response.Success(c, gin.H{"stats": stats})
}

// GetCourseReviewDetails returns detailed review information for a course
// GET /api/admin/courses/:id/review-details
func GetCourseReviewDetails(c *gin.Context) {
	subjectID := c.Param("id")

	var subject models.Subject
	if err := db.DB.
		Preload("Topics.SubTopics").
		Preload("Instructor").
		First(&subject, "id = ?", subjectID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Course not found")
		return
	}

	if subject.Status != models.CourseStatusUnderReview {
		api_response.Error(c, http.StatusBadRequest, "Course is not under review")
		return
	}

	// Get submission history
	var submissions []models.CourseReviewSubmission
	db.DB.Where("subject_id = ?", subjectID).
		Order("created_at DESC").
		Limit(5).
		Find(&submissions)

	// Calculate stats
	var topicsCount, lessonsCount, totalDuration int
	var quizzesCount int

	for _, topic := range subject.Topics {
		topicsCount++
		for _, lesson := range topic.SubTopics {
			lessonsCount++
			totalDuration += lesson.DurationMinutes
			if lesson.Type == models.SubTopicQuiz {
				quizzesCount++
			}
		}
	}

	// Get pricing
	var pricing models.CoursePricing
	db.DB.Where("subject_id = ?", subjectID).First(&pricing)

	// Get tags and categories
	type TagInfo struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	tags := []TagInfo{}
	if len(subject.Tags) > 0 {
		for _, t := range subject.Tags {
			tags = append(tags, TagInfo{Name: t.Name, Type: "tag"})
		}
	}

	categoryName := ""
	if subject.CategoryId != nil {
		var category models.Category
		if db.DB.First(&category, "id = ?", *subject.CategoryId).Error == nil {
			categoryName = category.Name
		}
	}

	// Get instructor info
	instructorName := stringOrEmpty(subject.InstructorName)
	instructorEmail := ""
	instructorBio := ""
	instructorAvatar := ""
	if subject.InstructorId != nil {
		var instructor models.User
		if db.DB.Select("id, name, email, bio, avatar").First(&instructor, "id = ?", *subject.InstructorId).Error == nil {
			instructorName = getUserName(&instructor)
			instructorEmail = instructor.Email
			instructorBio = stringOrEmpty(instructor.Bio)
			instructorAvatar = stringOrEmpty(instructor.Avatar)
		}
	}

	api_response.Success(c, gin.H{
		"course": gin.H{
			"id":                  subject.ID,
			"name":                subject.Name,
			"nameAr":              subject.NameAr,
			"description":         subject.Description,
			"thumbnail":           subject.ThumbnailUrl,
			"videoUrl":            subject.TrailerUrl,
			"category":            categoryName,
			"language":            subject.Language,
			"level":               subject.Level,
			"tags":                tags,
			"shortDescription":    subject.ShortDescription,
			"longDescription":     subject.LongDescription,
			"requirements":        subject.CoursePrerequisites,
			"whatYouLearn":        subject.WhatYouLearn,
			"targetAudience":      subject.TargetAudience,
			"certificateEnabled":  subject.HasCertificate,
			"coursePrerequisites": subject.CoursePrerequisites,
		},
		"instructor": gin.H{
			"id":     stringOrEmpty(subject.InstructorId),
			"name":   instructorName,
			"email":  instructorEmail,
			"bio":    instructorBio,
			"avatar": instructorAvatar,
		},
		"curriculum": gin.H{
			"topicsCount":   topicsCount,
			"lessonsCount":  lessonsCount,
			"quizzesCount":  quizzesCount,
			"totalDuration": totalDuration,
		},
		"pricing": gin.H{
			"type":     pricing.PricingType,
			"price":    pricing.Price,
			"currency": pricing.Currency,
			"discount": pricing.DiscountPrice,
		},
		"workflow": gin.H{
			"status":          subject.Status,
			"submittedAt":     formatTime(subject.SubmittedForReviewAt),
			"submittedBy":     stringOrEmpty(subject.InstructorId),
			"rejectionReason": subject.RejectionReason,
			"reviewedAt":      formatTime(subject.ReviewedAt),
			"reviewedBy":      stringOrEmpty(subject.ReviewedBy),
		},
		"submissions": submissions,
	})
}

// ApproveCourseWithReview is the admin approval handler that sends notifications
func ApproveCourseWithReview(c *gin.Context) {
	// First call the original handler
	ApproveCourse(c)

	// If successful, send notifications
	if c.Writer.Status() == http.StatusOK {
		subjectID := c.Param("id")

		var subject models.Subject
		if err := db.DB.First(&subject, "id = ?", subjectID).Error; err != nil {
			return
		}

		// Get instructor
		instructorName := stringOrEmpty(subject.InstructorName)
		instructorEmail := ""
		if subject.InstructorId != nil {
			var instructor models.User
			if db.DB.Select("id, name, email").First(&instructor, "id = ?", *subject.InstructorId).Error == nil {
				instructorName = getUserName(&instructor)
				instructorEmail = instructor.Email
			}
		}

		notificationSvc := services.NewWorkflowNotificationService()
		event := &services.WorkflowEvent{
			Event:           "approved",
			SubjectID:       subjectID,
			CourseName:      subject.Name,
			CourseNameAr:    stringOrEmpty(subject.NameAr),
			InstructorID:    stringOrEmpty(subject.InstructorId),
			InstructorName:  instructorName,
			InstructorEmail: instructorEmail,
		}

		notificationSvc.SendInAppNotification(event)
		notificationSvc.SendEmailNotification(event)
	}
}

// RejectCourseWithReview is the admin rejection handler that sends notifications
func RejectCourseWithReview(c *gin.Context) {
	subjectID := c.Param("id")

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Reason is required")
		return
	}

	userID, _ := c.Get("userId")
	uid, _ := userID.(string)

	var subject models.Subject
	if err := db.DB.First(&subject, "id = ?", subjectID).Error; err != nil {
		api_response.Error(c, http.StatusNotFound, "Course not found")
		return
	}

	if !subject.Status.CanTransitionTo(models.CourseStatusRejected) {
		api_response.Error(c, http.StatusBadRequest, "Cannot reject course from current status")
		return
	}

	now := time.Now()

	updates := map[string]interface{}{
		"status":           models.CourseStatusRejected,
		"reviewed_at":      now,
		"reviewed_by":      uid,
		"rejection_reason": req.Reason,
		"updated_at":       now,
	}

	if err := db.DB.Model(&subject).Updates(updates).Error; err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to reject course")
		return
	}

	db.DB.Model(&models.CourseReviewSubmission{}).
		Where("subject_id = ? AND status = ?", subjectID, "PENDING").
		Updates(map[string]interface{}{
			"status":         "REJECTED",
			"reviewer_id":    uid,
			"reviewer_notes": req.Reason,
			"reviewed_at":    now,
			"updated_at":     now,
		})

	recordChangeLog(c, subjectID, "REJECT", uid,
		map[string]interface{}{"from": string(subject.Status), "to": string(models.CourseStatusRejected), "reason": req.Reason},
		"Rejected: "+req.Reason)

	cache.NewCacheInvalidator().InvalidateSubject(c.Request.Context(), subjectID)
	LogAudit(c, "STATUS_CHANGE", "course", subjectID, gin.H{"action": "reject", "reason": req.Reason})

	// Get instructor for notifications
	instructorName := stringOrEmpty(subject.InstructorName)
	instructorEmail := ""
	if subject.InstructorId != nil {
		var instructor models.User
		if db.DB.Select("id, name, email").First(&instructor, "id = ?", *subject.InstructorId).Error == nil {
			instructorName = getUserName(&instructor)
			instructorEmail = instructor.Email
		}
	}

	// Send notifications
	notificationSvc := services.NewWorkflowNotificationService()
	event := &services.WorkflowEvent{
		Event:           "rejected",
		SubjectID:       subjectID,
		CourseName:      subject.Name,
		CourseNameAr:    stringOrEmpty(subject.NameAr),
		InstructorID:    stringOrEmpty(subject.InstructorId),
		InstructorName:  instructorName,
		InstructorEmail: instructorEmail,
		RejectionReason: req.Reason,
		ReviewerID:      uid,
		ReviewerName:    "",
	}

	notificationSvc.SendInAppNotification(event)
	notificationSvc.SendEmailNotification(event)

	api_response.Success(c, gin.H{
		"message": "Course rejected",
		"status":  models.CourseStatusRejected,
	})
}

// GetWorkflowStats returns workflow statistics for dashboard
// GET /api/admin/workflow/stats
func GetWorkflowStats(c *gin.Context) {
	notificationSvc := services.NewWorkflowNotificationService()
	stats, err := notificationSvc.GetReviewStats()
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch stats")
		return
	}

	api_response.Success(c, gin.H{"stats": stats})
}

// Helper functions
func getUserName(user *models.User) string {
	if user == nil {
		return ""
	}
	if user.Name != nil && *user.Name != "" {
		return *user.Name
	}
	if user.Email != "" {
		return user.Email
	}
	return ""
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}
