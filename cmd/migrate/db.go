package main

import (
	"log"
	"os"
	"time"

	db "thanawy-backend/internal/infrastructure/database"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func connectDatabase(dsn string) (*gorm.DB, error) {
	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
			SlowThreshold:             500 * time.Millisecond,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
		}),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := database.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetConnMaxLifetime(15 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	log.Println("Database connection established")
	return database, nil
}

func getDatabaseURL() string {
	// Try to use migration role for schema changes
	migrationDSN, err := db.GetMigrationDSN()
	if err == nil && migrationDSN != "" {
		log.Println("Attempting to use migration_user role for schema changes.")
		// Test if the role exists by trying to connect
		testDB, testErr := connectDatabase(migrationDSN)
		if testErr == nil {
			sqlDB, _ := testDB.DB()
			if sqlDB != nil {
				sqlDB.Close()
			}
			log.Println("Using migration_user role for schema changes.")
			return migrationDSN
		}
		log.Printf("Migration role not available, falling back to default connection: %v", testErr)
	}

	if directURL := os.Getenv("DATABASE_URL_DIRECT"); directURL != "" {
		log.Println("Using DATABASE_URL_DIRECT for migrations.")
		return directURL
	}
	if writeURL := os.Getenv("DATABASE_WRITE_DSN"); writeURL != "" {
		log.Println("Using DATABASE_WRITE_DSN for migrations.")
		return writeURL
	}
	return os.Getenv("DATABASE_URL")
}
