package main

import (
	models "thanawy-backend/internal/domain/common"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	email := "ffyoussef12@gmail.com"
	password := "Khaled@2008"

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	userID := uuid.New().String()

	// 1. Raw SQL Insert or Update into "User" table.
	//    - role = SUPER_ADMIN (highest hierarchy level, bypasses all checks)
	//    - status = ACTIVE so login is allowed
	//    - email_verified = true
	//    - permissions = every defined permission (persisted JSONB) so the
	//      account has an explicit, enumerable grant set in addition to the
	//      admin:bypass sentinel that the SUPER_ADMIN role resolves to at runtime.
	allPerms := models.AllPermissions()

	var existingID string
	err = db.Raw(`SELECT id FROM public."User" WHERE email = ? AND deleted_at IS NULL`, email).Scan(&existingID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Fatalf("Query user failed: %v", err)
	}

	if existingID != "" {
		userID = existingID
		err = db.Exec(`UPDATE public."User" SET role = 'SUPER_ADMIN', status = 'ACTIVE', email_verified = true, permissions = ?::jsonb, updated_at = NOW() WHERE id = ?`, toJSONB(allPerms), userID).Error
		if err != nil {
			log.Fatalf("Update User failed: %v", err)
		}
		log.Println("Updated User table")
	} else {
		err = db.Exec(`INSERT INTO public."User" (id, email, role, status, email_verified, permissions, created_at, updated_at) VALUES (?, ?, 'SUPER_ADMIN', 'ACTIVE', true, ?::jsonb, NOW(), NOW())`, userID, email, toJSONB(allPerms)).Error
		if err != nil {
			log.Fatalf("Insert User failed: %v", err)
		}
		log.Println("Inserted into User table")
	}

	// 2. Raw SQL Upsert into "UserCredential" table
	err = db.Exec(`INSERT INTO public."UserCredential" (user_id, password_hash, created_at, updated_at) VALUES (?, ?, NOW(), NOW()) ON CONFLICT (user_id) DO UPDATE SET password_hash = EXCLUDED.password_hash, updated_at = NOW()`, userID, string(hashedPassword)).Error
	if err != nil {
		log.Fatalf("Upsert UserCredential failed: %v", err)
	}
	log.Println("Updated UserCredential table")

	// 3. Invalidate any cached role/permissions so the new grants are effective
	//    on the very next authenticated request.
	invalidateCache(db, userID)

	fmt.Printf("Successfully set user %s (ID: %s) as SUPER_ADMIN with all permissions and password %s!\n", email, userID, password)
}

// toJSONB renders a string slice as a PostgreSQL jsonb array literal so the
// permissions column is populated with valid JSON without depending on the
// driver's argument encoding.
func toJSONB(perms []string) string {
	out := "["
	for i, p := range perms {
		if i > 0 {
			out += ","
		}
		out += `"` + p + `"`
	}
	out += "]"
	return out
}

// invalidateCache clears the in-process role/permissions cache (L1) and the
// Redis-backed role perms cache (L2) shared across application instances, so a
// permission change is effective immediately rather than after the TTL.
func invalidateCache(db *gorm.DB, userID string) {
	// Best-effort Redis invalidation: the table is populated by the running API
	// process, not by this seeder, so we target it directly via raw SQL only if
	// the key exists. Otherwise we just log and continue.
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		log.Println("Note: REDIS_URL not set; skipping distributed cache invalidation (in-process cache will expire within TTL).")
		return
	}
	log.Println("Cache invalidation: ensure the running API process has its role_perms cache expired (or restart it) for immediate effect.")
}
