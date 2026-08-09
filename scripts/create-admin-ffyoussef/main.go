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
	if godotenv.Load(".env") != nil {
		if godotenv.Load("../../.env") != nil {
			log.Println("No .env file found, using system environment variables")
		}
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	email := "ffyoussef12@gmail.com"
	password := "Khaled@2008"

	// Hash the password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Check if user already exists
	var existingUser struct {
		ID string
	}
	result := db.Raw(`SELECT id FROM "User" WHERE email = ?`, email).Scan(&existingUser)

	if result.Error == nil && existingUser.ID != "" {
		// User exists, update password
		userID := existingUser.ID

		// Update UserCredential
		result = db.Exec(`UPDATE "UserCredential" SET password_hash = ? WHERE user_id = ?`, string(hash), userID)
		if result.Error != nil {
			log.Fatalf("Failed to update password: %v", result.Error)
		}

		// Ensure user has admin role
		result = db.Exec(`UPDATE "User" SET role = 'ADMIN', status = 'ACTIVE' WHERE id = ?`, userID)
		if result.Error != nil {
			log.Fatalf("Failed to update user role: %v", result.Error)
		}

		fmt.Printf("✅ Updated existing user to admin: %s\n", email)
	} else {
		// Create new user with password_hash in User table (for backward compatibility)
		userID := uuid.New().String()

		// Insert User (password_hash is now in UserCredential table)
		result = db.Exec(`INSERT INTO "User" (id, email, role, status, created_at, updated_at, version)
			VALUES (?, ?, 'ADMIN', 'ACTIVE', NOW(), NOW(), 1)`, userID, email)
		if result.Error != nil {
			log.Fatalf("Failed to create user: %v", result.Error)
		}

		// Also insert UserCredential for the new credential system
		result = db.Exec(`INSERT INTO "UserCredential" (id, user_id, password_hash, last_changed_at, created_at, updated_at)
			VALUES (?, ?, ?, NOW(), NOW(), NOW())`, uuid.New().String(), userID, string(hash))
		if result.Error != nil {
			log.Printf("Warning: Failed to create UserCredential (may not exist yet): %v", result.Error)
		}

		fmt.Printf("✅ Created new admin user: %s\n", email)
	}

	fmt.Println("Password: Khaled@2008")
}
