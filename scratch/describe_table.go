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
		SELECT table_name, column_name, data_type 
		FROM information_schema.columns 
		WHERE column_name = 'tags'
	`)
	if err != nil {
		log.Fatalf("Error querying columns: %v", err)
	}
	defer rows.Close()

	fmt.Println("Tables with 'tags' column:")
	for rows.Next() {
		var tableName, columnName, dataType string
		if err := rows.Scan(&tableName, &columnName, &dataType); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("- %s.%s: %s\n", tableName, columnName, dataType)
	}
}
