//go:build ignore

package main

import (
	"log"
	"os"
	"path/filepath"
	"thanawy-backend/internal/config"
	"thanawy-backend/internal/db"

	"github.com/joho/godotenv"
)

func main() {
	// Load the env variables from the root .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found or failed to load: %v", err)
	}

	cfg := config.Load()
	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Ensuring FOREVER is in PlanInterval enum...")
	_ = database.Exec("ALTER TYPE \"PlanInterval\" ADD VALUE IF NOT EXISTS 'FOREVER'").Error

	migrationPath := filepath.Join("supabase", "migrations", "20260604_fix_missing_subscription_tables.sql")
	content, err := os.ReadFile(migrationPath)
	if err != nil {
		log.Fatalf("Failed to read migration file: %v", err)
	}

	log.Println("Executing migration...")
	err = database.Exec(string(content)).Error
	if err != nil {
		log.Fatalf("Failed to execute migration: %v", err)
	}

	log.Println("Migration applied successfully!")
}
