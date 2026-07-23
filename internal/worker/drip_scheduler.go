package worker

import (
	"encoding/json"
	"log"
	"time"

	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"
	"thanawy-backend/internal/services"

	"github.com/hibiken/asynq"
)

// =============================================================
// Drip Content Scheduler Worker
// =============================================================

// TypeDripContentRelease is the task type for drip content release
const TypeDripContentRelease = "drip:content_release"

// DripContentPayload contains data for drip release tasks
type DripContentPayload struct {
	SubTopicID string `json:"sub_topic_id"`
	SubjectID  string `json:"subject_id"`
	LessonName string `json:"lesson_name"`
	CourseName string `json:"course_name"`
}

// ScheduleDripContentRelease schedules a drip content release task
func ScheduleDripContentRelease(dripSchedule *models.LessonDripSchedule, subjectName, lessonName string) error {
	if dripSchedule.DripType != models.DripAbsolute || dripSchedule.ReleaseDate == nil {
		return nil // Only schedule absolute drip types
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
		SubjectID:  dripSchedule.SubTopicID, // Will be resolved
		LessonName: lessonName,
		CourseName: subjectName,
	}

	payloadBytes, _ := json.Marshal(payload)

	// Calculate delay until release
	delay := time.Until(*dripSchedule.ReleaseDate)
	if delay < 0 {
		delay = 0
	}

	task := asynq.NewTask(TypeDripContentRelease, payloadBytes)
	info, err := client.Enqueue(task, asynq.ProcessAt(*dripSchedule.ReleaseDate))
	if err != nil {
		log.Printf("[DripScheduler] Failed to enqueue drip task: %v", err)
		return err
	}

	log.Printf("[DripScheduler] Scheduled drip release for lesson %s at %s (task_id=%s)",
		dripSchedule.SubTopicID, dripSchedule.ReleaseDate.Format(time.RFC3339), info.ID)
	return nil
}

// HandleDripContentRelease processes drip content release notifications
func HandleDripContentRelease(task *asynq.Task) error {
	var payload DripContentPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		log.Printf("[DripScheduler] Failed to parse drip payload: %v", err)
		return err
	}

	// Get subject ID from SubTopic
	var subTopic models.SubTopic
	if err := db.DB.Preload("Topic").First(&subTopic, "id = ?", payload.SubTopicID).Error; err != nil {
		log.Printf("[DripScheduler] SubTopic not found: %s", payload.SubTopicID)
		return err
	}

	subjectID := subTopic.Topic.SubjectID

	// Get course name
	var subject models.Subject
	db.DB.Select("name, name_ar").First(&subject, "id = ?", subjectID)
	courseName := subject.Name
	if subject.NameAr != nil && *subject.NameAr != "" {
		courseName = *subject.NameAr
	}

	// Send notifications to enrolled students
	notificationSvc := services.NewWorkflowNotificationService()
	if err := notificationSvc.SendDripReleaseNotification(subjectID, courseName, subTopic.Title); err != nil {
		log.Printf("[DripScheduler] Failed to send drip notifications: %v", err)
		// Don't fail the task for notification errors
	}

	log.Printf("[DripScheduler] Drip content released: %s in course %s (%d notifications sent)",
		subTopic.Title, courseName, 0) // Count would require additional query

	return nil
}

// =============================================================
// Course Availability Scheduler
// =============================================================

// TypeCourseAvailability is the task type for course availability changes
const TypeCourseAvailability = "course:availability"

// CourseAvailabilityPayload contains data for availability tasks
type CourseAvailabilityPayload struct {
	SubjectID  string `json:"subject_id"`
	CourseName string `json:"course_name"`
	Action     string `json:"action"` // publish, unpublish, open_enrollment, close_enrollment
	WindowType string `json:"window_type"`
}

// ScheduleCourseAvailability schedules a course availability change
func ScheduleCourseAvailability(subjectID, courseName, action, windowType string, at time.Time) error {
	redisAddr := getRedisAddr()
	if redisAddr == "" {
		return nil
	}

	client := asynq.NewClient(getRedisConnOpt())
	defer client.Close()

	payload := CourseAvailabilityPayload{
		SubjectID:  subjectID,
		CourseName: courseName,
		Action:    action,
		WindowType: windowType,
	}

	payloadBytes, _ := json.Marshal(payload)

	delay := time.Until(at)
	if delay < 0 {
		delay = 0
	}

	task := asynq.NewTask(TypeCourseAvailability, payloadBytes)
	_, err := client.Enqueue(task, asynq.ProcessAt(at))
	if err != nil {
		log.Printf("[AvailabilityScheduler] Failed to enqueue task: %v", err)
		return err
	}

	log.Printf("[AvailabilityScheduler] Scheduled %s for course %s at %s", action, subjectID, at.Format(time.RFC3339))
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
		return err
	}

	now := time.Now()
	updates := map[string]interface{}{"updated_at": now}

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
	}

	if err := db.DB.Model(&subject).Updates(updates).Error; err != nil {
		log.Printf("[AvailabilityScheduler] Failed to update course %s: %v", payload.SubjectID, err)
		return err
	}

	log.Printf("[AvailabilityScheduler] Applied %s to course %s", payload.Action, payload.SubjectID)
	return nil
}

// =============================================================
// Helper functions
// =============================================================

func getRedisAddr() string {
	return asynqConfig.redisAddr
}

func getRedisConnOpt() asynq.RedisConnOpt {
	redisAddr := getRedisAddr()
	if redisAddr == "" {
		return asynq.RedisClientOpt{}
	}
	opts, _ := asynq.ParseRedisURI(redisAddr)
	return opts
}

var asynqConfig = struct {
	redisAddr string
}{}

func init() {
	// Import is handled at package level
}
