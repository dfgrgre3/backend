package models

import (
	"database/sql/driver"
	"testing"
)

func jsonValue(t *testing.T, v driver.Valuer) any {
	t.Helper()
	got, err := v.Value()
	if err != nil {
		t.Fatalf("Value() error: %v", err)
	}
	return got
}

func TestJSONTextValue(t *testing.T) {
	tests := []struct {
		name  string
		input JSONText
		want  any // nil means SQL NULL
	}{
		{"empty string becomes NULL", "", nil},
		{"whitespace becomes NULL", "   ", nil},
		{"valid object passes through", `{"a":1}`, `{"a":1}`},
		{"valid array passes through", `[1,2]`, `[1,2]`},
		{"valid string passes through", `"hello"`, `"hello"`},
		{"invalid JSON wrapped as JSON string", `hello world`, `"hello world"`},
		{"unterminated object wrapped", `{"a":`, `"{\"a\":"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jsonValue(t, tt.input)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("want NULL, got %q", got)
				}
				return
			}
			if got != tt.want.(string) {
				t.Fatalf("want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestJSONTextScan(t *testing.T) {
	var j JSONText
	if err := j.Scan(nil); err != nil || j != "" {
		t.Fatalf("nil scan: err=%v j=%q", err, j)
	}
	if err := j.Scan([]byte(`{"a":1}`)); err != nil || j != `{"a":1}` {
		t.Fatalf("bytes scan: err=%v j=%q", err, j)
	}
	if err := j.Scan(`[1]`); err != nil || j != `[1]` {
		t.Fatalf("string scan: err=%v j=%q", err, j)
	}
}

func TestInetTextValue(t *testing.T) {
	tests := []struct {
		name  string
		input InetText
		want  any
	}{
		{"empty becomes NULL", "", nil},
		{"whitespace becomes NULL", "  ", nil},
		{"ipv4 passes through", "127.0.0.1", "127.0.0.1"},
		{"ipv6 passes through", "::1", "::1"},
		{"cidr passes through", "10.0.0.0/8", "10.0.0.0/8"},
		{"garbage becomes NULL", "not-an-ip", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jsonValue(t, tt.input)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("want NULL, got %q", got)
				}
				return
			}
			if got != tt.want.(string) {
				t.Fatalf("want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestInetTextScan(t *testing.T) {
	var i InetText
	if err := i.Scan(nil); err != nil || i != "" {
		t.Fatalf("nil scan: err=%v i=%q", err, i)
	}
	if err := i.Scan([]byte("10.1.2.3")); err != nil || i != "10.1.2.3" {
		t.Fatalf("bytes scan: err=%v i=%q", err, i)
	}
}
