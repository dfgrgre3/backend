package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	_ = godotenv.Load()
	
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}
	
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	
	// Drop all tables in the public schema
	result := db.Exec(`DROP SCHEMA public CASCADE`)
	if result.Error != nil {
		log.Fatalf("Failed to drop public schema: %v", result.Error)
	}
	
	// Recreate the public schema
	result = db.Exec(`CREATE SCHEMA public`)
	if result.Error != nil {
		log.Fatalf("Failed to create public schema: %v", result.Error)
	}
	
	fmt.Println("Successfully dropped and recreated public schema")
}
