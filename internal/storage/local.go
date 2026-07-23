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

// LocalStorage implements the storage interface using the local filesystem.
type LocalStorage struct {
	baseDir string
	publicURL string
}

func NewLocalStorage(baseDir, publicURL string) (*LocalStorage, error) {
	cleanBaseDir := strings.TrimSpace(baseDir)
	if cleanBaseDir == "" {
		return nil, fmt.Errorf("local storage base dir is required")
	}

	if err := os.MkdirAll(cleanBaseDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create local storage dir: %w", err)
	}

	return &LocalStorage{
		baseDir:  cleanBaseDir,
		publicURL: strings.TrimSuffix(publicURL, "/"),
	}, nil
}

func (s *LocalStorage) Upload(ctx context.Context, filename string, content io.Reader, size int64, contentType string) (string, error) {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
	}

	targetPath := filepath.Join(s.baseDir, filepath.Clean(strings.TrimPrefix(filename, "/")))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", fmt.Errorf("failed to create local upload directory: %w", err)
	}

	file, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to create local file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, content); err != nil {
		return "", fmt.Errorf("failed to write local file: %w", err)
	}

	return s.GetURL(ctx, filename)
}

func (s *LocalStorage) GetURL(ctx context.Context, filename string) (string, error) {
	if s.publicURL != "" {
		return fmt.Sprintf("%s/%s", s.publicURL, strings.TrimPrefix(filename, "/")), nil
	}
	return fmt.Sprintf("/uploads/%s", strings.TrimPrefix(filename, "/")), nil
}

func (s *LocalStorage) Delete(ctx context.Context, filename string) error {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	targetPath := filepath.Join(s.baseDir, filepath.Clean(strings.TrimPrefix(filename, "/")))
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove local file: %w", err)
	}
	return nil
}

func (s *LocalStorage) List(ctx context.Context, prefix string) ([]string, error) {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	var files []string
	root := filepath.Join(s.baseDir, filepath.Clean(strings.TrimPrefix(prefix, "/")))
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(s.baseDir, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return files, nil
}

func (s *LocalStorage) Download(ctx context.Context, filename string) (io.ReadCloser, error) {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	targetPath := filepath.Join(s.baseDir, filepath.Clean(strings.TrimPrefix(filename, "/")))
	file, err := os.Open(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open local file: %w", err)
	}
	return file, nil
}

func (s *LocalStorage) GeneratePresignedUploadURL(ctx context.Context, filename string, contentType string, expiry time.Duration) (string, error) {
	return s.GetURL(ctx, filename)
}
