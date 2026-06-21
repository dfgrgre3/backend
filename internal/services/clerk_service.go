package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"thanawy-backend/internal/config"
	"thanawy-backend/internal/db"
	"thanawy-backend/internal/models"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

// InvalidateCacheCallback allows breaking circular import with middleware package
var InvalidateCacheCallback func(string)

var (
	// clerkProvisionCache uses sync.Map to avoid global mutex contention during HTTP calls.
	// Only singleflight.Group serializes concurrent requests for the same clerkId.
	clerkProvisionCache sync.Map
	clerkProvisionGroup singleflight.Group
)

type clerkCacheEntry struct {
	user      *models.User
	err       error
	expiresAt time.Time
}

// ClerkIDToUUID generates a deterministic UUID from a Clerk User ID.
// Only accepts IDs with the known "user_" prefix or valid UUID strings;
// anything else is treated as a Clerk ID and hashed deterministically.
func ClerkIDToUUID(clerkID string) string {
	if clerkID == "" {
		return uuid.New().String()
	}
	if strings.HasPrefix(clerkID, "user_") {
		u := uuid.NewSHA1(uuid.NameSpaceDNS, []byte(clerkID))
		return u.String()
	}
	if _, err := uuid.Parse(clerkID); err == nil {
		return clerkID
	}
	return uuid.NewSHA1(uuid.NameSpaceDNS, []byte(clerkID)).String()
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
	UnsafeMetadata map[string]any      `json:"unsafe_metadata"`
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
	// Fast path: check sync.Map cache without blocking other users
	if val, ok := clerkProvisionCache.Load(userId); ok {
		entry := val.(*clerkCacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.user, entry.err
		}
	}

	resVal, err, _ := clerkProvisionGroup.Do(userId, func() (interface{}, error) {
		// Re-check cache after acquiring singleflight (avoid duplicate work)
		if val, ok := clerkProvisionCache.Load(userId); ok {
			entry := val.(*clerkCacheEntry)
			if time.Now().Before(entry.expiresAt) {
				return &clerkProvisionResult{user: entry.user, err: entry.err}, nil
			}
		}

		user, err := provisionUserFromClerkInternal(userId)

		// Store result in sync.Map – no mutex contention with other users
		clerkProvisionCache.Store(userId, &clerkCacheEntry{
			user:      user,
			err:       err,
			expiresAt: time.Now().Add(5 * time.Minute),
		})

		return &clerkProvisionResult{user: user, err: err}, nil
	})

	if err != nil {
		return nil, err
	}
	result := resVal.(*clerkProvisionResult)
	return result.user, result.err
}

type clerkProvisionResult struct {
	user *models.User
	err  error
}

func parseAndValidateClerkRole(raw string) models.UserRole {
	role := models.UserRole(strings.ToUpper(strings.TrimSpace(raw)))
	if role == models.RoleAdmin || role == models.RoleSuperAdmin {
		log.Printf("[Security Warning] Attempt to auto-provision administrative role: %q from Clerk metadata was rejected. Defaulting to STUDENT", raw)
		return models.RoleStudent
	}
	if models.IsValidUserRole(role) {
		return role
	}
	return models.RoleStudent
}

func SanitizeClerkString(s string) string {
	return html.EscapeString(strings.TrimSpace(s))
}

func provisionUserFromClerkInternal(userId string) (*models.User, error) {
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
		role = parseAndValidateClerkRole(r)
	}

	var phone, country, gradeLevel, educationType string
	var dateOfBirth *time.Time
	var interestedSubjects []string

	if clerkUser.UnsafeMetadata != nil {
		if p, ok := clerkUser.UnsafeMetadata["phone"].(string); ok {
			phone = SanitizeClerkString(p)
		}
		if c, ok := clerkUser.UnsafeMetadata["country"].(string); ok {
			country = SanitizeClerkString(c)
		}
		if g, ok := clerkUser.UnsafeMetadata["gradeLevel"].(string); ok {
			gradeLevel = SanitizeClerkString(g)
		}
		if e, ok := clerkUser.UnsafeMetadata["educationType"].(string); ok {
			educationType = SanitizeClerkString(e)
		}
		if dob, ok := clerkUser.UnsafeMetadata["dateOfBirth"].(string); ok && dob != "" {
			if parsedDob, err := time.Parse("2006-01-02", SanitizeClerkString(dob)); err == nil {
				dateOfBirth = &parsedDob
			}
		}
		if is, ok := clerkUser.UnsafeMetadata["interestedSubjects"].([]any); ok {
			for _, item := range is {
				if s, ok := item.(string); ok {
					interestedSubjects = append(interestedSubjects, SanitizeClerkString(s))
				}
			}
		}
	}

	dbUserID := ClerkIDToUUID(userId)

	var existing models.User
	err = db.DB.Unscoped().Where("clerk_id = ? OR id = ? OR email = ?", userId, dbUserID, primaryEmail).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create user
			newUser := models.User{
				ID:                 dbUserID,
				ClerkID:            &userId,
				Email:              primaryEmail,
				EmailVerified:      true,
				Status:             models.StatusActive,
				Role:               role,
				Balance:            0,
				AiCredits:          0,
				ExamCredits:        0,
				TotalXP:            0,
				Level:              1,
				Username:           clerkUser.Username,
				Phone:              &phone,
				Country:            &country,
				GradeLevel:         &gradeLevel,
				EducationType:      &educationType,
				DateOfBirth:        dateOfBirth,
				InterestedSubjects: interestedSubjects,
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

	// Update existing user email & metadata (never override role after initial provisioning)
	updates := map[string]any{
		"clerk_id":       userId,
		"email":          primaryEmail,
		"email_verified": true,
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

	if clerkUser.UnsafeMetadata != nil {
		if p, ok := clerkUser.UnsafeMetadata["phone"].(string); ok && p != "" {
			updates["phone"] = SanitizeClerkString(p)
		}
		if c, ok := clerkUser.UnsafeMetadata["country"].(string); ok && c != "" {
			updates["country"] = SanitizeClerkString(c)
		}
		if g, ok := clerkUser.UnsafeMetadata["gradeLevel"].(string); ok && g != "" {
			updates["grade_level"] = SanitizeClerkString(g)
		}
		if e, ok := clerkUser.UnsafeMetadata["educationType"].(string); ok && e != "" {
			updates["education_type"] = SanitizeClerkString(e)
		}
		if dob, ok := clerkUser.UnsafeMetadata["dateOfBirth"].(string); ok && dob != "" {
			if parsedDob, err := time.Parse("2006-01-02", SanitizeClerkString(dob)); err == nil {
				updates["date_of_birth"] = &parsedDob
			}
		}
		if is, ok := clerkUser.UnsafeMetadata["interestedSubjects"].([]any); ok && len(is) > 0 {
			var interestedSubjects []string
			for _, item := range is {
				if s, ok := item.(string); ok {
					interestedSubjects = append(interestedSubjects, SanitizeClerkString(s))
				}
			}
			updates["interested_subjects"] = interestedSubjects
		}
	}

	if err := db.DB.Model(&models.User{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
		return nil, err
	}

	if InvalidateCacheCallback != nil {
		InvalidateCacheCallback(existing.ID)
	}

	log.Printf("[Clerk Provisioning] Dynamic user sync successful: %s (%s)", existing.ID, primaryEmail)
	return &existing, nil
}

// RevokeClerkUserSessions queries Clerk to find the user's Clerk ID by email,
// and then calls Clerk's logout endpoint to invalidate all active Clerk sessions immediately.
func RevokeClerkUserSessions(email string) error {
	cfg := config.Load()
	if cfg.ClerkSecretKey == "" {
		return errors.New("CLERK_SECRET_KEY is not configured")
	}

	// 1. Get user from Clerk by email
	clerkQueryURL := fmt.Sprintf("https://api.clerk.com/v1/users?email_address=%s", url.QueryEscape(email))
	req, err := http.NewRequest("GET", clerkQueryURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.ClerkSecretKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("clerk lookup returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var users []struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return err
	}

	if len(users) == 0 {
		return fmt.Errorf("user with email %s not found in Clerk", email)
	}

	clerkUserID := users[0].ID

	// 2. Call Clerk logout endpoint to revoke all sessions
	logoutURL := fmt.Sprintf("https://api.clerk.com/v1/users/%s/logout", clerkUserID)
	logoutReq, err := http.NewRequest("POST", logoutURL, nil)
	if err != nil {
		return err
	}
	logoutReq.Header.Set("Authorization", "Bearer "+cfg.ClerkSecretKey)
	logoutReq.Header.Set("Accept", "application/json")

	logoutResp, err := client.Do(logoutReq)
	if err != nil {
		return err
	}
	defer logoutResp.Body.Close()

	if logoutResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(logoutResp.Body)
		return fmt.Errorf("clerk logout returned status %d: %s", logoutResp.StatusCode, string(bodyBytes))
	}

	return nil
}
