package protected

import (
	"strings"
	"testing"
)

func TestValidateSettingsPatch(t *testing.T) {
	tests := []struct {
		name    string
		patch   map[string]interface{}
		wantErr string // empty means valid
	}{
		{"empty patch is valid", map[string]interface{}{}, ""},
		{"valid theme", map[string]interface{}{"theme": "dark"}, ""},
		{"theme is case-insensitive", map[string]interface{}{"theme": "LIGHT"}, ""},
		{"invalid theme", map[string]interface{}{"theme": "neon"}, "invalid value for 'theme'"},
		{"invalid theme type", map[string]interface{}{"theme": 5}, "invalid value for 'theme'"},
		{"valid fontSize", map[string]interface{}{"fontSize": "large"}, ""},
		{"invalid fontSize", map[string]interface{}{"fontSize": "huge"}, "invalid value for 'fontSize'"},
		{"valid language", map[string]interface{}{"language": "en"}, ""},
		{"invalid language", map[string]interface{}{"language": "fr"}, "invalid value for 'language'"},
		{"valid numberFormat", map[string]interface{}{"numberFormat": "arabic"}, ""},
		{"invalid numberFormat", map[string]interface{}{"numberFormat": "swahili"}, "invalid value for 'numberFormat'"},
		{"valid profileVisibility", map[string]interface{}{"profileVisibility": "private"}, ""},
		{"invalid profileVisibility", map[string]interface{}{"profileVisibility": "everyone"}, "invalid value for 'profileVisibility'"},

		{"valid boolean", map[string]interface{}{"showProgress": false}, ""},
		{"invalid boolean type", map[string]interface{}{"showProgress": 1}, "expected a boolean"},
		{"invalid boolean string", map[string]interface{}{"reducedMotion": "yes"}, "expected a boolean"},

		{"valid quiet hours", map[string]interface{}{"quietHoursStart": "22:00", "quietHoursEnd": "07:30"}, ""},
		{"invalid quiet hours time", map[string]interface{}{"quietHoursStart": "25:99"}, "HH:mm"},
		{"invalid quiet hours type", map[string]interface{}{"quietHoursEnd": 12}, "HH:mm"},

		{"valid taskReminderTime", map[string]interface{}{"taskReminderTime": "30"}, ""},
		{"invalid taskReminderTime non numeric", map[string]interface{}{"taskReminderTime": "abc"}, "taskReminderTime"},
		{"invalid taskReminderTime too small", map[string]interface{}{"taskReminderTime": "0"}, "taskReminderTime"},
		{"invalid taskReminderTime too big", map[string]interface{}{"taskReminderTime": "99999"}, "taskReminderTime"},

		{"valid examReminderDays", map[string]interface{}{"examReminderDays": 3.0}, ""},
		{"invalid examReminderDays fractional", map[string]interface{}{"examReminderDays": 3.5}, "examReminderDays"},
		{"invalid examReminderDays negative", map[string]interface{}{"examReminderDays": -1.0}, "examReminderDays"},
		{"invalid examReminderDays too big", map[string]interface{}{"examReminderDays": 31.0}, "examReminderDays"},

		{"unknown keys ignored", map[string]interface{}{"someFutureSetting": "x", "theme": "light"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSettingsPatch(tt.patch)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestOneOfEnum(t *testing.T) {
	if !oneOfEnum("dark", []string{"light", "dark"}) {
		t.Fatal("expected 'dark' to be allowed")
	}
	if oneOfEnum("neon", []string{"light", "dark"}) {
		t.Fatal("expected 'neon' to be rejected")
	}
}

func TestValidHHMM(t *testing.T) {
	valid := []string{"00:00", "09:05", "23:59", " 07:00 "}
	for _, v := range valid {
		if !validHHMM(v) {
			t.Fatalf("expected %q to be a valid HH:mm", v)
		}
	}
	invalid := []string{"", "7:00", "25:00", "12:60", "abc"}
	for _, v := range invalid {
		if validHHMM(v) {
			t.Fatalf("expected %q to be invalid HH:mm", v)
		}
	}
}
