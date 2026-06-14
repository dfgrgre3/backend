//go:build ignore

package main

import (
	"database/sql"
	"fmt"
	"log"
	"thanawy-backend/internal/config"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env not loaded: %v", err)
	}

	cfg := config.Load()
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Error opening db: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT id, email, role, status
		FROM "User" 
		WHERE role = 'ADMIN' OR email = 'admin@thanawy.com'
	`)
	if err != nil {
		log.Fatalf("Error querying users: %v", err)
	}
	defer rows.Close()

	fmt.Println("Admin/Matching Users:")
	for rows.Next() {
		var id, email, role, status string
		if err := rows.Scan(&id, &email, &role, &status); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("- ID: %s | Email: %s | Role: %s | Status: %s\n", id, email, role, status)
	}
}
