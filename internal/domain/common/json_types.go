package models

import (
	"database/sql/driver"
	"encoding/json"
	"net/netip"
	"strings"
)

// JSONText is a string value persisted into a jsonb column.
//
// The AuditLog table stores `changes`/`metadata` as jsonb, but the Go model
// keeps them as strings. Postgres rejects an empty string ("") or any
// non-JSON text for a jsonb column with SQLSTATE 22P02 ("invalid input
// syntax for type json"), which used to silently drop every audit insert.
//
// Value() therefore guarantees a jsonb-safe result:
//   - empty/whitespace  -> SQL NULL (column default applies when omitted)
//   - valid JSON        -> sent as-is
//   - any other text    -> wrapped into a JSON string so no data is lost
type JSONText string

// Value implements driver.Valuer.
func (j JSONText) Value() (driver.Value, error) {
	s := strings.TrimSpace(string(j))
	if s == "" {
		return nil, nil
	}
	if json.Valid([]byte(s)) {
		return s, nil
	}
	// Arbitrary text (e.g. free-form notes): encode as a JSON string so the
	// insert always succeeds and the content is preserved.
	encoded, err := json.Marshal(string(j))
	if err != nil {
		return nil, nil
	}
	return string(encoded), nil
}

// Scan implements sql.Scanner.
func (j *JSONText) Scan(value any) error {
	if value == nil {
		*j = ""
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*j = JSONText(v)
	case string:
		*j = JSONText(v)
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return err
		}
		*j = JSONText(encoded)
	}
	return nil
}

// InetText is a string value persisted into an inet column.
//
// Postgres rejects an empty string or any value that is not a valid IP
// address / CIDR range for inet columns. Empty or unparseable values are
// stored as SQL NULL so a single bad client IP can never fail an insert.
type InetText string

// Value implements driver.Valuer.
func (i InetText) Value() (driver.Value, error) {
	s := strings.TrimSpace(string(i))
	if s == "" {
		return nil, nil
	}
	if _, err := netip.ParseAddr(s); err == nil {
		return s, nil
	}
	if _, err := netip.ParsePrefix(s); err == nil {
		return s, nil
	}
	return nil, nil
}

// Scan implements sql.Scanner.
func (i *InetText) Scan(value any) error {
	if value == nil {
		*i = ""
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*i = InetText(v)
	case string:
		*i = InetText(v)
	default:
		*i = InetText("")
	}
	return nil
}
