package protected

import "testing"

func TestBackupCodesAreStoredAsJSON(t *testing.T) {
	codes := []string{"a", "b"}
	encoded := encodeBackupCodes(codes)
	if encoded != `["a","b"]` {
		t.Fatalf("unexpected encoded backup codes: %s", encoded)
	}
	decoded := decodeBackupCodes(encoded)
	if len(decoded) != 2 || decoded[0] != "a" || decoded[1] != "b" {
		t.Fatalf("unexpected decoded backup codes: %#v", decoded)
	}
}

func TestBackupCodesReadLegacyCommaSeparatedValue(t *testing.T) {
	decoded := decodeBackupCodes("a,b,")
	if len(decoded) != 2 || decoded[0] != "a" || decoded[1] != "b" {
		t.Fatalf("unexpected legacy decoded backup codes: %#v", decoded)
	}
}
