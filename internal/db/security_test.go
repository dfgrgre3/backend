package db

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestValidateMagicNumberRejectsGenericBinary(t *testing.T) {
	input := bytes.NewReader([]byte{0x00, 0x01, 0x02, 0x03})

	valid, detected, err := ValidateMagicNumber(input, []string{"application/octet-stream"})
	if err != nil {
		t.Fatalf("ValidateMagicNumber returned an error: %v", err)
	}
	if valid {
		t.Fatalf("generic binary content must be rejected, detected %q", detected)
	}
}

func TestValidateMagicNumberRejectsArbitraryZIP(t *testing.T) {
	input := zipFixture(t, map[string]string{"payload.exe": "not an office document"})

	valid, detected, err := ValidateMagicNumber(bytes.NewReader(input), []string{
		"application/zip",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	})
	if err != nil {
		t.Fatalf("ValidateMagicNumber returned an error: %v", err)
	}
	if valid {
		t.Fatalf("arbitrary ZIP content must be rejected, detected %q", detected)
	}
}

func TestValidateMagicNumberAcceptsMatchingOpenXMLDocument(t *testing.T) {
	mime := "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	input := zipFixture(t, map[string]string{
		"[Content_Types].xml": `<Types><Override ContentType="` + mime + `"/></Types>`,
		"word/document.xml":   "<document/>",
	})

	valid, detected, err := ValidateMagicNumber(bytes.NewReader(input), []string{mime})
	if err != nil {
		t.Fatalf("ValidateMagicNumber returned an error: %v", err)
	}
	if !valid || detected != mime {
		t.Fatalf("matching OpenXML document should be accepted; valid=%v detected=%q", valid, detected)
	}
}

func zipFixture(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for name, contents := range files {
		writer, err := archive.Create(name)
		if err != nil {
			t.Fatalf("create ZIP entry: %v", err)
		}
		if _, err := writer.Write([]byte(contents)); err != nil {
			t.Fatalf("write ZIP entry: %v", err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close ZIP fixture: %v", err)
	}
	return buffer.Bytes()
}
