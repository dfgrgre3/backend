package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStorageRoundTrip(t *testing.T) {
	baseDir := t.TempDir()
	store, err := NewLocalStorage(baseDir, "/uploads")
	if err != nil {
		t.Fatalf("NewLocalStorage() error = %v", err)
	}

	content := []byte("hello local storage")
	url, err := store.Upload(context.Background(), "test.txt", bytes.NewReader(content), int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	if url != "/uploads/test.txt" {
		t.Fatalf("Upload() URL = %q, want %q", url, "/uploads/test.txt")
	}

	filePath := filepath.Join(baseDir, "test.txt")
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("expected file to exist at %s: %v", filePath, err)
	}

	rc, err := store.Download(context.Background(), "test.txt")
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if !bytes.Equal(data, content) {
		t.Fatalf("Download() data = %q, want %q", string(data), string(content))
	}
}
