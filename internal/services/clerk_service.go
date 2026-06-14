package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"thanawy-backend/internal/config"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"

	"gorm.io/gorm"
	"github.com/google/uuid"
)

// InvalidateCacheCallback allows breaking circular import with middleware package
var InvalidateCacheCallback func(string)

// ClerkIDToUUID generates a deterministic UUID from a Clerk User ID
func ClerkIDToUUID(clerkID string) string {
	if !strings.HasPrefix(clerkID, "user_") {
		return clerkID
	}
	u := uuid.NewSHA1(uuid.NameSpaceDNS, []byte(clerkID))
	return u.String()
}

type clerkEmailAddress struct {
	EmailAddress string `json:"email_address"`
}

type clerkUserResponse struct {
	ID             string              `json:"id"`
	FirstName      *string             `json:"first_name"`
	LastName       *string             `json:"last_name"`
	Username       *string             `json:"username"`
	ImageURL       *string             `json:"image_url"`
	EmailAddresses []clerkEmailAddress `json:"email_addresses"`
	PublicMetadata map[string]any      `json:"public_metadata"`
}

func FetchUserFromClerk(userId string) (*clerkUserResponse, error) {
	cfg := config.Load()
	if cfg.ClerkSecretKey == "" {
		return nil, errors.New("CLERK_SECRET_KEY is not configured")
	}

	url := fmt.Sprintf("https://api.clerk.com/v1/users/%s", userId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.ClerkSecretKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("user not found in Clerk: %s", userId)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("clerk api returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var clerkUser clerkUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&clerkUser); err != nil {
		return nil, err
	}

	return &clerkUser, nil
}

func ProvisionUserFromClerk(userId string) (*models.User, error) {
	clerkUser, err := FetchUserFromClerk(userId)
	if err != nil {
		return nil, err
	}

	var primaryEmail string
	if len(clerkUser.EmailAddresses) > 0 {
		primaryEmail = clerkUser.EmailAddresses[0].EmailAddress
	}

	if primaryEmail == "" {
		return nil, fmt.Errorf("user %s has no email addresses in Clerk", userId)
	}

	var name string
	if clerkUser.FirstName != nil {
		name = *clerkUser.FirstName
	}
	if clerkUser.LastName != nil {
		if name != "" {
			name += " " + *clerkUser.LastName
		} else {
			name = *clerkUser.LastName
		}
	}

	role := models.RoleStudent
	if r, ok := clerkUser.PublicMetadata["role"].(string); ok && r != "" {
		role = models.UserRole(strings.ToUpper(r))
	}

	dbUserID := ClerkIDToUUID(userId)

	var existing models.User
	err = db.DB.Unscoped().Where("id = ?", dbUserID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create user
			newUser := models.User{
				ID:            dbUserID,
				Email:         primaryEmail,
				EmailVerified: true,
				Status:        models.StatusActive,
				Role:          role,
				Balance:       0,
				AiCredits:     0,
				ExamCredits:   0,
				TotalXP:       0,
				Level:         1,
				Username:      clerkUser.Username,
			}
			if name != "" {
				newUser.Name = &name
			}
			if clerkUser.ImageURL != nil {
				newUser.Avatar = clerkUser.ImageURL
			}

			if err := db.DB.Create(&newUser).Error; err != nil {
				return nil, err
			}
			log.Printf("[Clerk Provisioning] Dynamic user creation successful: %s (%s)", dbUserID, primaryEmail)
			
			if InvalidateCacheCallback != nil {
				InvalidateCacheCallback(dbUserID)
			}
			
			return &newUser, nil
		}
		return nil, err
	}

	// Update existing user email & metadata
	updates := map[string]any{
		"email":          primaryEmail,
		"email_verified": true,
		"role":           role,
	}
	if name != "" {
		updates["name"] = &name
	}
	if clerkUser.Username != nil {
		updates["username"] = clerkUser.Username
	}
	if clerkUser.ImageURL != nil {
		updates["avatar"] = clerkUser.ImageURL
	}

	if err := db.DB.Model(&models.User{}).Where("id = ?", dbUserID).Updates(updates).Error; err != nil {
		return nil, err
	}

	if InvalidateCacheCallback != nil {
		InvalidateCacheCallback(dbUserID)
	}

	log.Printf("[Clerk Provisioning] Dynamic user sync successful: %s (%s)", dbUserID, primaryEmail)
	return &existing, nil
}
