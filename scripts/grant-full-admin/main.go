package main

// grant-full-admin promotes an existing (or missing) account to a full-access
// administrator: role SUPER_ADMIN, status ACTIVE, and the `admin:bypass`
// wildcard grant persisted in User.permissions.
//
// Why SUPER_ADMIN and not ADMIN: only RoleSuperAdmin's defaults include
// PermAdminBypass (see internal/domain/common/permissions_defaults.go). The
// plain ADMIN role deliberately omits the wildcard so restricted admins remain
// possible, so promoting to ADMIN would NOT grant every permission.
//
// Credentials are read from the environment so nothing sensitive lands in git
// (same convention as scripts/create-admin-ffyoussef and scripts/disable-admin-mfa):
//
//	$env:ADMIN_EMAIL="someone@example.com"
//	$env:ADMIN_PASSWORD="..."   # optional: only set to (re)set the password
//	go run ./scripts/grant-full-admin
//
// Run from the repository root (d:\backend) so .env is picked up.
//
// If `go run` is blocked by a Windows Application Control / WDAC policy
// ("An Application Control policy has blocked this file"), build to an allowed
// directory first and run the binary:
//
//	go build -o ./bin/grant-full-admin.exe ./scripts/grant-full-admin
//	./bin/grant-full-admin.exe

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type userRow struct {
	ID               string
	Email            string
	Role             string
	Status           string
	TwoFactorEnabled bool
	Permissions      models.JSONStringArray
}

func main() {
	if godotenv.Load(".env") != nil {
		if godotenv.Load("../../.env") != nil {
			log.Println("No .env file found, using system environment variables")
		}
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	email := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL")))
	if email == "" {
		log.Fatal("ADMIN_EMAIL is not set")
	}
	// Optional: only rotates the password when provided.
	password := os.Getenv("ADMIN_PASSWORD")

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	step := 0
	stepf := func(format string, args ...interface{}) {
		step++
		fmt.Printf("\n[STEP %d] %s\n", step, fmt.Sprintf(format, args...))
	}

	// ── 1) Locate or create the account ──────────────────────────────
	stepf("Looking up %s", email)
	var before userRow
	db.Raw(`SELECT id, email, role, status, two_factor_enabled
	        FROM "User" WHERE lower(email) = ? AND deleted_at IS NULL`, email).Scan(&before)

	if before.ID == "" {
		fmt.Println("  Not found — creating the account")
		newID := uuid.New().String()
		if err := db.Exec(`INSERT INTO "User" (id, email, role, status, created_at, updated_at, version)
		                   VALUES (?, ?, ?, ?, NOW(), NOW(), 1)`,
			newID, email, string(models.RoleSuperAdmin), string(models.StatusActive)).Error; err != nil {
			log.Fatalf("Failed to create user: %v", err)
		}
		before.ID = newID
		before.Email = email
	} else {
		fmt.Printf("  id=%s role=%s status=%s twoFactorEnabled=%t\n",
			before.ID, before.Role, before.Status, before.TwoFactorEnabled)
	}
	userID := before.ID

	// ── 2) Role + status ─────────────────────────────────────────────
	stepf("Setting role=%s and status=%s", models.RoleSuperAdmin, models.StatusActive)
	res := db.Exec(`UPDATE "User"
	                SET role = ?, status = ?, status_reason = NULL, status_expires_at = NULL,
	                    email_verified = true, updated_at = NOW()
	                WHERE id = ?`,
		string(models.RoleSuperAdmin), string(models.StatusActive), userID)
	if res.Error != nil {
		log.Fatalf("Failed to update role/status: %v", res.Error)
	}
	fmt.Printf("  Rows affected: %d\n", res.RowsAffected)

	// ── 3) Persist the wildcard grant ────────────────────────────────
	// `admin:bypass` satisfies every permission check in PermissionGrantMatches.
	// The `permissions:custom` sentinel must NOT be present, otherwise role
	// defaults are skipped and only the stored list applies.
	//
	// This is additive: the existing grants are kept and `admin:bypass` is
	// appended if missing. Replacing the list outright would be equivalent
	// while the role stays SUPER_ADMIN (bypass is a wildcard), but it would
	// silently strip the account down to nothing if the role were ever
	// demoted back to ADMIN.
	stepf("Adding %s to permissions and removing the %s sentinel",
		models.PermAdminBypass, models.PermPermissionsCustom)
	res = db.Exec(`UPDATE "User"
	               SET permissions = (
	                     SELECT COALESCE(jsonb_agg(p ORDER BY ord), '[]'::jsonb)
	                     FROM (
	                       SELECT p, ord FROM jsonb_array_elements_text(
	                              CASE WHEN jsonb_typeof(COALESCE(permissions, '[]'::jsonb)) = 'array'
	                                   THEN permissions ELSE '[]'::jsonb END
	                            ) WITH ORDINALITY AS t(p, ord)
	                       WHERE p NOT IN (?, ?)
	                       UNION ALL
	                       SELECT ?, 0
	                     ) merged
	                   ),
	                   updated_at = NOW()
	               WHERE id = ?`,
		models.PermAdminBypass, models.PermPermissionsCustom,
		models.PermAdminBypass, userID)
	if res.Error != nil {
		log.Fatalf("Failed to update permissions: %v", res.Error)
	}
	fmt.Printf("  Rows affected: %d\n", res.RowsAffected)

	// ── 4) Password (optional) ───────────────────────────────────────
	if password != "" {
		stepf("Upserting the bcrypt credential in UserCredential")
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(password), 12)
		if hashErr != nil {
			log.Fatalf("Failed to hash password: %v", hashErr)
		}
		res = db.Exec(`INSERT INTO "UserCredential" (id, user_id, password_hash, last_changed_at, created_at, updated_at)
		               VALUES (?, ?, ?, NOW(), NOW(), NOW())
		               ON CONFLICT (user_id) DO UPDATE
		                 SET password_hash = EXCLUDED.password_hash,
		                     last_changed_at = NOW(),
		                     reset_token = NULL,
		                     reset_expires_at = NULL,
		                     expires_at = NULL,
		                     deleted_at = NULL,
		                     updated_at = NOW()`,
			uuid.New().String(), userID, string(hash))
		if res.Error != nil {
			log.Fatalf("Failed to upsert credential: %v", res.Error)
		}
		fmt.Printf("  Rows affected: %d\n", res.RowsAffected)
	} else {
		stepf("ADMIN_PASSWORD not set — leaving the existing password untouched")
	}

	// ── 5) Invalidate the role/permission caches ─────────────────────
	// middleware.resolveUserFromDB caches role+perms for 5 minutes in-process
	// and in Redis under role_perms:<id>; without clearing it the old
	// permission set stays live. Also clears the login lockout counters.
	stepf("Clearing the Redis role/permission cache and lockout counters")
	clearRedis(userID)

	// ── 6) Verify ────────────────────────────────────────────────────
	stepf("Verifying the final state")
	var after userRow
	// NOTE: permissions must be scanned as part of a struct row so GORM applies
	// JSONStringArray.Scan to the jsonb column. Scanning it into a bare
	// JSONStringArray makes GORM treat the slice as the destination *list* and
	// appends the raw jsonb text as a single element.
	db.Raw(`SELECT id, email, role, status, two_factor_enabled, permissions
	        FROM "User" WHERE id = ?`, userID).Scan(&after)
	storedPerms := after.Permissions

	var hasCredential bool
	db.Raw(`SELECT EXISTS (SELECT 1 FROM "UserCredential"
	        WHERE user_id = ? AND deleted_at IS NULL)`, userID).Scan(&hasCredential)

	probe := &models.User{Role: models.UserRole(after.Role), Permissions: storedPerms}
	fmt.Printf("  id:              %s\n", after.ID)
	fmt.Printf("  email:           %s\n", after.Email)
	fmt.Printf("  role:            %s\n", after.Role)
	fmt.Printf("  status:          %s\n", after.Status)
	fmt.Printf("  stored perms:    %d entries, admin:bypass=%t, permissions:custom=%t\n",
		len(storedPerms), contains(storedPerms, models.PermAdminBypass),
		contains(storedPerms, models.PermPermissionsCustom))
	fmt.Printf("  effective perms: %d entries\n", len(probe.GetEffectivePermissions()))
	fmt.Printf("  has credential:  %t\n", hasCredential)
	fmt.Printf("  2FA enabled:     %t\n", after.TwoFactorEnabled)

	// Spot-check high-privilege permissions through the real matcher.
	for _, p := range []string{
		models.PermSystemManage,
		models.PermUsersManage,
		models.PermUsersImpersonate,
		models.PermDashboardViewFinancialMetrics,
		models.PermAuditLogsView,
	} {
		fmt.Printf("  HasPermission(%-38s) = %t\n", p, probe.HasPermission(p))
	}

	if after.TwoFactorEnabled {
		fmt.Println("\n[WARN] 2FA is enabled: login returns MFA_REQUIRED and needs the")
		fmt.Println("       authenticator code. Run scripts/disable-admin-mfa to turn it off.")
	}
	fmt.Println("\n[OK] Done — the account now holds admin:bypass (full access).")
	fmt.Println("     Log out and back in so a fresh JWT carries the new role.")
}

// contains reports whether the permission list holds the given grant.
func contains(perms models.JSONStringArray, want string) bool {
	for _, p := range perms {
		if p == want {
			return true
		}
	}
	return false
}

// clearRedis removes the cached role/permission entry plus the lockout counters
// for the user and notifies other instances, mirroring
// middleware.InvalidateRolePermsCache and scripts/clear-rate-limits.
func clearRedis(userID string) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" || strings.EqualFold(os.Getenv("DISABLE_REDIS"), "true") {
		fmt.Println("  Redis not configured — skipping (in-process cache expires within 5 minutes)")
		return
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		fmt.Printf("  Invalid REDIS_URL, skipping: %v\n", err)
		return
	}
	client := redis.NewClient(opts)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	keys := []string{
		fmt.Sprintf("role_perms:%s", userID),
		fmt.Sprintf("lockout:%s", userID),
		fmt.Sprintf("failed_attempts:%s", userID),
	}
	deleted, err := client.Del(ctx, keys...).Result()
	if err != nil {
		fmt.Printf("  Redis DEL failed, skipping: %v\n", err)
		return
	}
	if err := client.Publish(ctx, "cache:invalidate:role_perms", userID).Err(); err != nil {
		fmt.Printf("  Redis PUBLISH failed: %v\n", err)
	}
	fmt.Printf("  Deleted %d key(s), published cache:invalidate:role_perms\n", deleted)
}
