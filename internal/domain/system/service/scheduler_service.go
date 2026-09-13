package systemservice

import (
	"encoding/json"
	"fmt"
	"strings"
	models "thanawy-backend/internal/domain/common"
	"time"

	db "thanawy-backend/internal/infrastructure/database"

	"github.com/google/uuid"
)

// SchedulerService handles scheduled item processing
type SchedulerService struct {
	quit chan struct{}
}

var schedulerServiceInstance *SchedulerService

// GetSchedulerService returns the singleton scheduler service
func GetSchedulerService() *SchedulerService {
	if schedulerServiceInstance == nil {
		schedulerServiceInstance = &SchedulerService{
			quit: make(chan struct{}),
		}
		// Start the background processor
		go schedulerServiceInstance.Start()
	}
	return schedulerServiceInstance
}

// Start begins the background scheduler
func (s *SchedulerService) Start() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.ProcessPendingItems()
		case <-s.quit:
			return
		}
	}
}

// Stop stops the scheduler
func (s *SchedulerService) Stop() {
	close(s.quit)
}

// ProcessPendingItems processes all pending items that are due
func (s *SchedulerService) ProcessPendingItems() {
	var items []models.ScheduledItem
	now := time.Now().UTC()

	db.DB.Where("status = ? AND scheduled_for <= ?", "pending", now).Find(&items)

	for _, item := range items {
		go s.ProcessItem(item.ID)
	}
}

// ProcessItem processes a single scheduled item
func (s *SchedulerService) ProcessItem(itemID string) error {
	var item models.ScheduledItem
	if err := db.DB.First(&item, "id = ?", itemID).Error; err != nil {
		return err
	}

	// Update status to processing
	item.Status = "processing"
	db.DB.Save(&item)

	// Process based on type
	var err error
	switch item.Type {
	case "announcement":
		err = s.processAnnouncement(item)
	case "exam":
		err = s.processExam(item)
	case "task":
		err = s.processTask(item)
	case "post":
		err = s.processPost(item)
	case "content":
		err = s.processContent(item)
	default:
		err = fmt.Errorf("unknown item type: %s", item.Type)
	}

	if err != nil {
		// Mark as failed
		item.Status = "failed"
		item.Error = err.Error()
		item.RetryCount++

		// Check if we should retry
		if item.RetryCount < item.MaxRetries {
			item.Status = "pending"
			// Reschedule for 5 minutes later
			item.ScheduledFor = time.Now().UTC().Add(5 * time.Minute)
		}
	} else {
		item.Status = "completed"
		now := time.Now().UTC()
		item.ExecutedAt = &now
	}

	return db.DB.Save(&item).Error
}

// announcementContent is the expected shape of item.Content for type "announcement".
// Mirrors the fields accepted by CreatePublicAnnouncement (public_community_handler.go).
type announcementContent struct {
	Message  string `json:"message"`
	Priority string `json:"priority"`
	Category string `json:"category"`
}

// processAnnouncement creates the real Notification row that backs public
// announcements (see protected.GetPublicAnnouncements /
// protected.CreatePublicAnnouncement, which read/write the same table).
func (s *SchedulerService) processAnnouncement(item models.ScheduledItem) error {
	if db.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	var content announcementContent
	if len(item.Content) > 0 {
		if err := json.Unmarshal(item.Content, &content); err != nil {
			return fmt.Errorf("invalid announcement content for scheduled item %s: %w", item.ID, err)
		}
	}
	message := strings.TrimSpace(content.Message)
	if message == "" {
		message = item.Description
	}
	if message == "" {
		return fmt.Errorf("scheduled announcement %s has no message/description", item.ID)
	}

	notification := models.Notification{
		UserID:   item.CreatedBy,
		Title:    item.Title,
		Message:  message,
		Type:     models.NotificationInfo,
		Category: strings.ToUpper(defaultOr(content.Category, "GENERAL")),
		Priority: strings.ToUpper(defaultOr(content.Priority, "MEDIUM")),
		IsRead:   false,
	}
	if err := db.DB.Create(&notification).Error; err != nil {
		return fmt.Errorf("failed to create announcement for scheduled item %s: %w", item.ID, err)
	}
	return nil
}

// examContent is the expected shape of item.Content for type "exam".
type examContent struct {
	SubjectID  string  `json:"subjectId"`
	Type       string  `json:"type"`
	Difficulty string  `json:"difficulty"`
	Duration   int     `json:"duration"`
	MaxScore   float64 `json:"maxScore"`
}

// processExam creates the real Exam row (models.common.Exam) so the exam
// becomes visible/active from its scheduled time.
func (s *SchedulerService) processExam(item models.ScheduledItem) error {
	if db.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	var content examContent
	if len(item.Content) > 0 {
		if err := json.Unmarshal(item.Content, &content); err != nil {
			return fmt.Errorf("invalid exam content for scheduled item %s: %w", item.ID, err)
		}
	}
	if strings.TrimSpace(content.SubjectID) == "" {
		return fmt.Errorf("scheduled exam %s is missing required subjectId in content", item.ID)
	}

	examType := models.ExamType(strings.ToUpper(defaultOr(content.Type, string(models.ExamTypeQuiz))))
	exam := models.Exam{
		ID:          uuid.New().String(),
		SubjectID:   content.SubjectID,
		Title:       item.Title,
		Type:        examType,
		Description: item.Description,
		Difficulty:  defaultOr(content.Difficulty, "medium"),
		IsActive:    true,
		Duration:    content.Duration,
		MaxScore:    content.MaxScore,
	}
	if exam.MaxScore == 0 {
		exam.MaxScore = 100
	}
	if err := db.DB.Create(&exam).Error; err != nil {
		return fmt.Errorf("failed to create exam for scheduled item %s: %w", item.ID, err)
	}
	return nil
}

// taskContent is the expected shape of item.Content for type "task".
type taskContent struct {
	UserID      string `json:"userId"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	SubjectID   string `json:"subjectId"`
}

// processTask creates the real Task row (models.common.Task).
func (s *SchedulerService) processTask(item models.ScheduledItem) error {
	if db.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	var content taskContent
	if len(item.Content) > 0 {
		if err := json.Unmarshal(item.Content, &content); err != nil {
			return fmt.Errorf("invalid task content for scheduled item %s: %w", item.ID, err)
		}
	}
	userID := content.UserID
	if userID == "" {
		userID = item.CreatedBy
	}
	if userID == "" {
		return fmt.Errorf("scheduled task %s has no target userId (content.userId or createdBy)", item.ID)
	}

	description := item.Description
	if content.Description != "" {
		description = content.Description
	}

	task := models.Task{
		ID:       uuid.New().String(),
		UserID:   userID,
		Title:    item.Title,
		Status:   models.TaskPending,
		Priority: strings.ToUpper(defaultOr(content.Priority, "MEDIUM")),
	}
	if description != "" {
		task.Description = &description
	}
	if content.SubjectID != "" {
		subjectID := content.SubjectID
		task.SubjectID = &subjectID
	}
	if err := db.DB.Create(&task).Error; err != nil {
		return fmt.Errorf("failed to create task for scheduled item %s: %w", item.ID, err)
	}
	return nil
}

// postContent is the expected shape of item.Content for type "post" (blog post).
type postContent struct {
	Slug       string   `json:"slug"`
	Body       string   `json:"content"`
	AuthorID   string   `json:"authorId"`
	CategoryID string   `json:"categoryId"`
	Tags       []string `json:"tags"`
	Image      string   `json:"image"`
}

// processPost creates the real BlogPost row, publishing it immediately
// (Status "PUBLISHED", PublishedAt = now) since its scheduled time has come.
func (s *SchedulerService) processPost(item models.ScheduledItem) error {
	if db.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	var content postContent
	if len(item.Content) > 0 {
		if err := json.Unmarshal(item.Content, &content); err != nil {
			return fmt.Errorf("invalid post content for scheduled item %s: %w", item.ID, err)
		}
	}
	slug := strings.TrimSpace(content.Slug)
	if slug == "" {
		return fmt.Errorf("scheduled post %s is missing required content.slug", item.ID)
	}
	authorID := content.AuthorID
	if authorID == "" {
		authorID = item.CreatedBy
	}
	if authorID == "" {
		return fmt.Errorf("scheduled post %s has no authorId (content.authorId or createdBy)", item.ID)
	}

	now := time.Now().UTC()
	post := models.BlogPost{
		ID:          uuid.New().String(),
		Title:       item.Title,
		Slug:        slug,
		Content:     content.Body,
		AuthorID:    authorID,
		CategoryID:  content.CategoryID,
		Tags:        models.JSONStringArray(content.Tags),
		Status:      "PUBLISHED",
		Image:       content.Image,
		PublishedAt: &now,
	}
	if err := db.DB.Create(&post).Error; err != nil {
		return fmt.Errorf("failed to create post for scheduled item %s: %w", item.ID, err)
	}
	return nil
}

// processContent processes generic content publishing.
//
// TODO(P012): no corresponding content-publishing service/model was found
// in this codebase (unlike announcement->Notification, exam->Exam,
// task->Task and post->BlogPost, which map onto real, existing tables).
// Wiring this up correctly requires knowing what a scheduled "content" item
// is actually supposed to publish (a Lesson? a generic CMS entry?) — that
// mapping doesn't exist yet, so this returns an explicit error instead of
// fabricating a fake write, and instead of the previous version which
// logged and returned nil (silently "succeeding" while doing nothing).
func (s *SchedulerService) processContent(item models.ScheduledItem) error {
	return fmt.Errorf("TODO(P012): scheduled item %s has type \"content\", but no content-publishing service exists in this codebase yet — not implemented", item.ID)
}

// defaultOr returns value if non-empty, otherwise fallback.
func defaultOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
