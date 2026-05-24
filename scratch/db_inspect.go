package main

import (
	"log"
	"thanawy-backend/internal/config"
	"thanawy-backend/internal/db"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	cfg := config.Load()
	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// 1. List all types in public schema
	var types []string
	err = database.Raw(`
		SELECT typname 
		FROM pg_type 
		JOIN pg_namespace ON pg_namespace.oid = pg_type.typnamespace 
		WHERE nspname = 'public'
		ORDER BY typname
	`).Scan(&types).Error
	if err != nil {
		log.Fatalf("Failed to list types: %v", err)
	}
	log.Printf("Types in public schema: %v", types)

	// 2. List all tables
	var tables []string
	err = database.Raw(`
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = 'public'
		ORDER BY table_name
	`).Scan(&tables).Error
	if err != nil {
		log.Fatalf("Failed to list tables: %v", err)
	}
	log.Printf("Tables in public schema: %v", tables)
}
