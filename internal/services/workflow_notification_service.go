package services

import (
	"log"
	"os"

	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"

	"github.com/google/uuid"
)

// WorkflowNotificationService handles notifications for course workflow events
type WorkflowNotificationService struct {
	notificationSvc *NotificationService
	emailSvc        *EmailService
}

// NewWorkflowNotificationService creates a new workflow notification service
func NewWorkflowNotificationService() *WorkflowNotificationService {
	return &WorkflowNotificationService{
		notificationSvc: GetNotificationService(),
		emailSvc:        GetEmailService(),
	}
}

// WorkflowEvent represents a workflow event
type WorkflowEvent struct {
	Event           string // submitted, approved, rejected, archived, unarchived
	SubjectID       string
	CourseName      string
	CourseNameAr    string
	InstructorID    string
	InstructorName  string
	InstructorEmail string
	ReviewerID      string
	ReviewerName    string
	RejectionReason string
	Extra           string
}

// SendInAppNotification creates an in-app notification
func (s *WorkflowNotificationService) SendInAppNotification(event *WorkflowEvent) error {
	notification := models.Notification{
		ID:      uuid.New().String(),
		UserID:  event.InstructorID,
		Type:    "workflow",
		Title:   WorkflowNotificationTitle(event.Event, event.CourseName),
		Message: WorkflowNotificationBody(event.Event, event.CourseName, event.Extra),
		Status:  "pending",
	}

	if err := s.notificationSvc.QueueNotification(notification); err != nil {
		log.Printf("[WorkflowNotification] Failed to queue in-app notification: %v", err)
		return err
	}

	return nil
}

// SendEmailNotification sends an email notification
func (s *WorkflowNotificationService) SendEmailNotification(event *WorkflowEvent) error {
	if event.InstructorEmail == "" {
		return nil
	}

	var subject, body string
	baseURL := os.Getenv("FRONTEND_URL")
	if baseURL == "" {
		baseURL = "https://tolo-academy.com"
	}

	switch event.Event {
	case "submitted":
		subject = "طلب مراجعة كورس جديد"
		body = GetCourseSubmittedForReviewEmailTemplate(
			event.InstructorName,
			event.CourseName,
			baseURL+"/admin/courses/"+event.SubjectID+"/review",
		)
	case "approved":
		subject = "تمت الموافقة على كورسك! 🎉"
		body = GetCourseApprovedEmailTemplate(
			event.InstructorName,
			event.CourseName,
			baseURL+"/courses/"+event.SubjectID,
		)
	case "rejected":
		subject = "تم طلب تعديلات على كورسك"
		body = GetCourseRejectedEmailTemplate(
			event.InstructorName,
			event.CourseName,
			event.RejectionReason,
			baseURL+"/instructor/courses/"+event.SubjectID+"/edit",
		)
	case "archived":
		subject = "تم أرشفة الكورس"
		body = GetCourseArchivedEmailTemplate(
			event.InstructorName,
			event.CourseName,
			event.Extra,
		)
	default:
		return nil
	}

	return s.emailSvc.SendEmail(event.InstructorEmail, subject, body, true)
}

// SendAdminReviewNotification notifies admins about a new submission
func (s *WorkflowNotificationService) SendAdminReviewNotification(event *WorkflowEvent) error {
	// Get all admins
	var admins []models.User
	if err := db.DB.Where("role IN ?", []string{"ADMIN", "SUPER_ADMIN"}).Find(&admins).Error; err != nil {
		return err
	}

	baseURL := os.Getenv("FRONTEND_URL")
	if baseURL == "" {
		baseURL = "https://tolo-academy.com"
	}

	reviewURL := baseURL + "/admin/courses/" + event.SubjectID + "/review"
	emailBody := GetCourseSubmittedForReviewEmailTemplate(
		event.InstructorName,
		event.CourseName,
		reviewURL,
	)

	for _, admin := range admins {
		// In-app notification
		notification := models.Notification{
			ID:      uuid.New().String(),
			UserID:  admin.ID,
			Type:    "review_request",
			Title:   "كورس جديد بانتظار المراجعة",
			Message: event.InstructorName + " submitted " + event.CourseName + " for review",
			Status:  "pending",
		}
		s.notificationSvc.QueueNotification(notification)

		// Email (if enabled)
		if admin.Email != "" {
			s.emailSvc.SendEmail(admin.Email, "طلب مراجعة كورس جديد", emailBody, true)
		}
	}

	return nil
}

// SendDripReleaseNotification notifies students about new content
func (s *WorkflowNotificationService) SendDripReleaseNotification(subjectID, courseName, lessonName string) error {
	// Get enrolled students
	var enrollments []models.Enrollment
	if err := db.DB.Where("subject_id = ? AND status = ?", subjectID, "ACTIVE").Find(&enrollments).Error; err != nil {
		return err
	}

	baseURL := os.Getenv("FRONTEND_URL")
	if baseURL == "" {
		baseURL = "https://tolo-academy.com"
	}

	lessonURL := baseURL + "/courses/" + subjectID + "/lessons/" + lessonName

	for _, enrollment := range enrollments {
		var user models.User
		if err := db.DB.Select("id, email, name").First(&user, "id = ?", enrollment.UserID).Error; err != nil {
			continue
		}

		// In-app notification
		notification := models.Notification{
			ID:      uuid.New().String(),
			UserID:  user.ID,
			Type:    "content_release",
			Title:   WorkflowNotificationTitle("drip_released", courseName),
			Message: WorkflowNotificationBody("drip_released", courseName, lessonName),
			Status:  "pending",
		}
		s.notificationSvc.QueueNotification(notification)

		// Email (with personalized name)
		if user.Email != "" {
			personalizedBody := GetDripContentReleasedEmailTemplate(
				getUserDisplayName(&user),
				courseName,
				lessonName,
				lessonURL,
			)
			s.emailSvc.SendEmail(user.Email, "درس جديد متاح! 📚", personalizedBody, true)
		}
	}

	return nil
}

// getUserDisplayName returns the user's display name
func getUserDisplayName(user *models.User) string {
	if user.Name != nil && *user.Name != "" {
		return *user.Name
	}
	if user.Email != "" {
		return user.Email
	}
	return "طالب"
}

// =============================================================
// Admin Review Queue Service
// =============================================================

// ReviewQueueItem represents an item in the admin review queue
type ReviewQueueItem struct {
	SubjectID       string
	SubjectName     string
	SubjectNameAr   string
	InstructorID    string
	InstructorName  string
	InstructorEmail string
	SubmittedAt     string
	TopicsCount     int
	LessonsCount    int
	LastUpdatedAt   string
	TotalStudents   int
	TotalRevenue    float64
}

// GetReviewQueue returns courses pending review for admin dashboard
func (s *WorkflowNotificationService) GetReviewQueue(page, limit int) ([]ReviewQueueItem, int64, error) {
	var subjects []models.Subject
	var total int64

	query := db.DB.Model(&models.Subject{}).
		Where("status = ?", models.CourseStatusUnderReview)

	query.Count(&total)

	offset := (page - 1) * limit
	if err := query.Order("submitted_for_review_at ASC").
		Limit(limit).Offset(offset).
		Find(&subjects).Error; err != nil {
		return nil, 0, err
	}

	items := make([]ReviewQueueItem, 0, len(subjects))
	for _, subject := range subjects {
		// Get instructor
		var instructor models.User
		db.DB.Select("id, email, name").
			First(&instructor, "id = ?", subject.InstructorId)

		// Get topic/lesson counts
		var topicsCount, lessonsCount int64
		db.DB.Model(&models.Topic{}).Where("subject_id = ?", subject.ID).Count(&topicsCount)
		db.DB.Model(&models.SubTopic{}).
			Joins("JOIN Topic ON Topic.id = SubTopic.topic_id").
			Where("Topic.subject_id = ?", subject.ID).
			Count(&lessonsCount)

		item := ReviewQueueItem{
			SubjectID:       subject.ID,
			SubjectName:     subject.Name,
			SubjectNameAr:   stringOrEmpty(subject.NameAr),
			InstructorID:    stringOrEmpty(subject.InstructorId),
			InstructorName:  getUserDisplayName(&instructor),
			InstructorEmail: instructor.Email,
			SubmittedAt:     "",
			TopicsCount:     int(topicsCount),
			LessonsCount:    int(lessonsCount),
			LastUpdatedAt:   subject.UpdatedAt.Format("2006-01-02 15:04:05"),
		}

		if subject.SubmittedForReviewAt != nil {
			item.SubmittedAt = subject.SubmittedForReviewAt.Format("2006-01-02 15:04:05")
		}

		items = append(items, item)
	}

	return items, total, nil
}

// GetReviewStats returns statistics about the review queue
func (s *WorkflowNotificationService) GetReviewStats() (map[string]interface{}, error) {
	var pendingCount int64
	var todayCount int64
	var avgReviewTime float64

	db.DB.Model(&models.Subject{}).
		Where("status = ?", models.CourseStatusUnderReview).
		Count(&pendingCount)

	db.DB.Model(&models.Subject{}).
		Where("status = ? AND DATE(submitted_for_review_at) = CURRENT_DATE",
			models.CourseStatusUnderReview).
		Count(&todayCount)

	// Calculate average review time (for recently reviewed courses)
	var reviewedStats struct {
		AvgHours float64
	}
	db.DB.Model(&models.Subject{}).
		Select("AVG(EXTRACT(EPOCH FROM (reviewed_at - submitted_for_review_at)) / 3600) as avg_hours").
		Where("status = ? AND reviewed_at IS NOT NULL AND submitted_for_review_at IS NOT NULL",
			models.CourseStatusPublished).
		Order("reviewed_at DESC").
		Limit(50).
		Scan(&reviewedStats)

	if reviewedStats.AvgHours > 0 {
		avgReviewTime = reviewedStats.AvgHours
	}

	// Status distribution
	statusCounts := make(map[string]int64)
	var allCounts []struct {
		Status string
		Count  int64
	}
	db.DB.Model(&models.Subject{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&allCounts)
	for _, sc := range allCounts {
		statusCounts[sc.Status] = sc.Count
	}

	return map[string]interface{}{
		"pendingReview":      pendingCount,
		"submittedToday":     todayCount,
		"avgReviewHours":     avgReviewTime,
		"statusDistribution": statusCounts,
	}, nil
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
