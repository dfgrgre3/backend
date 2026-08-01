package main

import (
	"fmt"
	"log"
	"os"

	"thanawy-backend/internal/config"
	"thanawy-backend/internal/db"

	"github.com/joho/godotenv"
)

type constraintInfo struct {
	TableName        string `gorm:"column:table_name"`
	ConstraintName   string `gorm:"column:constraint_name"`
	ColumnName       string `gorm:"column:column_name"`
	ForeignTable     string `gorm:"column:foreign_table_name"`
	ForeignColumn    string `gorm:"column:foreign_column_name"`
}

func main() {
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load()

	cfg := config.Load()
	databaseURL := getDatabaseURL(cfg)
	database, err := db.Connect(databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	var constraints []constraintInfo
	query := `
		SELECT
			tc.table_name, 
			tc.constraint_name,
			kcu.column_name, 
			ccu.table_name AS foreign_table_name,
			ccu.column_name AS foreign_column_name 
		FROM 
			information_schema.table_constraints AS tc 
			JOIN information_schema.key_column_usage AS kcu
			  ON tc.constraint_name = kcu.constraint_name
			  AND tc.table_schema = kcu.table_schema
			JOIN information_schema.constraint_column_usage AS ccu
			  ON ccu.constraint_name = tc.constraint_name
			  AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' AND ccu.table_name='User';
	`

	if err := database.Raw(query).Scan(&constraints).Error; err != nil {
		log.Fatalf("Failed to query constraints: %v", err)
	}

	fmt.Println("=== Foreign Key Constraints Referencing User Table ===")
	for _, c := range constraints {
		fmt.Printf("Table: %-30s | Column: %-20s | Constraint: %s\n", c.TableName, c.ColumnName, c.ConstraintName)
	}
	fmt.Printf("Total constraints: %d\n", len(constraints))
}

func getDatabaseURL(cfg *config.Config) string {
	if directURL := os.Getenv("DATABASE_URL_DIRECT"); directURL != "" {
		return directURL
	}
	if cfg.DatabaseWriteURL != "" {
		return cfg.DatabaseWriteURL
	}
	return cfg.DatabaseURL
}
