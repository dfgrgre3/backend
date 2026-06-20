package main

import (
	"log"
	"os"

	"thanawy-backend/internal/config"
	"thanawy-backend/internal/db"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load()

	cfg := config.Load()
	databaseURL := os.Getenv("DATABASE_URL_DIRECT")
	if databaseURL == "" {
		databaseURL = cfg.DatabaseURL
	}

	database, err := db.Connect(databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	log.Println("Deleting migration record for 0056_add_lesson_table...")
	if err := database.Exec(`
		DELETE FROM schema_migrations 
		WHERE id = '0056_add_lesson_table';
	`).Error; err != nil {
		log.Fatalf("Failed to delete migration record: %v", err)
	}

	log.Println("Migration record deleted successfully.")
}
