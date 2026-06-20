package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"thanawy-backend/internal/config"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"

	"github.com/joho/godotenv"
)

type clerkUserSearchResult struct {
	ID           string   `json:"id"`
	EmailAddress []struct {
		EmailAddress string `json:"email_address"`
	} `json:"email_addresses"`
}

func main() {
	// Parse CLI flags
	batchSize := flag.Int("batch-size", 100, "Number of users to fetch from DB per batch")
	delayMs := flag.Int("delay", 200, "Delay in milliseconds between Clerk API calls to respect rate limits")
	dryRun := flag.Bool("dry-run", false, "Simulate backfill without writing to the database")
	flag.Parse()

	// Load environment variables
	_ = godotenv.Load(".env.local")
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	cfg := config.Load()
	if cfg.ClerkSecretKey == "" {
		log.Fatal("CRITICAL ERROR: CLERK_SECRET_KEY is not configured in the environment variables.")
	}

	// Connect to Database
	databaseURL := getDatabaseURL(cfg)
	database, err := db.Connect(databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Printf("Starting Clerk ID backfill (Dry-Run: %t, Batch Size: %d, Delay: %dms)...", *dryRun, *batchSize, *delayMs)

	// Fetch users where clerk_id is null or empty
	var totalProcessed, totalUpdated, totalFailed, totalNotFound, totalDuplicates int

	// Using keyset pagination to process users in batches safely (immune to offset shifts and loop anomalies)
	var lastID string = ""
	for {
		var batch []models.User
		query := database.Model(&models.User{}).
			Where("clerk_id IS NULL OR clerk_id = ''")
		if lastID != "" {
			query = query.Where("id > ?", lastID)
		}
		err := query.Order("id ASC").Limit(*batchSize).Find(&batch).Error

		if err != nil {
			log.Fatalf("Error querying users batch from database: %v", err)
		}

		if len(batch) == 0 {
			break
		}

		log.Printf("Processing batch of %d users (Last ID: %s)...", len(batch), lastID)

		for _, user := range batch {
			lastID = user.ID
			totalProcessed++
			log.Printf("[%d] Processing user ID: %s (Email: %s)...", totalProcessed, user.ID, user.Email)

			// Query Clerk API by email
			clerkID, err := queryClerkIDByEmail(user.Email, cfg.ClerkSecretKey)
			if err != nil {
				if errors.Is(err, errUserNotFoundInClerk) {
					log.Printf("⚠️ User not found in Clerk for email: %s. Skipping.", user.Email)
					totalNotFound++
				} else {
					log.Printf("❌ Failed to query Clerk for user %s: %v", user.Email, err)
					totalFailed++
				}
				continue
			}

			log.Printf("✅ Found Clerk ID: %s for email: %s", clerkID, user.Email)

			// Check if clerk_id is already assigned to another user in the database
			var duplicateCheck []models.User
			err = database.Where("clerk_id = ? AND id != ?", clerkID, user.ID).Limit(1).Find(&duplicateCheck).Error
			if err != nil {
				log.Printf("❌ Failed to query database to check duplicate clerk_id: %v", err)
				totalFailed++
				continue
			}
			if len(duplicateCheck) > 0 {
				log.Printf("⚠️ Clerk ID %s is already assigned to user ID %s (Email: %s). Skipping user ID %s.", clerkID, duplicateCheck[0].ID, duplicateCheck[0].Email, user.ID)
				totalDuplicates++
				continue
			}

			if !*dryRun {
				// Update database
				err := database.Model(&models.User{}).
					Where("id = ?", user.ID).
					Update("clerk_id", clerkID).Error
				if err != nil {
					log.Printf("❌ Failed to update database for user %s: %v", user.ID, err)
					totalFailed++
					continue
				}
				totalUpdated++
			} else {
				totalUpdated++
			}

			// Respect rate limits
			if *delayMs > 0 {
				time.Sleep(time.Duration(*delayMs) * time.Millisecond)
			}
		}
	}

	log.Printf("=== Backfill Summary ===")
	log.Printf("Total users processed: %d", totalProcessed)
	log.Printf("Successfully backfilled: %d", totalUpdated)
	log.Printf("Not found in Clerk: %d", totalNotFound)
	log.Printf("Skipped (Duplicate Clerk ID): %d", totalDuplicates)
	log.Printf("Failed (Errors): %d", totalFailed)
	log.Printf("========================")
}

var errUserNotFoundInClerk = errors.New("user not found in Clerk")

func queryClerkIDByEmail(email string, secretKey string) (string, error) {
	clerkQueryURL := fmt.Sprintf("https://api.clerk.com/v1/users?email_address=%s", url.QueryEscape(email))
	req, err := http.NewRequest("GET", clerkQueryURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+secretKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("clerk api returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var results []clerkUserSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return "", err
	}

	if len(results) == 0 {
		return "", errUserNotFoundInClerk
	}

	// Find the exact email match since query is prefix/substring match sometimes
	for _, res := range results {
		for _, emailObj := range res.EmailAddress {
			if strings.EqualFold(emailObj.EmailAddress, email) {
				return res.ID, nil
			}
		}
	}

	// Fallback to first result if exact match is missing but results exist
	return results[0].ID, nil
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
