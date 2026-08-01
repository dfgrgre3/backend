package worker

import (
	"context"
	"encoding/json"
	"log"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"
	"time"

	"github.com/hibiken/asynq"
)

const (
	TypeSessionCleanup = "auth:session_cleanup"
)

type SessionCleanupPayload struct{}

func NewSessionCleanupTask() (*asynq.Task, error) {
	payload := SessionCleanupPayload{}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeSessionCleanup, data), nil
}

type SessionCleanupHandler struct{}

func (h *SessionCleanupHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p SessionCleanupPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}

	log.Println("[SessionCleanupWorker] Starting session cleanup...")
	now := time.Now().UTC()
	// Mark expired sessions as inactive/expired
	result := db.DB.WithContext(ctx).Model(&models.UserSession{}).
		Where("expires_at < ? AND is_active = ?", now, true).
		Updates(map[string]interface{}{
			"is_active": false,
			"status":    "expired",
		})

	if result.Error != nil {
		log.Printf("[SessionCleanupWorker] Failed to update expired sessions: %v", result.Error)
		return result.Error
	}
	log.Printf("[SessionCleanupWorker] Deactivated %d expired sessions", result.RowsAffected)

	// Hard delete old sessions (> 90 days ago) to keep database tidy
	cleanupCutoff := now.AddDate(0, 0, -90)
	deleteResult := db.DB.WithContext(ctx).
		Where("(status = ? OR status = ? OR is_active = ?) AND updated_at < ?", "expired", "revoked", false, cleanupCutoff).
		Unscoped().
		Delete(&models.UserSession{})

	if deleteResult.Error != nil {
		log.Printf("[SessionCleanupWorker] Failed to hard-delete old sessions: %v", deleteResult.Error)
		return deleteResult.Error
	}
	log.Printf("[SessionCleanupWorker] Hard deleted %d old sessions from database", deleteResult.RowsAffected)

	return nil
}
