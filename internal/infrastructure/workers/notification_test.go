package worker

import (
	models "thanawy-backend/internal/domain/common"
	"testing"
)

func TestNormalizeNotificationType(t *testing.T) {
	tests := []struct {
		input string
		want  models.NotificationType
	}{
		{"success", models.NotificationSuccess},
		{" WARNING ", models.NotificationWarning},
		{"error", models.NotificationError},
		{"unknown", models.NotificationInfo},
		{"", models.NotificationInfo},
	}
	for _, tt := range tests {
		if got := normalizeNotificationType(tt.input); got != tt.want {
			t.Fatalf("normalizeNotificationType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizePriority(t *testing.T) {
	for input, want := range map[string]string{
		"high": "HIGH", " LOW ": "LOW", "normal": "MEDIUM", "": "MEDIUM",
	} {
		if got := normalizePriority(input); got != want {
			t.Fatalf("normalizePriority(%q) = %q, want %q", input, got, want)
		}
	}
}
