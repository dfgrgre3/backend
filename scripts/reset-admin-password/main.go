package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	if godotenv.Load(".env") != nil {
		// Try parent dirs too
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

	email := os.Getenv("DEFAULT_ADMIN_EMAIL")
	if email == "" {
		email = "admin@thanawy.com"
	}
	password := os.Getenv("DEFAULT_ADMIN_PASSWORD")
	if password == "" {
		log.Fatal("DEFAULT_ADMIN_PASSWORD not set")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// password_hash now lives in UserCredential table, not User
	var existingID string
	db.Raw(`SELECT id FROM "User" WHERE email = ? AND deleted_at IS NULL`, email).Scan(&existingID)

	if existingID == "" {
		log.Printf("No user found with email %s. Creating admin user...", email)
		result := db.Exec(`INSERT INTO "User" (id, email, role, status, created_at, updated_at, version)
			VALUES (gen_random_uuid(), ?, 'ADMIN', 'ACTIVE', NOW(), NOW(), 1)`, email)
		if result.Error != nil {
			log.Fatalf("Failed to create admin: %v", result.Error)
		}
		db.Raw(`SELECT id FROM "User" WHERE email = ?`, email).Scan(&existingID)
	}

	// Upsert password hash into UserCredential
	result := db.Exec(`INSERT INTO "UserCredential" (user_id, password_hash, last_changed_at, created_at, updated_at)
		VALUES (?, ?, NOW(), NOW(), NOW())
		ON CONFLICT (user_id) DO UPDATE SET password_hash = EXCLUDED.password_hash, updated_at = NOW()`, existingID, string(hash))
	if result.Error != nil {
		log.Fatalf("Failed to update password: %v", result.Error)
	}
	fmt.Printf("✅ Admin user ready: %s\n", email)
}
