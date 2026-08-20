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
		dsn = os.Getenv("DATABASE_URL_DIRECT")
	}
	if dsn == "" {
		log.Fatal("DATABASE_URL or DATABASE_URL_DIRECT is required")
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

	fmt.Println("Dropping search_vector column if exists...")

	// Drop the column if it exists
	result, err := sqlDB.Exec(`ALTER TABLE "Subject" DROP COLUMN IF EXISTS search_vector`)
	if err != nil {
		fmt.Printf("Error dropping column: %v\n", err)
	} else {
		rows, _ := result.RowsAffected()
		fmt.Printf("Column dropped (affected rows: %d)\n", rows)
	}

	// Drop the index if it exists
	result, err = sqlDB.Exec(`DROP INDEX IF EXISTS idx_subject_search_vector`)
	if err != nil {
		fmt.Printf("Error dropping index: %v\n", err)
	} else {
		fmt.Println("Index dropped (if existed)")
	}

	// Drop the function if it exists
	result, err = sqlDB.Exec(`DROP FUNCTION IF EXISTS update_subject_search_vector()`)
	if err != nil {
		fmt.Printf("Error dropping function: %v\n", err)
	} else {
		fmt.Println("Function dropped (if existed)")
	}

	// Drop the trigger if it exists
	result, err = sqlDB.Exec(`DROP TRIGGER IF EXISTS trg_subject_search_vector ON "Subject"`)
	if err != nil {
		fmt.Printf("Error dropping trigger: %v\n", err)
	} else {
		fmt.Println("Trigger dropped (if existed)")
	}

	fmt.Println("\nCleanup complete. Migration can now be re-applied.")
}
