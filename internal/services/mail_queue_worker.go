package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"thanawy-backend/internal/db"

	"gorm.io/gorm"
)

// MailTask represents a persistent email job stored in PostgreSQL
type MailTask struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	To        string         `gorm:"index;size:255" json:"to"`
	Subject   string         `gorm:"size:255" json:"subject"`
	Body      string         `gorm:"type:text" json:"body"`
	Status    string         `gorm:"index;size:50;default:'pending'" json:"status"` // pending, sent, failed, retry
	Attempts  int            `gorm:"default:0" json:"attempts"`
	MaxRetry  int            `gorm:"default:3" json:"maxRetry"`
	LastError string         `gorm:"type:text" json:"lastError"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type MailQueueWorker struct {
	mu     sync.Mutex
	active bool
	quit   chan struct{}
}

var mailQueueWorkerInstance *MailQueueWorker
var workerOnce sync.Once

// GetMailQueueWorker returns the singleton instance of MailQueueWorker
func GetMailQueueWorker() *MailQueueWorker {
	workerOnce.Do(func() {
		mailQueueWorkerInstance = &MailQueueWorker{
			quit: make(chan struct{}),
		}
	})
	return mailQueueWorkerInstance
}

// Start launches the background polling worker
func (w *MailQueueWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.active {
		return
	}

	w.active = true
	w.quit = make(chan struct{})

	go w.run()
	log.Println("[MailWorker] Background daemon started successfully")
}

// Stop halts the polling worker
func (w *MailQueueWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.active {
		return
	}

	close(w.quit)
	w.active = false
	log.Println("[MailWorker] Background daemon stopped")
}

// Enqueue adds a new mail task to the database to be processed asynchronously
func (w *MailQueueWorker) Enqueue(to, subject, body string) error {
	task := MailTask{
		To:       to,
		Subject:  subject,
		Body:     body,
		Status:   "pending",
		Attempts: 0,
		MaxRetry: 3,
	}

	if db.DB != nil {
		if err := db.DB.Create(&task).Error; err != nil {
			return fmt.Errorf("failed to save mail task to DB: %w", err)
		}
		return nil
	}

	// Fallback to direct synchronous execution if DB is not available
	log.Println("[MailWorker] Database connection offline, executing fallback email dispatch")
	emailSvc := GetEmailService()
	return emailSvc.SendEmail(to, subject, body, true)
}

func (w *MailQueueWorker) run() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.quit:
			return
		case <-ticker.C:
			w.processPendingTasks()
		}
	}
}

func (w *MailQueueWorker) processPendingTasks() {
	if db.DB == nil {
		return
	}

	var tasks []MailTask
	// Use context with timeout to prevent slow queries from blocking
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Fetch only needed columns, filter by status and deleted_at, order by created_at ASC
	err := db.DB.WithContext(ctx).
		Unscoped().
		Select("id, \"to\", subject, body, status, attempts, max_retry, last_error, created_at, updated_at").
		Where("status IN ? AND deleted_at IS NULL", []string{"pending", "retry"}).
		Order("created_at ASC").
		Limit(100).
		Find(&tasks).Error

	if err != nil {
		log.Printf("[MailWorker] Failed to query email tasks: %v", err)
		return
	}

	if len(tasks) == 0 {
		return
	}

	emailSvc := GetEmailService()

	for _, task := range tasks {
		// Calculate backoff if retrying
		if task.Attempts > 0 {
			// Exponential backoff: check if enough time has passed
			// attempt 1: wait 30s, attempt 2: wait 2m, attempt 3: wait 5m
			backoff := time.Duration(task.Attempts*task.Attempts*30) * time.Second
			if time.Since(task.UpdatedAt) < backoff {
				continue
			}
		}

		task.Attempts++
		log.Printf("[MailWorker] Dispatching email task %d to %s (Attempt %d)", task.ID, task.To, task.Attempts)

		sendErr := emailSvc.SendEmail(task.To, task.Subject, task.Body, true)
		if sendErr != nil {
			log.Printf("[MailWorker] Failed to send email to %s: %v", task.To, sendErr)
			task.LastError = sendErr.Error()

			if task.Attempts >= task.MaxRetry {
				task.Status = "failed"
			} else {
				task.Status = "retry"
			}
		} else {
			log.Printf("[MailWorker] Email task %d sent successfully to %s", task.ID, task.To)
			task.Status = "sent"
			task.LastError = ""
		}

		if saveErr := db.DB.Save(&task).Error; saveErr != nil {
			log.Printf("[MailWorker] Failed to update email task %d status: %v", task.ID, saveErr)
		}
	}
}
