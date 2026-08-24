package bootstrap

import (
	"log"
	"os"

	"thanawy-backend/internal/infrastructure/config"
	"thanawy-backend/internal/infrastructure/storage"
)

func initStorage(cfg *config.Config) {
	if cfg.StorageType == "local" {
		baseDir := os.Getenv("STORAGE_PATH")
		if baseDir == "" {
			baseDir = "./uploads"
		}
		publicURL := os.Getenv("STORAGE_PUBLIC_URL")
		storageSvc, err := storage.NewLocalStorage(baseDir, publicURL)
		if err != nil {
			log.Fatalf("Failed to initialize local storage: %v", err)
		}
		storage.GlobalStorage = storageSvc
		log.Printf("Storage initialized with local provider at %s", baseDir)
		return
	}

	if cfg.StorageType != "s3" {
		log.Fatalf("Unsupported storage provider %q. Cloud storage (s3) is required.", cfg.StorageType)
	}
	if cfg.S3.Endpoint == "" {
		log.Fatal("S3_ENDPOINT is required for cloud storage")
	}

	storageSvc, err := storage.NewS3Storage(
		cfg.S3.Endpoint,
		cfg.S3.AccessKey,
		cfg.S3.SecretKey,
		cfg.S3.Bucket,
		cfg.S3.Region,
		cfg.S3.UseSSL,
		cfg.S3.PublicURL,
	)
	if err != nil {
		log.Fatalf("Failed to initialize S3 storage: %v", err)
	}
	storage.GlobalStorage = storageSvc
	log.Println("Storage initialized with S3 provider (Cloudflare R2)")
}
