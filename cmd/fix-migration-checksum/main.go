package main

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type migrationRecord struct {
	ID       string `gorm:"primaryKey;column:id"`
	Checksum string `gorm:"not null;column:checksum"`
}

func (migrationRecord) TableName() string {
	return "schema_migrations"
}

func main() {
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load()

	databaseURL := os.Getenv("DATABASE_URL_DIRECT")
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}
	if databaseURL == "" {
		log.Fatal("DATABASE_URL or DATABASE_URL_DIRECT is required")
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Read all migration files
	entries, err := os.ReadDir("internal/db/migrations")
	if err != nil {
		log.Fatalf("Failed to read migrations directory: %v", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sql" {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	// Update checksums for all migrations
	for _, name := range names {
		id := name[:len(name)-len(filepath.Ext(name))]
		content, err := os.ReadFile("internal/db/migrations/" + name)
		if err != nil {
			log.Printf("Failed to read migration %s: %v", name, err)
			continue
		}

		sum := sha256.Sum256(content)
		checksum := hex.EncodeToString(sum[:])

		log.Printf("Updating checksum for %s: %s", id, checksum)

		result := db.Exec(`UPDATE "schema_migrations" SET checksum = ? WHERE id = ?`, checksum, id)
		if result.Error != nil {
			log.Printf("Failed to update checksum for %s: %v", id, result.Error)
		} else {
			log.Printf("Successfully updated checksum for %s", id)
		}
	}

	log.Println("All checksums updated successfully")
}
