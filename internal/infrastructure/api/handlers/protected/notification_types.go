package protected

import "time"

// NotificationRequest represents a broadcast notification request
type NotificationRequest struct {
	UserIDs      []string   `json:"userIds" binding:"required,min=1"`
	Title        string     `json:"title" binding:"required,max=200"`
	Message      string     `json:"message" binding:"required,max=2000"`
	Type         string     `json:"type" binding:"omitempty,oneof=info success warning error"`
	Channels     []string   `json:"channels" binding:"required,min=1"`
	ActionURL    string     `json:"actionUrl" binding:"omitempty"`
	Priority     string     `json:"priority" binding:"omitempty,oneof=high normal low"`
	ScheduledFor *time.Time `json:"scheduledFor,omitempty"`
}

// NotificationResponse represents the response from sending notifications
type NotificationResponse struct {
	BroadcastID string              `json:"broadcastId"`
	Summary     NotificationSummary `json:"summary"`
	Queued      bool                `json:"queued"`
}

// NotificationSummary contains delivery statistics
type NotificationSummary struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failure int `json:"failure"`
	Queued  int `json:"queued"`
}
