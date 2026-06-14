package main

import (
	"log"
	"thanawy-backend/internal/config"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"

	"github.com/google/uuid"
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

	// Try inserting a dummy LessonAttachment
	log.Println("Attempting to insert a test subtopic...")
	/*
	subTopic := models.SubTopic{
		ID:      uuid.New().String(),
		TopicID: "00000000-0000-0000-0000-000000000000", // dummy or let GORM handle if possible, but let's see. Wait, TopicID is a FK?
		Title:   "Test Subtopic for Attachment",
	}
	*/
	
	// Let's check first if we can insert it or if there is a real topic we can use, or just insert the attachment directly.
	// Since FK constraint might fail, let's find an existing subtopic or bypass FK constraints by disabling trigger check / using transaction.
	// Wait, does LessonAttachment have a FK constraint in DB? We checked earlier, there is no FOREIGN KEY constraint on LessonAttachment in baseline schema!
	// So we can insert LessonAttachment with ANY sub_topic_id!
	
	attachment := models.LessonAttachment{
		ID:         uuid.New().String(),
		SubTopicID: uuid.New().String(),
		Title:      "Test Attachment PDF",
		FileUrl:    "https://example.com/test.pdf",
		FileType:   "pdf",
		FileSize:   1024,
	}

	log.Printf("Inserting attachment: %+v", attachment)
	if err := database.Create(&attachment).Error; err != nil {
		log.Fatalf("INSERT FAILED: %v", err)
	}
	log.Println("INSERT SUCCEEDED!")
}
