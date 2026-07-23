package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeCourseTagSlug(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		explicit *string
		expected string
	}{
		{name: "uses explicit slug", input: "Advanced Topics", explicit: ptr("custom-slug"), expected: "custom-slug"},
		{name: "falls back to name slug", input: "Advanced Topics", explicit: nil, expected: "advanced-topics"},
		{name: "trims whitespace", input: "  My Course  ", explicit: nil, expected: "my-course"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, normalizeCourseTagSlug(tt.input, tt.explicit))
		})
	}
}

func TestNormalizeReviewCommentStatus(t *testing.T) {
	assert.Equal(t, "PENDING", normalizeReviewCommentStatus(""))
	assert.Equal(t, "APPROVED", normalizeReviewCommentStatus("approved"))
	assert.Equal(t, "REJECTED", normalizeReviewCommentStatus("rejected"))
}

func TestNormalizeRelationType(t *testing.T) {
	assert.Equal(t, "related", normalizeRelationType(""))
	assert.Equal(t, "prerequisite", normalizeRelationType("prerequisite"))
	assert.Equal(t, "related", normalizeRelationType("RELATED"))
}
