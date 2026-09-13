package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// Task names
const (
	TypeMultiChannelNotification = "notification:multi_channel"
)

// NotificationPayload matches the frontend NotificationJobPayload
type NotificationPayload struct {
	UserID   string                 `json:"userId"`
	Type     string                 `json:"type"`
	Title    string                 `json:"title"`
	Message  string                 `json:"message"`
	Channels []string               `json:"channels"`
	Metadata map[string]interface{} `json:"metadata"`
	Priority string                 `json:"priority"`
}

// NewMultiChannelNotificationTask creates a new task for multi-channel notifications
func NewMultiChannelNotificationTask(payload NotificationPayload) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeMultiChannelNotification, data), nil
}

// NotificationHandler handles notification tasks
type NotificationHandler struct{}

func (h *NotificationHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p NotificationPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	log.Printf("Processing notification for user %s: %s", p.UserID, p.Title)
	if _, err := uuid.Parse(p.UserID); err != nil {
		return fmt.Errorf("invalid notification user id: %w: %w", err, asynq.SkipRetry)
	}
	if strings.TrimSpace(p.Title) == "" || strings.TrimSpace(p.Message) == "" {
		return fmt.Errorf("notification title and message are required: %w", asynq.SkipRetry)
	}
	if len(p.Channels) == 0 {
		return fmt.Errorf("notification must specify at least one channel: %w", asynq.SkipRetry)
	}

	var firstErr error
	for _, channel := range p.Channels {
		if err := h.sendViaChannel(ctx, channel, p); err != nil {
			log.Printf("Failed to send via %s: %v", channel, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

func (h *NotificationHandler) sendViaChannel(ctx context.Context, channel string, p NotificationPayload) error {
	switch channel {
	case "email":
		return h.sendEmail(ctx, p)
	case "sms":
		return h.sendSMS(ctx, p)
	case "push":
		return h.sendPush(ctx, p)
	case "in-app":
		return h.sendInApp(ctx, p)
	default:
		return fmt.Errorf("unknown channel: %s", channel)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// KNOWN GAP: every channel below is an UNIMPLEMENTED STUB, not a working
// integration. Each one logs "Would send..." and returns nil (success)
// without actually delivering anything. This means EnqueueNotification /
// CreateNotificationTask (the HTTP endpoint that queues this task) currently
// no-op the entire multi-channel notification system silently — any caller
// believes a notification was sent when nothing happened.
//
// This is left unimplemented deliberately (not silently) because wiring up
// real delivery requires provider credentials/config this audit does not
// have visibility into (SMTP/mail service, SMS gateway, push notification
// service). Before relying on this path in production:
//   - sendEmail needs a real mail sender (there is an existing verification/
//     forgot-password email flow in internal/domain/notification/service —
//     check whether its underlying mail client can be reused here).
//   - sendSMS needs an SMS gateway integration (e.g. Twilio, a local
//     provider) — none currently exists in this codebase.
//   - sendPush needs a push notification service (FCM/APNs or similar).
//   - sendInApp needs to actually persist a Notification row (the
//     `models.Notification` used by GetNotifications already exists) —
//     currently it only logs.
// ─────────────────────────────────────────────────────────────────────────

func (h *NotificationHandler) sendEmail(_ context.Context, _ NotificationPayload) error {
	return fmt.Errorf("email notification provider is not configured: %w", asynq.SkipRetry)
}

func (h *NotificationHandler) sendSMS(_ context.Context, _ NotificationPayload) error {
	return fmt.Errorf("sms notification provider is not configured: %w", asynq.SkipRetry)
}

func (h *NotificationHandler) sendPush(_ context.Context, _ NotificationPayload) error {
	return fmt.Errorf("push notification provider is not configured: %w", asynq.SkipRetry)
}

func (h *NotificationHandler) sendInApp(ctx context.Context, p NotificationPayload) error {
	if db.DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	notification := models.Notification{
		UserID: p.UserID, Title: p.Title, Message: p.Message,
		Type: normalizeNotificationType(p.Type), Category: "GENERAL",
		Priority: normalizePriority(p.Priority), Status: "delivered",
		Channels: models.StringArray{"in-app"}, IsRead: false,
	}
	if actionURL, ok := p.Metadata["actionUrl"].(string); ok && actionURL != "" {
		notification.Link = &actionURL
	}
	return db.DB.WithContext(ctx).Create(&notification).Error
}

func normalizeNotificationType(value string) models.NotificationType {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "SUCCESS":
		return models.NotificationSuccess
	case "WARNING":
		return models.NotificationWarning
	case "ERROR":
		return models.NotificationError
	default:
		return models.NotificationInfo
	}
}

func normalizePriority(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "high":
		return "HIGH"
	case "low":
		return "LOW"
	default:
		return "MEDIUM"
	}
}
