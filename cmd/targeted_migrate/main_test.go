package main

import (
	"reflect"
	"testing"
)

func TestSplitSQL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Simple statements",
			input:    "SELECT 1; SELECT 2;",
			expected: []string{"SELECT 1;", "SELECT 2;"},
		},
		{
			name:     "With comments",
			input:    "SELECT 1; -- line comment\nSELECT 2; /* block comment */ SELECT 3;",
			expected: []string{"SELECT 1;", "SELECT 2;", "SELECT 3;"},
		},
		{
			name:     "Semicolon inside single quote",
			input:    "SELECT 'hello; world'; SELECT 2;",
			expected: []string{"SELECT 'hello; world';", "SELECT 2;"},
		},
		{
			name:     "Semicolon inside double quote",
			input:    `SELECT "hello; world"; SELECT 2;`,
			expected: []string{`SELECT "hello; world";`, "SELECT 2;"},
		},
		{
			name:     "Dollar quote",
			input:    "SELECT $$hello; world$$; SELECT 2;",
			expected: []string{"SELECT $$hello; world$$;", "SELECT 2;"},
		},
		{
			name:     "Named dollar quote",
			input:    "SELECT $tag$hello; world$tag$; SELECT 2;",
			expected: []string{"SELECT $tag$hello; world$tag$;", "SELECT 2;"},
		},
		{
			name:     "Escaped single quote",
			input:    "SELECT 'hello''world'; SELECT 2;",
			expected: []string{"SELECT 'hello''world';", "SELECT 2;"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitSQL(tc.input)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("splitSQL(%q) = %v; want %v", tc.input, got, tc.expected)
			}
		})
	}
}
