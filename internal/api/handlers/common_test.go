package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestIsDuplicateKeyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"normal error", errors.New("some random error"), false},
		{"duplicate key message", errors.New("pq: duplicate key value violates unique constraint"), true},
		{"unique constraint message", errors.New("UNIQUE constraint failed: UserSettings.user_id"), true},
		{"PostgreSQL code 23505", errors.New("ERROR: duplicate key value (SQLSTATE 23505)"), true},
		{"mixed case", errors.New("Duplicate Key Value"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDuplicateKeyError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStringOrEmpty(t *testing.T) {
	tests := []struct {
		name     string
		input    *string
		expected string
	}{
		{"nil pointer", nil, ""},
		{"empty string", ptr(""), ""},
		{"non-empty string", ptr("hello"), "hello"},
		{"whitespace string", ptr("  "), "  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringOrEmpty(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name     string
		inputs   []string
		expected string
	}{
		{"all empty", []string{"", "", ""}, ""},
		{"first non-empty", []string{"a", "b", "c"}, "a"},
		{"second non-empty", []string{"", "b", "c"}, "b"},
		{"last non-empty", []string{"", "", "c"}, "c"},
		{"single value", []string{"hello"}, "hello"},
		{"no values", []string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := firstNonEmpty(tt.inputs...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildSlug(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		explicit *string
		expected string
	}{
		{"basic name", "Hello World", nil, "hello-world"},
		{"explicit slug", "Hello World", ptr("custom-slug"), "custom-slug"},
		{"whitespace name", "  Test  ", nil, "test"},
		{"explicit empty", "Test", ptr(""), "test"},
		{"explicit whitespace", "Test", ptr("  "), "test"},
		{"arabic name", "كتب مدرسية", nil, "كتب-مدرسية"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSlug(tt.input, tt.explicit)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseNotificationsPagination_UsesCursorWhenBeforeProvided(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/notifications?limit=20&offset=15&before=2026-08-02T00:00:00Z", nil)

	limit, offset, before := parseNotificationsPagination(c)
	assert.Equal(t, 20, limit)
	assert.Equal(t, 0, offset)
	if assert.NotNil(t, before) {
		assert.Equal(t, "2026-08-02T00:00:00Z", before.Format(time.RFC3339))
	}
}

func TestAggregateUserCountRows(t *testing.T) {
	rows := []userAggregateCountRow{
		{UserID: "u1", Kind: "tasks", Count: 2},
		{UserID: "u1", Kind: "sessions", Count: 5},
		{UserID: "u1", Kind: "achievements", Count: 1},
		{UserID: "u2", Kind: "enrollments", Count: 3},
		{UserID: "u2", Kind: "tasks", Count: 1},
	}

	taskMap, sessionMap, achievementMap, enrollmentMap := aggregateUserCountRows(rows)

	assert.EqualValues(t, 2, taskMap["u1"])
	assert.EqualValues(t, 1, taskMap["u2"])
	assert.EqualValues(t, 5, sessionMap["u1"])
	assert.EqualValues(t, 1, achievementMap["u1"])
	assert.EqualValues(t, 3, enrollmentMap["u2"])
}

func ptr(s string) *string {
	return &s
}

func TestSafeCreate(t *testing.T) {
	t.Skip("Requires database connection - use integration tests")
}

func TestUpsertBy(t *testing.T) {
	t.Skip("Requires database connection - use integration tests")
}

func TestCreateOrAssign(t *testing.T) {
	t.Skip("Requires database connection - use integration tests")
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "id = ?", queryID)
	assert.Equal(t, "id = ?", idQuery)
	assert.Equal(t, "status = ?", statusQuery)
	assert.Equal(t, "2006-01-02", dateFormat)
	assert.Equal(t, "User not found", errUserNotFound)
	assert.Equal(t, "Authentication required", authRequired)
}

func TestIsDuplicateKeyError_RealWorldCases(t *testing.T) {
	realErrors := []string{
		`pq: duplicate key value violates unique constraint "uni_UserSettings_user_id"`,
		`UNIQUE constraint failed: UserSettings.user_id`,
		`ERROR: duplicate key value violates unique constraint "uq_category_slug_type" (SQLSTATE 23505)`,
		`duplicate key value violates unique constraint "uq_user_settings_user_id"`,
	}

	for _, msg := range realErrors {
		t.Run("real error", func(t *testing.T) {
			err := errors.New(msg)
			assert.True(t, IsDuplicateKeyError(err), "Should detect: %s", msg)
		})
	}
}

func TestStringOrEmpty_Dereference(t *testing.T) {
	val := "test"
	assert.Equal(t, "test", stringOrEmpty(&val))
	assert.Equal(t, "", stringOrEmpty(nil))
}

func TestFirstNonEmpty_Order(t *testing.T) {
	assert.Equal(t, "first", firstNonEmpty("first", "second"))
	assert.Equal(t, "second", firstNonEmpty("", "second"))
	assert.Equal(t, "third", firstNonEmpty("", "", "third"))
}

func TestBuildSlug_EdgeCases(t *testing.T) {
	assert.Equal(t, "", buildSlug("", nil))
	assert.Equal(t, "a-b-c", buildSlug("a b c", nil))
	assert.Equal(t, "test", buildSlug("TEST", nil))
	assert.Equal(t, "test", buildSlug("  TEST  ", nil))
}

func TestIsDuplicateKeyError_NonDuplicates(t *testing.T) {
	nonDuplicateErrors := []string{
		"connection refused",
		"table does not exist",
		"column not found",
		"permission denied",
		"timeout",
		"",
	}

	for _, msg := range nonDuplicateErrors {
		t.Run("non-duplicate", func(t *testing.T) {
			err := errors.New(msg)
			assert.False(t, IsDuplicateKeyError(err))
		})
	}
}

func TestStringOrEmpty_AllVariants(t *testing.T) {
	empty := ""
	space := " "
	tab := "\t"
	newline := "\n"

	assert.Equal(t, "", stringOrEmpty(&empty))
	assert.Equal(t, " ", stringOrEmpty(&space))
	assert.Equal(t, "\t", stringOrEmpty(&tab))
	assert.Equal(t, "\n", stringOrEmpty(&newline))
	assert.Equal(t, "", stringOrEmpty(nil))
}

func TestFirstNonEmpty_MixedTypes(t *testing.T) {
	assert.Equal(t, "0", firstNonEmpty("", "0"))
	assert.Equal(t, "false", firstNonEmpty("", "false"))
	assert.Equal(t, " ", firstNonEmpty("", " "))
}

func TestBuildSlug_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"numbers", "Test 123", "test-123"},
		{"special chars", "Hello! @#$", "hello!-@#$"},
		{"multiple spaces", "a  b  c", "a--b--c"},
		{"tabs", "a\tb", "a\tb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSlug(tt.input, nil)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsDuplicateKeyError_UniqueConstraintVariants(t *testing.T) {
	variants := []string{
		"unique constraint",
		"UNIQUE constraint",
		"Unique Constraint",
		"duplicate key",
		"Duplicate Key",
		"DUPLICATE KEY",
		"23505",
	}

	for _, variant := range variants {
		t.Run(variant, func(t *testing.T) {
			err := errors.New(variant)
			assert.True(t, IsDuplicateKeyError(err), "Should match: %s", variant)
		})
	}
}
