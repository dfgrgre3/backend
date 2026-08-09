package worker

import (
	"encoding/json"
	"log"
	models "thanawy-backend/internal/domain/common"
	"thanawy-backend/internal/infrastructure/cache"
	"time"

	"github.com/hibiken/asynq"
	notificationservice "thanawy-backend/internal/domain/notification/service"
	db "thanawy-backend/internal/infrastructure/database"
	// ⚠️ تأكد من استيراد حزمة models والمسار الصحيح لـ asynq config helpers
	// "thanawy-backend/internal/domain/course/models"
	// "thanawy-backend/internal/infrastructure/queue"
)

// =============================================================
// Constants & Task Types
// =============================================================

const (
	// TypeDripContentRelease is the task type for drip content release
	TypeDripContentRelease = "drip:content_release"

	// TypeCourseAvailability is the task type for course availability changes
	TypeCourseAvailability = "course:availability"
)

// =============================================================
// Payloads
// =============================================================

// DripContentPayload contains data for drip release tasks
type DripContentPayload struct {
	SubTopicID string `json:"sub_topic_id"`
	SubjectID  string `json:"subject_id"`
	LessonName string `json:"lesson_name"`
	CourseName string `json:"course_name"`
}

// CourseAvailabilityPayload contains data for availability tasks
type CourseAvailabilityPayload struct {
	SubjectID  string `json:"subject_id"`
	CourseName string `json:"course_name"`
	Action     string `json:"action"` // publish, unpublish, open_enrollment, close_enrollment
	WindowType string `json:"window_type"`
}

// =============================================================
// Drip Content Scheduler
// =============================================================

// ScheduleDripContentRelease schedules a drip content release task
func ScheduleDripContentRelease(dripSchedule *models.LessonDripSchedule, subjectName, lessonName string) error {
	// Only schedule absolute drip types with a valid release date
	if dripSchedule.DripType != models.DripAbsolute || dripSchedule.ReleaseDate == nil {
		return nil
	}

	redisAddr := getRedisAddr()
	if redisAddr == "" {
		log.Printf("[DripScheduler] Redis not configured, cannot schedule drip task")
		return nil
	}

	client := asynq.NewClient(getRedisConnOpt())
	defer client.Close()

	payload := DripContentPayload{
		SubTopicID: dripSchedule.SubTopicID,
		SubjectID:  "", // Will be resolved from SubTopic during handling
		LessonName: lessonName,
		CourseName: subjectName,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[DripScheduler] Failed to marshal drip payload: %v", err)
		return err
	}

	task := asynq.NewTask(TypeDripContentRelease, payloadBytes)
	info, err := client.Enqueue(task, asynq.ProcessAt(*dripSchedule.ReleaseDate))
	if err != nil {
		log.Printf("[DripScheduler] Failed to enqueue drip task: %v", err)
		return err
	}

	log.Printf("[DripScheduler] Scheduled drip release for lesson %s at %s (task_id=%s)",
		dripSchedule.SubTopicID,
		dripSchedule.ReleaseDate.Format(time.RFC3339),
		info.ID,
	)
	return nil
}

// HandleDripContentRelease processes drip content release notifications
func HandleDripContentRelease(task *asynq.Task) error {
	var payload DripContentPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		log.Printf("[DripScheduler] Failed to parse drip payload: %v", err)
		return err
	}

	// Gracefully return if no SubTopicID (e.g., periodic fallback check)
	if payload.SubTopicID == "" {
		return nil
	}

	// Get subject ID from SubTopic
	var subTopic models.SubTopic
	if err := db.DB.Preload("Topic").First(&subTopic, "id = ?", payload.SubTopicID).Error; err != nil {
		log.Printf("[DripScheduler] SubTopic not found: %s", payload.SubTopicID)
		return err
	}

	subjectID := subTopic.Topic.SubjectID

	// Get course name (prefer Arabic name if available)
	var subject models.Subject
	if err := db.DB.Select("name, name_ar").First(&subject, "id = ?", subjectID).Error; err != nil {
		log.Printf("[DripScheduler] Subject not found: %s", subjectID)
		return err
	}

	courseName := subject.Name
	if subject.NameAr != nil && *subject.NameAr != "" {
		courseName = *subject.NameAr
	}

	// Send notifications to enrolled students
	notificationSvc := notificationservice.NewWorkflowNotificationService()
	if err := notificationSvc.SendDripReleaseNotification(subjectID, courseName, subTopic.Title); err != nil {
		log.Printf("[DripScheduler] Failed to send drip notifications: %v", err)
		// Don't fail the task for notification errors
	}

	log.Printf("[DripScheduler] Drip content released: %s in course %s", subTopic.Title, courseName)
	return nil
}

// =============================================================
// Course Availability Scheduler
// =============================================================

// ScheduleCourseAvailability schedules a course availability change
func ScheduleCourseAvailability(subjectID, courseName, action, windowType string, at time.Time) error {
	redisAddr := getRedisAddr()
	if redisAddr == "" {
		log.Printf("[AvailabilityScheduler] Redis not configured, cannot schedule availability task")
		return nil
	}

	client := asynq.NewClient(getRedisConnOpt())
	defer client.Close()

	payload := CourseAvailabilityPayload{
		SubjectID:  subjectID,
		CourseName: courseName,
		Action:     action,
		WindowType: windowType,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[AvailabilityScheduler] Failed to marshal availability payload: %v", err)
		return err
	}

	task := asynq.NewTask(TypeCourseAvailability, payloadBytes)
	_, err = client.Enqueue(task, asynq.ProcessAt(at))
	if err != nil {
		log.Printf("[AvailabilityScheduler] Failed to enqueue task: %v", err)
		return err
	}

	localTime := at.In(SchedulerLocation()).Format("2006-01-02 15:04:05 MST")
	log.Printf("[AvailabilityScheduler] Scheduled %s for course %s at %s (local: %s)",
		action, subjectID, at.UTC().Format(time.RFC3339), localTime,
	)
	return nil
}

// HandleCourseAvailability processes course availability changes
func HandleCourseAvailability(task *asynq.Task) error {
	var payload CourseAvailabilityPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}

	var subject models.Subject
	if err := db.DB.First(&subject, "id = ?", payload.SubjectID).Error; err != nil {
		log.Printf("[AvailabilityScheduler] Subject not found: %s", payload.SubjectID)
		return err
	}

	now := time.Now().UTC()
	updates := map[string]interface{}{
		"updated_at": now,
	}

	switch payload.Action {
	case "publish":
		updates["status"] = models.CourseStatusPublished
		updates["is_published"] = true
		updates["published_at"] = now
	case "unpublish":
		updates["status"] = models.CourseStatusArchived
		updates["is_published"] = false
		updates["archived_at"] = now
	case "open_enrollment":
		updates["enrollment_type"] = models.EnrollmentTypeOpen
	case "close_enrollment":
		updates["enrollment_type"] = models.EnrollmentTypeLimited
	default:
		log.Printf("[AvailabilityScheduler] Unknown action: %s for course %s", payload.Action, payload.SubjectID)
		return nil
	}

	if err := db.DB.Model(&subject).Updates(updates).Error; err != nil {
		log.Printf("[AvailabilityScheduler] Failed to update course %s: %v", payload.SubjectID, err)
		return err
	}

	log.Printf("[AvailabilityScheduler] Applied %s to course %s", payload.Action, payload.SubjectID)
	return nil
}

// =============================================================
// Helper Functions
// =============================================================

// asynqConfig holds the Redis configuration for Asynq
// ⚠️ TODO: Replace with your actual config source (e.g., viper, env vars)
var asynqConfig = struct {
	redisAddr string
}{
	redisAddr: "", // Set this via init or config loader
}

func getRedisAddr() string {
	return asynqConfig.redisAddr
}

func getRedisConnOpt() asynq.RedisConnOpt {
	return cache.ParseAsynqRedisConnOpt(getRedisAddr())
}

// ⚠️ Placeholder: Implement or import these from your infrastructure package
// func cache.ParseAsynqRedisConnOpt(addr string) asynq.RedisConnOpt { ... }
// func SchedulerLocation() *time.Location { ... }

func init() {
	// Initialize asynqConfig.redisAddr from your configuration source here
	// Example: asynqConfig.redisAddr = os.Getenv("REDIS_URL")
}
