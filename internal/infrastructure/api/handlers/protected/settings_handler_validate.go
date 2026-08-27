package protected

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// validateSettingsPatch validates the incoming preferences patch and returns an
// error describing the first invalid field/value encountered. Unknown keys are
// intentionally ignored to stay forward-compatible with settings added later,
// but every recognized key must carry a value of the expected type and within
// the allowed range/set.
func validateSettingsPatch(patch map[string]interface{}) error {
	// String enum fields.
	for key, allowed := range map[string][]string{
		"theme":        {"light", "dark", "system"},
		"fontSize":     {"small", "medium", "large"},
		"language":     {"ar", "en"},
		"numberFormat": {"english", "arabic"},
		// "friends", not "contacts": matches the only three options the
		// privacy settings UI actually offers (settings/privacy/page.tsx).
		"profileVisibility": {"public", "private", "friends"},
		"allowMessages":     {"everyone", "friends", "none"},
	} {
		if raw, ok := patch[key]; ok {
			val, isStr := raw.(string)
			if !isStr || !oneOfEnum(strings.ToLower(strings.TrimSpace(val)), allowed) {
				return fmt.Errorf("invalid value for '%s': must be one of %v", key, allowed)
			}
		}
	}

	// Boolean fields: if present, they must be an actual JSON boolean.
	for _, key := range []string{
		"reducedMotion", "highContrast", "compactMode", "efficiencyMode",
		"notificationsEnabled", "studyReminders", "emailNotifications", "pushNotifications",
		"taskReminders", "dailyGoalReminders", "examReminders", "deadlineReminders",
		"progressReports", "weeklyReport", "achievementAlerts", "commentNotifications", "mentionNotifications",
		"pushEnabled", "emailEnabled", "smsEnabled", "quietHoursEnabled",
		"soundEnabled", "vibrationEnabled", "showOnlineStatus", "showProgress",
		"showLastSeen", "showAchievements", "allowFriendRequests",
		"dataCollection", "personalization", "analytics",
	} {
		if raw, ok := patch[key]; ok {
			if _, isBool := raw.(bool); !isBool {
				return fmt.Errorf("invalid value for '%s': expected a boolean (true/false)", key)
			}
		}
	}

	// Quiet hours must be a valid HH:mm time.
	for _, key := range []string{"quietHoursStart", "quietHoursEnd"} {
		if raw, ok := patch[key]; ok {
			val, isStr := raw.(string)
			if !isStr || !validHHMM(val) {
				return fmt.Errorf("invalid value for '%s': expected time in HH:mm format", key)
			}
		}
	}

	// Task reminder time is expressed in whole minutes (default: "30").
	if raw, ok := patch["taskReminderTime"]; ok {
		val, isStr := raw.(string)
		if !isStr {
			return fmt.Errorf("invalid value for 'taskReminderTime': expected minutes as a string")
		}
		mins, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil || mins < 1 || mins > 1440 {
			return fmt.Errorf("invalid value for 'taskReminderTime': expected minutes between 1 and 1440")
		}
	}

	// Exam reminders must be a whole number of days within a sensible range.
	if raw, ok := patch["examReminderDays"]; ok {
		days, isNum := raw.(float64)
		if !isNum || days != float64(int(days)) || days < 0 || days > 30 {
			return fmt.Errorf("invalid value for 'examReminderDays': expected an integer between 0 and 30")
		}
	}

	return nil
}

// oneOfEnum reports whether v matches any of the allowed values.
func oneOfEnum(v string, allowed []string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

// validHHMM reports whether v parses as a 24-hour clock time in strict HH:mm format.
// The value must be exactly 5 characters (e.g. "07:00", not "7:00") after trimming.
func validHHMM(v string) bool {
	s := strings.TrimSpace(v)
	if len(s) != 5 {
		return false
	}
	_, err := time.Parse("15:04", s)
	return err == nil
}
