package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalStorage implements Storage interface for local file storage
type LocalStorage struct {
	baseDir   string
	publicURL string
}

// NewLocalStorage creates a new instance of LocalStorage
func NewLocalStorage(baseDir, publicURL string) (*LocalStorage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}
	return &LocalStorage{
		baseDir:   baseDir,
		publicURL: strings.TrimSuffix(publicURL, "/"),
	}, nil
}

// Upload saves a file and returns its access URL
func (l *LocalStorage) Upload(ctx context.Context, filename string, content io.Reader, size int64, contentType string) (string, error) {
	cleaned := filepath.Clean(filename)
	if strings.HasPrefix(cleaned, "..") || strings.HasPrefix(cleaned, "/") || strings.HasPrefix(cleaned, "\\") {
		return "", fmt.Errorf("invalid path traversal attempt")
	}

	targetPath := filepath.Join(l.baseDir, cleaned)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create target subdirectory: %w", err)
	}

	dst, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, content); err != nil {
		return "", fmt.Errorf("failed to write file content: %w", err)
	}

	return l.GetURL(ctx, filename)
}

// GetURL returns a public URL for a file
func (l *LocalStorage) GetURL(ctx context.Context, filename string) (string, error) {
	return fmt.Sprintf("%s/%s", l.publicURL, filepath.ToSlash(filename)), nil
}

// Delete removes a file from storage
func (l *LocalStorage) Delete(ctx context.Context, filename string) error {
	cleaned := filepath.Clean(filename)
	if strings.HasPrefix(cleaned, "..") || strings.HasPrefix(cleaned, "/") || strings.HasPrefix(cleaned, "\\") {
		return fmt.Errorf("invalid path traversal attempt")
	}
	return os.Remove(filepath.Join(l.baseDir, cleaned))
}

// List returns a list of files with the given prefix
func (l *LocalStorage) List(ctx context.Context, prefix string) ([]string, error) {
	var files []string
	searchPath := filepath.Join(l.baseDir, prefix)
	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, err := filepath.Rel(l.baseDir, path)
			if err == nil {
				files = append(files, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	return files, err
}

// Download returns the content of a file
func (l *LocalStorage) Download(ctx context.Context, filename string) (io.ReadCloser, error) {
	cleaned := filepath.Clean(filename)
	if strings.HasPrefix(cleaned, "..") || strings.HasPrefix(cleaned, "/") || strings.HasPrefix(cleaned, "\\") {
		return nil, fmt.Errorf("invalid path traversal attempt")
	}
	return os.Open(filepath.Join(l.baseDir, cleaned))
}

// GeneratePresignedUploadURL returns a public URL for a file (direct upload is simulated as direct link in local)
func (l *LocalStorage) GeneratePresignedUploadURL(ctx context.Context, filename string, contentType string, expiry time.Duration) (string, error) {
	return l.GetURL(ctx, filename)
}
