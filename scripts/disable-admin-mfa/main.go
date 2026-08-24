package main

// Fix-AdminMFA: Disables MFA enforcement for the admin user
// Run: go run scripts/disable-admin-mfa/main.go

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("No .env file found - using system environment")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	// Target email is env-driven rather than hardcoded so this script isn't
	// tied to one specific account (and doesn't bake a personal email into
	// source control).
	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		log.Fatal("ADMIN_EMAIL is not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	step := 0
	helper := func(msg string) {
		step++
		fmt.Printf("\n[STEP %d] %s\n", step, msg)
	}

	helper("Inspecting current state of the admin user")
	var before []map[string]interface{}
	db.Raw(`
		SELECT id, email, role, status, two_factor_enabled,
		       two_factor_secret IS NOT NULL AS has_secret,
		       backup_codes IS NOT NULL AS has_backup
		FROM "User"
		WHERE email = ?`, adminEmail).Scan(&before)

	for _, row := range before {
		fmt.Printf("  %v\n", row)
	}
	if len(before) == 0 {
		log.Fatalf("No user found with email %s", adminEmail)
	}

	helper("Disabling 2FA on the User row")
	res := db.Exec(`
		UPDATE "User"
		SET two_factor_enabled = false,
		    two_factor_secret  = NULL,
		    backup_codes       = NULL,
		    updated_at         = ?
		WHERE email = ?`, time.Now(), adminEmail)
	fmt.Printf("  Rows affected: %d\n", res.RowsAffected)

	helper("Removing two_factor_settings enforcement rows")
	res = db.Exec(`
		DELETE FROM public.two_factor_settings
		WHERE user_id IN (SELECT id FROM "User" WHERE email = ?)`,
		adminEmail)
	fmt.Printf("  Rows affected: %d\n", res.RowsAffected)

	helper("Removing two_factor_secrets rows")
	res = db.Exec(`
		DELETE FROM public.two_factor_secrets
		WHERE user_id IN (SELECT id FROM "User" WHERE email = ?)`,
		adminEmail)
	fmt.Printf("  Rows affected: %d\n", res.RowsAffected)

	helper("Verifying the fix")
	var after []map[string]interface{}
	db.Raw(`
		SELECT id, email, role, status, two_factor_enabled,
		       two_factor_secret IS NOT NULL AS has_secret,
		       backup_codes IS NOT NULL AS has_backup
		FROM "User"
		WHERE email = ?`, adminEmail).Scan(&after)
	for _, row := range after {
		fmt.Printf("  %v\n", row)
	}

	var settingsCount int64
	db.Raw(`
		SELECT COUNT(*) FROM public.two_factor_settings tfs
		JOIN "User" u ON u.id = tfs.user_id
		WHERE u.email = ?`, adminEmail).Scan(&settingsCount)
	fmt.Printf("  Remaining two_factor_settings rows: %d\n", settingsCount)

	var secretsCount int64
	db.Raw(`
		SELECT COUNT(*) FROM public.two_factor_secrets tfs
		JOIN "User" u ON u.id = tfs.user_id
		WHERE u.email = ?`, adminEmail).Scan(&secretsCount)
	fmt.Printf("  Remaining two_factor_secrets rows: %d\n", secretsCount)

	fmt.Println("\n[OK] Done. Next steps:")
	fmt.Println("  1) Restart the Go backend")
	fmt.Println("  2) Clear browser cookies (access_token, refresh_token)")
	fmt.Println("  3) Re-login and retry /admin/users")
}
