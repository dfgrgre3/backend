package main

import (
	"fmt"
	"log"
	"os"

	"thanawy-backend/internal/infrastructure/config"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load()

	cfg := config.Load()
	databaseURL := getDatabaseURL(cfg)
	database, err := db.Connect(databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// Find a user who has some records (e.g. any user)
	var userID string
	err = database.Raw(`SELECT id FROM "User" LIMIT 1`).Scan(&userID).Error
	if err != nil {
		log.Fatalf("Failed to query user: %v", err)
	}

	if userID == "" {
		log.Println("No users found in database.")
		return
	}

	newID := uuid.New().String()
	log.Printf("Testing ID update cascade for user: %s -> %s", userID, newID)

	tx := database.Begin()
	err = tx.Exec(`UPDATE "User" SET id = ? WHERE id = ?`, newID, userID).Error
	if err != nil {
		fmt.Printf("Cascade failed: %v\n", err)
	} else {
		fmt.Println("Cascade SUCCESS! The database foreign keys have ON UPDATE CASCADE.")
	}
	tx.Rollback()
}

func getDatabaseURL(cfg *config.Config) string {
	if directURL := os.Getenv("DATABASE_URL_DIRECT"); directURL != "" {
		return directURL
	}
	if cfg.DatabaseWriteURL != "" {
		return cfg.DatabaseWriteURL
	}
	return cfg.DatabaseURL
}
