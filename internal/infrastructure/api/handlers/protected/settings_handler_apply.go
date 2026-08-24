package protected

import (
	"strings"
	models "thanawy-backend/internal/domain/common"
)

// normalizeEnum mirrors the normalization validateSettingsPatch applies before
// checking a string enum value against its allowed set (trim + lowercase).
// Applying the same normalization here ensures the persisted value matches
// what was actually validated — without it, a value like " DARK " passes
// validation (checked case-insensitively) but would be persisted verbatim,
// so a later exact-match comparison (e.g. settings.Theme == "dark") would
// silently fail even though the value was accepted.
func normalizeEnum(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func applySettingsPatch(settings *models.UserSettings, patch map[string]interface{}) {
	applyUISettings(settings, patch)
	applyNotificationSettings(settings, patch)
	applyPrivacySettings(settings, patch)
	applyAdvancedSettings(settings, patch)
}

func applyUISettings(settings *models.UserSettings, patch map[string]interface{}) {
	if v, ok := patch["theme"].(string); ok {
		settings.Theme = normalizeEnum(v)
	}
	if v, ok := patch["fontSize"].(string); ok {
		settings.FontSize = normalizeEnum(v)
	}
	if v, ok := patch["reducedMotion"].(bool); ok {
		settings.ReducedMotion = v
	}
	if v, ok := patch["highContrast"].(bool); ok {
		settings.HighContrast = v
	}
	if v, ok := patch["compactMode"].(bool); ok {
		settings.CompactMode = v
	}
	if v, ok := patch["efficiencyMode"].(bool); ok {
		settings.EfficiencyMode = v
	}
	if v, ok := patch["language"].(string); ok {
		settings.Language = normalizeEnum(v)
	}
	if v, ok := patch["numberFormat"].(string); ok {
		settings.NumberFormat = normalizeEnum(v)
	}
}

func applyNotificationSettings(settings *models.UserSettings, patch map[string]interface{}) {
	if v, ok := patch["notificationsEnabled"].(bool); ok {
		settings.NotificationsEnabled = v
	}
	if v, ok := patch["studyReminders"].(bool); ok {
		settings.StudyReminders = v
	}
	if v, ok := patch["emailNotifications"].(bool); ok {
		settings.EmailNotifications = v
	}
	if v, ok := patch["pushNotifications"].(bool); ok {
		settings.PushNotifications = v
	}
}

func applyPrivacySettings(settings *models.UserSettings, patch map[string]interface{}) {
	if v, ok := patch["profileVisibility"].(string); ok {
		settings.ProfileVisibility = normalizeEnum(v)
	}
	if v, ok := patch["showOnlineStatus"].(bool); ok {
		settings.ShowOnlineStatus = v
	}
	if v, ok := patch["showProgress"].(bool); ok {
		settings.ShowProgress = v
	}
}

func applyAdvancedSettings(settings *models.UserSettings, patch map[string]interface{}) {
	applyReminderSettings(settings, patch)
	applyReportAndAlertSettings(settings, patch)
	applyChannelSettings(settings, patch)
	applyQuietHoursAndSoundSettings(settings, patch)
}

func applyReminderSettings(settings *models.UserSettings, patch map[string]interface{}) {
	if v, ok := patch["taskReminders"].(bool); ok {
		settings.TaskReminders = v
	}
	if v, ok := patch["taskReminderTime"].(string); ok {
		settings.TaskReminderTime = v
	}
	if v, ok := patch["dailyGoalReminders"].(bool); ok {
		settings.DailyGoalReminders = v
	}
	if v, ok := patch["examReminders"].(bool); ok {
		settings.ExamReminders = v
	}
	if v, ok := patch["examReminderDays"].(float64); ok {
		settings.ExamReminderDays = int(v)
	}
	if v, ok := patch["deadlineReminders"].(bool); ok {
		settings.DeadlineReminders = v
	}
}

func applyReportAndAlertSettings(settings *models.UserSettings, patch map[string]interface{}) {
	if v, ok := patch["progressReports"].(bool); ok {
		settings.ProgressReports = v
	}
	if v, ok := patch["weeklyReport"].(bool); ok {
		settings.WeeklyReport = v
	}
	if v, ok := patch["achievementAlerts"].(bool); ok {
		settings.AchievementAlerts = v
	}
	if v, ok := patch["commentNotifications"].(bool); ok {
		settings.CommentNotifications = v
	}
	if v, ok := patch["mentionNotifications"].(bool); ok {
		settings.MentionNotifications = v
	}
}

func applyChannelSettings(settings *models.UserSettings, patch map[string]interface{}) {
	if v, ok := patch["pushEnabled"].(bool); ok {
		settings.PushEnabled = v
	}
	if v, ok := patch["emailEnabled"].(bool); ok {
		settings.EmailEnabled = v
	}
	if v, ok := patch["smsEnabled"].(bool); ok {
		settings.SmsEnabled = v
	}
}

func applyQuietHoursAndSoundSettings(settings *models.UserSettings, patch map[string]interface{}) {
	if v, ok := patch["quietHoursEnabled"].(bool); ok {
		settings.QuietHoursEnabled = v
	}
	if v, ok := patch["quietHoursStart"].(string); ok {
		settings.QuietHoursStart = v
	}
	if v, ok := patch["quietHoursEnd"].(string); ok {
		settings.QuietHoursEnd = v
	}
	if v, ok := patch["soundEnabled"].(bool); ok {
		settings.SoundEnabled = v
	}
	if v, ok := patch["vibrationEnabled"].(bool); ok {
		settings.VibrationEnabled = v
	}
}
