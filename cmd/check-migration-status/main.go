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

	fmt.Println("Database connected successfully!")

	// Check search_vector column
	var columnExists int64
	db.Raw(`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'Subject' AND column_name = 'search_vector'`).Scan(&columnExists)
	if columnExists > 0 {
		fmt.Println("✓ Column 'search_vector' exists in Subject table")
	} else {
		fmt.Println("✗ Column 'search_vector' does NOT exist in Subject table")
	}

	// Check for the index
	var indexExists int64
	db.Raw(`SELECT COUNT(*) FROM pg_indexes WHERE indexname = 'idx_subject_search_vector'`).Scan(&indexExists)
	if indexExists > 0 {
		fmt.Println("✓ Index 'idx_subject_search_vector' exists")
	} else {
		fmt.Println("✗ Index 'idx_subject_search_vector' does NOT exist")
	}

	// Check for the function
	var functionExists int64
	db.Raw(`SELECT COUNT(*) FROM pg_proc WHERE proname = 'update_subject_search_vector'`).Scan(&functionExists)
	if functionExists > 0 {
		fmt.Println("✓ Function 'update_subject_search_vector' exists")
	} else {
		fmt.Println("✗ Function 'update_subject_search_vector' does NOT exist")
	}

	// Check for the trigger
	var triggerExists int64
	db.Raw(`SELECT COUNT(*) FROM pg_trigger WHERE tgname = 'trg_subject_search_vector'`).Scan(&triggerExists)
	if triggerExists > 0 {
		fmt.Println("✓ Trigger 'trg_subject_search_vector' exists")
	} else {
		fmt.Println("✗ Trigger 'trg_subject_search_vector' does NOT exist")
	}

	// List migrations
	fmt.Println("\n--- Applied Migrations ---")
	var migrations []struct {
		ID        string
		Checksum  string
		AppliedAt string
	}
	db.Raw(`SELECT id, checksum, "appliedAt" FROM schema_migrations ORDER BY "appliedAt"`).Scan(&migrations)
	for _, m := range migrations {
		fmt.Printf("  %s (applied: %s)\n", m.ID, m.AppliedAt)
	}
}
