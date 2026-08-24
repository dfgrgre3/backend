package main

import (
	"log"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	_ = godotenv.Load(".env.local")
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Get database URL directly without any config validation
	databaseURL := getDatabaseURL()
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	// Initialize Database directly using GORM - no config, no storage validation
	database, err := connectDatabase(databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Checking/Applying SQL database migrations...")
	if err := runSQLMigrations(database); err != nil {
		log.Fatalf("CRITICAL: SQL migrations failed: %v", err)
	}
	log.Println("SQL database migrations applied successfully.")

	log.Println("Migration process completed.")
}
