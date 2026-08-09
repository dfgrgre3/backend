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

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get database instance: %v", err)
	}

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	fmt.Println("Successfully connected to database!")

	// Check if schema_migrations table exists
	var exists bool
	db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'schema_migrations' AND table_schema = 'public')").Scan(&exists)
	if exists {
		fmt.Println("schema_migrations table exists")

		// Count migrations
		var count int64
		db.Raw("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
		fmt.Printf("Number of applied migrations: %d\n", count)
	} else {
		fmt.Println("schema_migrations table does not exist")
	}
}
