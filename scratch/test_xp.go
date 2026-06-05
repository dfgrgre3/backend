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
		log.Fatalf("Connect failed: %v", err)
	}

	var nullCount int64
	database.Table("User").Where("total_xp IS NULL OR study_xp IS NULL OR quest_xp IS NULL OR task_xp IS NULL OR exam_xp IS NULL OR challenge_xp IS NULL OR level IS NULL").Count(&nullCount)
	log.Printf("Number of users with NULL XP/level fields: %d", nullCount)
}
