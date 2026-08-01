package main

import (
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	email := "ffyoussef12@gmail.com"
	password := "Khaled@2008"

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	userID := uuid.New().String()

	// 1. Raw SQL Insert or Update into "User" table
	var existingID string
	err = db.Raw(`SELECT id FROM public."User" WHERE email = ? AND deleted_at IS NULL`, email).Scan(&existingID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Fatalf("Query user failed: %v", err)
	}

	if existingID != "" {
		userID = existingID
		err = db.Exec(`UPDATE public."User" SET role = 'ADMIN', status = 'ACTIVE', email_verified = true, password_hash = ? WHERE id = ?`, string(hashedPassword), userID).Error
		if err != nil {
			log.Fatalf("Update User failed: %v", err)
		}
		log.Println("Updated User table")
	} else {
		err = db.Exec(`INSERT INTO public."User" (id, email, password_hash, role, status, email_verified, created_at, updated_at) VALUES (?, ?, ?, 'ADMIN', 'ACTIVE', true, NOW(), NOW())`, userID, email, string(hashedPassword)).Error
		if err != nil {
			log.Fatalf("Insert User failed: %v", err)
		}
		log.Println("Inserted into User table")
	}

	// 2. Raw SQL Upsert into "UserCredential" table
	err = db.Exec(`INSERT INTO public."UserCredential" (user_id, password_hash, created_at, updated_at) VALUES (?, ?, NOW(), NOW()) ON CONFLICT (user_id) DO UPDATE SET password_hash = EXCLUDED.password_hash, updated_at = NOW()`, userID, string(hashedPassword)).Error
	if err != nil {
		log.Fatalf("Upsert UserCredential failed: %v", err)
	}
	log.Println("Updated UserCredential table")

	fmt.Printf("Successfully set user %s (ID: %s) as ADMIN with password %s!\n", email, userID, password)
}
