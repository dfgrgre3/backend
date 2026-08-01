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
	
	// Drop the schema_migrations table
	result := db.Exec(`DROP TABLE IF EXISTS schema_migrations CASCADE`)
	if result.Error != nil {
		log.Fatalf("Failed to drop schema_migrations table: %v", result.Error)
	}
	
	fmt.Println("Successfully dropped schema_migrations table")
}
