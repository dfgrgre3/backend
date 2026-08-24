package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

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

	// Implement the logic that was in Next.js
	for _, channel := range p.Channels {
		if err := h.sendViaChannel(ctx, channel, p); err != nil {
			log.Printf("Failed to send via %s: %v", channel, err)
			// Continue with other channels
		}
	}

	return nil
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

func (h *NotificationHandler) sendEmail(_ context.Context, p NotificationPayload) error {
	log.Printf("[Worker] STUB (not implemented): would send email to %s: %s", p.UserID, p.Title)
	return nil
}

func (h *NotificationHandler) sendSMS(_ context.Context, p NotificationPayload) error {
	log.Printf("[Worker] STUB (not implemented): would send SMS to %s", p.UserID)
	return nil
}

func (h *NotificationHandler) sendPush(_ context.Context, p NotificationPayload) error {
	log.Printf("[Worker] STUB (not implemented): would send push to %s: %s", p.UserID, p.Title)
	return nil
}

func (h *NotificationHandler) sendInApp(_ context.Context, p NotificationPayload) error {
	// STUB (not implemented): should create a models.Notification row so it
	// shows up via GetNotifications, but currently only logs.
	log.Printf("[Worker] STUB (not implemented): storing in-app notification for %s", p.UserID)
	return nil
}
