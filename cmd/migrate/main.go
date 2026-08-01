package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"thanawy-backend/internal/db"
)

type migrationRecord struct {
	ID        string    `gorm:"primaryKey;column:id"`
	Checksum  string    `gorm:"not null;column:checksum"`
	AppliedAt time.Time `gorm:"not null;column:appliedAt"`
}

func (migrationRecord) TableName() string {
	return "schema_migrations"
}

func main() {
	// Load environment variables
	_ = godotenv.Load(".env.local")
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Get database URL directly without any config validation
	databaseURL := getDatabaseURL()
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	// Initialize Database directly using GORM - no config, no storage validation
	database, err := connectDatabase(databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Checking/Applying SQL database migrations...")
	if err := runSQLMigrations(database); err != nil {
		log.Fatalf("CRITICAL: SQL migrations failed: %v", err)
	}
	log.Println("SQL database migrations applied successfully.")

	log.Println("Migration process completed.")
}

func connectDatabase(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
			SlowThreshold:             500 * time.Millisecond,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
		}),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetConnMaxLifetime(15 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	log.Println("Database connection established")
	return db, nil
}

func getDatabaseURL() string {
	// Try to use migration role for schema changes
	migrationDSN, err := db.GetMigrationDSN()
	if err == nil && migrationDSN != "" {
		log.Println("Attempting to use migration_user role for schema changes.")
		// Test if the role exists by trying to connect
		testDB, testErr := connectDatabase(migrationDSN)
		if testErr == nil {
			sqlDB, _ := testDB.DB()
			if sqlDB != nil {
				sqlDB.Close()
			}
			log.Println("Using migration_user role for schema changes.")
			return migrationDSN
		}
		log.Printf("Migration role not available, falling back to default connection: %v", testErr)
	}

	if directURL := os.Getenv("DATABASE_URL_DIRECT"); directURL != "" {
		log.Println("Using DATABASE_URL_DIRECT for migrations.")
		return directURL
	}
	if writeURL := os.Getenv("DATABASE_WRITE_DSN"); writeURL != "" {
		log.Println("Using DATABASE_WRITE_DSN for migrations.")
		return writeURL
	}
	return os.Getenv("DATABASE_URL")
}

func runSQLMigrations(database *gorm.DB) error {
	if err := database.Exec(`SELECT pg_advisory_lock(hashtext('thanawy_backend_schema_migrations'))`).Error; err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		if err := database.Exec(`SELECT pg_advisory_unlock(hashtext('thanawy_backend_schema_migrations'))`).Error; err != nil {
			log.Printf("failed to release migration lock: %v", err)
		}
	}()

	if err := ensureMigrationTable(database); err != nil {
		return err
	}

	names, err := getMigrationNames()
	if err != nil {
		return err
	}

	for _, name := range names {
		if err := applyMigration(database, name); err != nil {
			return err
		}
	}

	return nil
}

func ensureMigrationTable(database *gorm.DB) error {
	// Create the migration table if it doesn't exist
	// This needs to be done before running migrations
	err := database.Exec(`
		CREATE TABLE IF NOT EXISTS "schema_migrations" (
			id text PRIMARY KEY,
			checksum text NOT NULL,
			"appliedAt" timestamptz NOT NULL DEFAULT now()
		)
	`).Error
	if err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}
	return nil
}

func getMigrationNames() ([]string, error) {
	// Read from filesystem instead of embed
	entries, err := os.ReadDir("internal/db/migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sql" {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func applyMigration(database *gorm.DB, name string) error {
	id := name[:len(name)-len(filepath.Ext(name))]
	contents, err := os.ReadFile("internal/db/migrations/" + name)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}

	if len(contents) == 0 {
		log.Printf("Skipping empty migration file %s", name)
		return nil
	}

	sum := sha256.Sum256(contents)
	checksum := hex.EncodeToString(sum[:])

	var existing migrationRecord
	err = database.First(&existing, "id = ?", id).Error
	if err == nil {
		if existing.Checksum != checksum {
			return fmt.Errorf("migration %s checksum mismatch: applied checksum %s, file checksum %s", id, existing.Checksum, checksum)
		}
		log.Printf("Migration %s already applied (checksum matches)", id)
		return nil
	}

	if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("check migration %s: %w", id, err)
	}

	log.Printf("Applying database migration %s", id)
	
	// For baseline schema, check if tables already exist and mark as applied if so
	if id == "0000_baseline_schema" {
		// Check if any core table exists (e.g., User table)
		var tableExists int64
		database.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'User' AND table_schema = 'public'").Scan(&tableExists)
		
		if tableExists > 0 {
			// Check if the User table has a primary key constraint
			var hasPK int64
			database.Raw("SELECT COUNT(*) FROM information_schema.table_constraints WHERE table_name = 'User' AND constraint_type = 'PRIMARY KEY' AND table_schema = 'public'").Scan(&hasPK)
			
			if hasPK > 0 {
				log.Printf("Baseline schema tables already exist with proper constraints, marking migration as applied")
				// Manually insert the migration record using raw SQL with ON CONFLICT
				sqlDB, _ := database.DB()
				_, err = sqlDB.Exec(`INSERT INTO schema_migrations (id, checksum, "appliedAt") VALUES ($1, $2, $3) ON CONFLICT (id) DO UPDATE SET checksum = EXCLUDED.checksum, "appliedAt" = EXCLUDED."appliedAt"`, id, checksum, time.Now().UTC())
				if err != nil {
					return fmt.Errorf("record migration %s: %w", id, err)
				}
				return nil
			}
			
			// Tables exist but don't have proper constraints - need to drop and recreate
			log.Printf("Tables exist but missing constraints, dropping and recreating")
			database.Exec(`DROP SCHEMA public CASCADE`)
			database.Exec(`CREATE SCHEMA public`)
		}
		
		// Tables don't exist, need to apply the migration
		log.Printf("Applying baseline schema migration")
		
		// For fresh database, we need to handle the schema_migrations table conflict
		// First, drop the schema_migrations table that was created by ensureMigrationTable
		database.Exec(`DROP TABLE IF EXISTS schema_migrations CASCADE`)
		
		// Execute the entire baseline schema as a single block to preserve dependencies
		content := string(contents)
		// Remove the schema_migrations table definition more comprehensively
		content = regexp.MustCompile(`(?s)--\n-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -\n--\n\nCREATE TABLE public\.schema_migrations \([^)]+\);\n\n--\n-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -\n--\n\nALTER TABLE ONLY public\.schema_migrations\n    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY \(id\);\n\n`).ReplaceAllString(content, "")
		
		if err := database.Exec(content).Error; err != nil {
			return fmt.Errorf("apply migration %s: %w", id, err)
		}
		
		// Create the schema_migrations table manually
		database.Exec(`
			CREATE TABLE schema_migrations (
				id text PRIMARY KEY,
				checksum text NOT NULL,
				"appliedAt" timestamptz NOT NULL DEFAULT now()
			)
		`)
		// Manually insert the migration record using raw SQL
		sqlDB, _ := database.DB()
		_, err = sqlDB.Exec(`INSERT INTO schema_migrations (id, checksum, "appliedAt") VALUES ($1, $2, $3)`, id, checksum, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("record migration %s: %w", id, err)
		}
		return nil
	}
	
	// For other migrations, use statement splitting
	return database.Transaction(func(tx *gorm.DB) error {
		statements := splitSQLStatements(string(contents))
		for i, stmt := range statements {
			trimmed := strings.TrimSpace(stmt)
			if trimmed == "" || strings.HasPrefix(trimmed, "--") {
				continue
			}
			// Skip CREATE TABLE IF NOT EXISTS for schema_migrations as it's already created
			if strings.Contains(strings.ToUpper(trimmed), "CREATE TABLE") && 
			   strings.Contains(strings.ToUpper(trimmed), "SCHEMA_MIGRATIONS") {
				log.Printf("Skipping schema_migrations table creation (already exists)")
				continue
			}
			if err := tx.Exec(stmt).Error; err != nil {
				return fmt.Errorf("apply migration %s statement %d: %w\nStatement: %.200s", id, i+1, err, stmt)
			}
		}
		return tx.Exec(`INSERT INTO schema_migrations (id, checksum, "appliedAt") VALUES (?, ?, ?) ON CONFLICT (id) DO UPDATE SET checksum = EXCLUDED.checksum, "appliedAt" = EXCLUDED."appliedAt"`, id, checksum, time.Now().UTC()).Error
	})
}

func extractTableNameFromAlter(stmt string) string {
	// Extract table name from ALTER TABLE statement
	// Format: ALTER TABLE ONLY public."TableName" or ALTER TABLE public."TableName"
	re := regexp.MustCompile(`ALTER TABLE (?:ONLY )?public\."?(\w+)"?`)
	matches := re.FindStringSubmatch(stmt)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func splitSQLStatements(contents string) []string {
	var statements []string
	var current strings.Builder
	var inDollarQuote bool
	var dollarQuoteTag string

	// Find all dollar quote patterns in the content
	dollarQuoteRegex := regexp.MustCompile(`\$[a-zA-Z_]*\$`)

	lines := strings.Split(contents, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}

		current.WriteString(line)
		current.WriteString("\n")

		// Handle dollar-quoted strings ($$ or $tag$)
		matches := dollarQuoteRegex.FindAllStringIndex(line, -1)
		for _, match := range matches {
			tag := line[match[0]:match[1]]
			if !inDollarQuote {
				// Start of dollar-quoted string
				dollarQuoteTag = tag
				inDollarQuote = true
			} else if tag == dollarQuoteTag {
				// End of dollar-quoted string (matching tag)
				inDollarQuote = false
				dollarQuoteTag = ""
			}
		}

		// Only split on semicolon if not inside a dollar-quoted string
		if !inDollarQuote && strings.HasSuffix(trimmed, ";") {
			stmt := current.String()
			statements = append(statements, stmt)
			current.Reset()
		}
	}

	if current.Len() > 0 {
		statements = append(statements, current.String())
	}

	return statements
}
