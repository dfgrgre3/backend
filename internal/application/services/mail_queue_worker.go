package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	notificationservice "thanawy-backend/internal/domain/notification/service"
	db "thanawy-backend/internal/infrastructure/database"

	"gorm.io/gorm"
)

// MailTask represents a persistent email job stored in PostgreSQL
type MailTask struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	To        string         `gorm:"index;size:255" json:"to"`
	Subject   string         `gorm:"size:255" json:"subject"`
	Body      string         `gorm:"type:text" json:"body"`
	Status    string         `gorm:"index;size:50;default:'pending'" json:"status"` // pending, processing, sent, failed, retry
	Attempts  int            `gorm:"default:0" json:"attempts"`
	MaxRetry  int            `gorm:"column:max_retry;default:3" json:"maxRetry"`
	LastError string         `gorm:"column:last_error;type:text" json:"lastError"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Polling behaviour. When the queue is empty the worker backs off
// exponentially up to maxPollInterval instead of hammering the database every
// 10s, and drops straight back to minPollInterval as soon as work appears.
const (
	minPollInterval = 10 * time.Second
	maxPollInterval = 5 * time.Minute
	claimBatchSize  = 100

	// A task claimed by a worker that then crashed stays in 'processing'
	// forever. Anything stuck for longer than this is returned to the queue.
	processingStaleAfter = 10 * time.Minute
)

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
		log.Println("[MailWorker] Already running, skipping duplicate start")
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
	if !w.active {
		w.mu.Unlock()
		return
	}

	w.active = false
	quit := w.quit
	w.mu.Unlock()

	close(quit)
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
	emailSvc := notificationservice.GetEmailService()
	return emailSvc.SendEmail(to, subject, body, true)
}

func (w *MailQueueWorker) run() {
	interval := minPollInterval
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-w.quit:
			return
		case <-timer.C:
			processed := w.processPendingTasks()

			// Back off while the queue is empty, reset the moment it is not.
			if processed > 0 {
				interval = minPollInterval
			} else if interval < maxPollInterval {
				interval *= 2
				if interval > maxPollInterval {
					interval = maxPollInterval
				}
			}
			timer.Reset(interval)
		}
	}
}

// processPendingTasks claims a batch of due tasks and dispatches them.
// It returns the number of tasks dispatched so the caller can adjust its poll
// interval.
func (w *MailQueueWorker) processPendingTasks() int {
	if db.DB == nil {
		return 0
	}

	w.requeueStaleTasks()

	tasks, err := w.claimTasks()
	if err != nil {
		log.Printf("[MailWorker] Failed to claim email tasks: %v", err)
		return 0
	}
	if len(tasks) == 0 {
		return 0
	}

	emailSvc := notificationservice.GetEmailService()

	for _, task := range tasks {
		log.Printf("[MailWorker] Dispatching email task %d to %s (Attempt %d)", task.ID, task.To, task.Attempts)

		status := "sent"
		lastError := ""
		if sendErr := emailSvc.SendEmail(task.To, task.Subject, task.Body, true); sendErr != nil {
			log.Printf("[MailWorker] Failed to send email to %s: %v", task.To, sendErr)
			lastError = sendErr.Error()
			if task.Attempts >= task.MaxRetry {
				status = "failed"
			} else {
				status = "retry"
			}
		} else {
			log.Printf("[MailWorker] Email task %d sent successfully to %s", task.ID, task.To)
		}

		w.finalizeTask(task.ID, status, lastError)
	}

	return len(tasks)
}

// claimTasks atomically moves a batch of due tasks from pending/retry into
// 'processing' and returns them. FOR UPDATE SKIP LOCKED means concurrent
// workers (rolling deploys, multiple APP_MODE=worker replicas) each get a
// disjoint batch instead of racing to send the same email twice.
//
// The retry backoff is evaluated in SQL — attempt 1 waits 30s, attempt 2 waits
// 2m, attempt 3 waits 4m30s — so rows that are not due yet are never fetched.
func (w *MailQueueWorker) claimTasks() ([]MailTask, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const claimSQL = `
		WITH due AS (
			SELECT id
			FROM "MailTask"
			WHERE deleted_at IS NULL
			  AND status IN ('pending', 'retry')
			  AND (
			        attempts = 0
			     OR updated_at <= NOW() - make_interval(secs => attempts * attempts * 30)
			  )
			ORDER BY created_at ASC
			LIMIT ?
			FOR UPDATE SKIP LOCKED
		)
		UPDATE "MailTask" AS t
		SET status     = 'processing',
		    attempts   = t.attempts + 1,
		    updated_at = NOW()
		FROM due
		WHERE t.id = due.id
		RETURNING t.id, t."to", t.subject, t.body, t.status,
		          t.attempts, t.max_retry, t.last_error,
		          t.created_at, t.updated_at`

	var tasks []MailTask
	// The queue must always be read from the write source: a replica may not
	// yet have the row that was just enqueued, and SELECT ... FOR UPDATE is
	// not valid against a read-only replica anyway.
	err := db.WriteDB(ctx).Raw(claimSQL, claimBatchSize).Scan(&tasks).Error
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

// finalizeTask records the terminal state of a dispatched task.
func (w *MailQueueWorker) finalizeTask(id uint, status, lastError string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := db.WriteDB(ctx).
		Model(&MailTask{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     status,
			"last_error": lastError,
			"updated_at": time.Now().UTC(),
		}).Error
	if err != nil {
		log.Printf("[MailWorker] Failed to update email task %d status: %v", id, err)
	}
}

// requeueStaleTasks returns tasks abandoned by a crashed worker to the queue.
// Without this, a process that dies mid-dispatch would leave its claimed rows
// in 'processing' forever.
//
// The UPDATE is only issued when a stale row actually exists. In the common
// case (no crashed worker) the cheap existence probe below matches zero rows
// and we skip the UPDATE entirely, avoiding a full-table scan on every poll
// cycle. The probe is served by the partial index
// idx_mail_task_processing_stale (migration 0132) when present.
func (w *MailQueueWorker) requeueStaleTasks() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const staleProbeSQL = `
		SELECT 1
		FROM "MailTask"
		WHERE deleted_at IS NULL
		  AND status = 'processing'
		  AND updated_at < NOW() - make_interval(secs => ?)
		LIMIT 1`

	var found int
	if err := db.WriteDB(ctx).Raw(staleProbeSQL, processingStaleAfter.Seconds()).Scan(&found).Error; err != nil {
		log.Printf("[MailWorker] Failed to probe for stale email tasks: %v", err)
		return
	}
	if found == 0 {
		return
	}

	const requeueSQL = `
		UPDATE "MailTask"
		SET status     = CASE WHEN attempts >= max_retry THEN 'failed' ELSE 'retry' END,
		    last_error = 'worker terminated before the task completed',
		    updated_at = NOW()
		WHERE deleted_at IS NULL
		  AND status = 'processing'
		  AND updated_at < NOW() - make_interval(secs => ?)`

	result := db.WriteDB(ctx).Exec(requeueSQL, processingStaleAfter.Seconds())
	if result.Error != nil {
		log.Printf("[MailWorker] Failed to requeue stale email tasks: %v", result.Error)
		return
	}
	if result.RowsAffected > 0 {
		log.Printf("[MailWorker] Requeued %d stale email task(s) left behind by a terminated worker", result.RowsAffected)
	}
}
