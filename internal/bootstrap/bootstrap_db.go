package bootstrap

import (
	"log"
	db "thanawy-backend/internal/infrastructure/database"
	"time"

	"thanawy-backend/internal/infrastructure/config"

	"gorm.io/gorm"
)

func connectDatabaseWithRetry(cfg *config.Config) (*gorm.DB, error) {
	attempts := getEnvInt("DB_CONNECT_ATTEMPTS", 5)
	delay := time.Duration(getEnvInt("DB_CONNECT_RETRY_SECONDS", 2)) * time.Second

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		database, err := db.ConnectWithWriteDSN(cfg.DatabaseURL, cfg.DatabaseWriteURL)
		if err == nil {
			if attempt > 1 {
				log.Printf("Database connected after %d attempts", attempt)
			}
			return database, nil
		}

		lastErr = err
		if attempt == attempts {
			break
		}

		log.Printf("Database is not ready yet (attempt %d/%d): %v. Retrying in %s...", attempt, attempts, err, delay)
		time.Sleep(delay)
	}

	return nil, lastErr
}
