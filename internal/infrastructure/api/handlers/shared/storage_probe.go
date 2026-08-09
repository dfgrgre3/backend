package shared

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"

	"thanawy-backend/internal/infrastructure/storage"
)

// ProbeStorage performs a round-trip upload/download/delete against the
// configured object storage to verify it is reachable and writable.
func ProbeStorage(ctx context.Context) error {
	if storage.GlobalStorage == nil {
		return fmt.Errorf("storage is not configured")
	}
	name := "__health_probe__/" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	payload := []byte("storage-health-probe")
	if _, err := storage.GlobalStorage.Upload(ctx, name, bytes.NewReader(payload), int64(len(payload)), "text/plain"); err != nil {
		return fmt.Errorf("storage upload probe failed: %w", err)
	}
	reader, err := storage.GlobalStorage.Download(ctx, name)
	if err != nil {
		_ = storage.GlobalStorage.Delete(ctx, name)
		return fmt.Errorf("storage download probe failed: %w", err)
	}
	if err := reader.Close(); err != nil {
		_ = storage.GlobalStorage.Delete(ctx, name)
		return fmt.Errorf("storage download probe close failed: %w", err)
	}
	if err := storage.GlobalStorage.Delete(ctx, name); err != nil {
		return fmt.Errorf("storage delete probe failed: %w", err)
	}
	return nil
}
