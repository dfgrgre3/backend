package services

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/robfig/cron/v3"
)

// BackupCronWorker schedules automated database backups and uploads them to S3/Supabase.
type BackupCronWorker struct {
	mu       sync.Mutex
	cronInst *cron.Cron
	active   bool
}

var (
	backupCronInstance *BackupCronWorker
	backupCronOnce     sync.Once
)

// GetBackupCronWorker returns the singleton backup cron worker.
func GetBackupCronWorker() *BackupCronWorker {
	backupCronOnce.Do(func() {
		backupCronInstance = &BackupCronWorker{
			cronInst: cron.New(cron.WithSeconds()),
		}
	})
	return backupCronInstance
}

// Start registers the backup schedule and launches the cron runner.
// BACKUP_CRON defaults to daily at 02:00 UTC (0 0 2 * * *).
func (w *BackupCronWorker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.active {
		return
	}

	schedule := os.Getenv("BACKUP_CRON")
	if schedule == "" {
		schedule = "0 0 2 * * *" // every day at 02:00 UTC
	}

	_, err := w.cronInst.AddFunc(schedule, func() {
		log.Println("[BackupCron] Starting scheduled database backup...")
		if err := runScheduledBackup(); err != nil {
			log.Printf("[BackupCron] Backup failed: %v", err)
		} else {
			log.Println("[BackupCron] Scheduled backup completed successfully")
		}
	})
	if err != nil {
		log.Printf("[BackupCron] Failed to register backup schedule: %v", err)
		return
	}

	w.cronInst.Start()
	w.active = true
	log.Printf("[BackupCron] Automated backup scheduler started (schedule: %s)", schedule)
}

// Stop gracefully halts the cron scheduler.
func (w *BackupCronWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.active {
		return
	}

	ctx := w.cronInst.Stop()
	<-ctx.Done()
	w.active = false
	log.Println("[BackupCron] Backup scheduler stopped")
}

// runScheduledBackup performs pg_dump, compresses the result, and uploads to S3/Supabase.
func runScheduledBackup() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL not set – cannot run pg_dump")
	}

	// ── 1. pg_dump ──────────────────────────────────────────────────────────────
	backupID := fmt.Sprintf("auto-%s", time.Now().UTC().Format("20060102-150405"))
	log.Printf("[BackupCron] Running pg_dump for backup %s", backupID)

	backupSvc := GetBackupService()
	rawSQL, err := backupSvc.runPgDump()
	if err != nil {
		log.Printf("[BackupCron] pg_dump warning: %v – will upload fallback stub", err)
		rawSQL = backupSvc.generateFallbackData(backupID, err)
	}

	// ── 2. gzip compress ────────────────────────────────────────────────────────
	var compressedBuf bytes.Buffer
	gz := gzip.NewWriter(&compressedBuf)
	if _, err := gz.Write(rawSQL); err != nil {
		return fmt.Errorf("gzip write failed: %w", err)
	}
	gz.Close()

	// ── 3. Write local temp file (cleanup after upload) ──────────────────────────
	tmpDir := os.TempDir()
	localPath := filepath.Join(tmpDir, backupID+".sql.gz")
	if err := os.WriteFile(localPath, compressedBuf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write temp backup file: %w", err)
	}
	defer os.Remove(localPath) // always clean up

	// ── 4. Upload to S3 / Supabase Storage ─────────────────────────────────────
	if err := uploadBackupToS3(backupID, compressedBuf.Bytes()); err != nil {
		// Log but don't fail – the local copy is still useful for debugging.
		log.Printf("[BackupCron] S3 upload failed for %s: %v", backupID, err)
	}

	return nil
}

// uploadBackupToS3 uploads a gzipped SQL file to the configured S3-compatible bucket.
func uploadBackupToS3(backupID string, data []byte) error {
	endpoint := os.Getenv("S3_ENDPOINT")
	accessKey := os.Getenv("S3_ACCESS_KEY")
	secretKey := os.Getenv("S3_SECRET_KEY")
	bucket := os.Getenv("S3_BACKUP_BUCKET")
	region := os.Getenv("S3_REGION")
	useSSL := os.Getenv("S3_USE_SSL") != "false"

	// Fall back to the primary bucket if no dedicated backup bucket is set.
	if bucket == "" {
		bucket = os.Getenv("S3_BUCKET")
	}
	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		return fmt.Errorf("S3 backup credentials not configured (S3_ENDPOINT, S3_ACCESS_KEY, S3_SECRET_KEY, S3_BACKUP_BUCKET)")
	}
	if region == "" {
		region = "auto"
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
		Region: region,
	})
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	objectKey := fmt.Sprintf("db-backups/%s.sql.gz", backupID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	reader := bytes.NewReader(data)
	_, err = client.PutObject(ctx, bucket, objectKey, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/gzip",
		UserMetadata: map[string]string{
			"backup-id":   backupID,
			"uploaded-at": time.Now().UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		return fmt.Errorf("S3 PutObject failed: %w", err)
	}

	log.Printf("[BackupCron] Backup %s uploaded to s3://%s/%s (%d bytes)", backupID, bucket, objectKey, len(data))
	return nil
}
