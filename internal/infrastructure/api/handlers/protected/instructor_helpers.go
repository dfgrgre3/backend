package protected

import (
	"errors"
	"net/mail"
	"strconv"
	"strings"
	models "thanawy-backend/internal/domain/common"

	"github.com/gin-gonic/gin"
)

const (
	instructorStatusPending     = "PENDING"
	instructorStatusUnderReview = "UNDER_REVIEW"
	instructorStatusApproved    = "APPROVED"
	instructorStatusRejected    = "REJECTED"
	instructorStatusSuspended   = "SUSPENDED"

	defaultPage      = 1
	defaultLimit     = 10
	maxLimit         = 100
	maxExportLimit   = 10000
	maxBulkDeleteIDs = 100

	maxNameLength        = 150
	maxUsernameLength    = 50
	maxEmailLength       = 254
	maxBioLength         = 2000
	maxPhoneLength       = 30
	maxCountryLength     = 80
	maxArrayItems        = 30
	maxArrayItemLength   = 80
	maxExperienceYears   = 100
	maxCommissionPercent = 100
)

type instructorStatusCount struct {
	InstructorStatus string `gorm:"column:instructor_status"`
	Count            int64  `gorm:"column:count"`
}

type instructorUpdateInput struct {
	Name            *string   `json:"name"`
	Email           *string   `json:"email"`
	Username        *string   `json:"username"`
	Bio             *string   `json:"bio"`
	Status          *string   `json:"status"`
	Specialties     *[]string `json:"specialties"`
	Languages       *[]string `json:"languages"`
	CommissionRate  *float64  `json:"commissionRate"`
	Phone           *string   `json:"phone"`
	Country         *string   `json:"country"`
	ExperienceYears *int      `json:"experience"`
}

type instructorReviewInput struct {
	Status string `json:"status"`
	Notes  string `json:"notes"`
}

type instructorViolationInput struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

type instructorContractInput struct {
	Title string `json:"title"`
	Notes string `json:"notes"`
}

type instructorNotificationInput struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

func validInstructorStatus(status string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case instructorStatusUnderReview:
		return instructorStatusUnderReview, true
	case instructorStatusApproved:
		return instructorStatusApproved, true
	case instructorStatusRejected:
		return instructorStatusRejected, true
	case instructorStatusSuspended:
		return instructorStatusSuspended, true
	case instructorStatusPending, "":
		return instructorStatusPending, true
	default:
		return "", false
	}
}

func normalizeInstructorStatus(status string) string {
	if normalized, ok := validInstructorStatus(status); ok {
		return normalized
	}
	return instructorStatusPending
}

func normalizeInstructorStatusDefault(status string) string {
	return normalizeInstructorStatus(status)
}

func getInstructorStatus(user *models.User) string {
	if user == nil {
		return instructorStatusPending
	}
	return normalizeInstructorStatusDefault(user.InstructorStatus)
}

func parsePagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", strconv.Itoa(defaultPage)))
	if page < 1 {
		page = defaultPage
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	return page, limit
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
	)
	return replacer.Replace(value)
}

func stringPtrToString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func truncateString(value string, max int) string {
	if max <= 0 {
		return ""
	}

	runes := []rune(value)
	if len(runes) <= max {
		return value
	}

	return string(runes[:max])
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", errors.New("email is required")
	}
	if len(email) > maxEmailLength {
		return "", errors.New("email is too long")
	}

	addr, err := mail.ParseAddress(email)
	if err != nil {
		return "", errors.New("invalid email")
	}

	return strings.ToLower(strings.TrimSpace(addr.Address)), nil
}

func normalizeInstructorName(name, fallbackEmail string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = strings.Split(fallbackEmail, "@")[0]
	}
	name = truncateString(name, maxNameLength)
	if strings.TrimSpace(name) == "" {
		name = "Instructor"
	}
	return name
}

func normalizeInstructorOptionalText(value string, maxLen int) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) > maxLen {
		return "", errors.New("value is too long")
	}
	return value, nil
}

func normalizeInstructorExperienceYears(value *int) (int, bool, error) {
	if value == nil {
		return 0, false, nil
	}
	experienceYears := *value
	if experienceYears < 0 || experienceYears > maxExperienceYears {
		return 0, false, errors.New("invalid experience years")
	}
	return experienceYears, true, nil
}

func normalizeInstructorCommissionRate(value *float64) (float64, bool, error) {
	if value == nil {
		return 0, false, nil
	}
	commissionRate := *value
	if commissionRate < 0 || commissionRate > maxCommissionPercent {
		return 0, false, errors.New("commission rate must be between 0 and 100")
	}
	return commissionRate, true, nil
}

func buildInstructorUpdateMap(input instructorUpdateInput, _ *models.User) (map[string]interface{}, error) {
	updates := map[string]interface{}{}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, errors.New("name cannot be empty")
		}
		if len(name) > maxNameLength {
			return nil, errors.New("name is too long")
		}
		updates["name"] = name
	}

	if input.Email != nil {
		email, err := normalizeEmail(*input.Email)
		if err != nil {
			return nil, err
		}
		updates["email"] = email
	}

	if input.Username != nil {
		username := normalizeUsername(*input.Username)
		if username == "" {
			return nil, errors.New("username cannot be empty")
		}
		updates["username"] = username
	}

	if input.Bio != nil {
		bio := strings.TrimSpace(*input.Bio)
		if len(bio) > maxBioLength {
			return nil, errors.New("bio is too long")
		}
		if bio == "" {
			updates["bio"] = nil
		} else {
			updates["bio"] = bio
		}
	}

	if input.Phone != nil {
		phone := strings.TrimSpace(*input.Phone)
		if len(phone) > maxPhoneLength {
			return nil, errors.New("phone is too long")
		}
		if phone == "" {
			updates["phone"] = nil
		} else {
			updates["phone"] = phone
		}
	}

	if input.Country != nil {
		country := strings.TrimSpace(*input.Country)
		if len(country) > maxCountryLength {
			return nil, errors.New("country is too long")
		}
		if country == "" {
			updates["country"] = nil
		} else {
			updates["country"] = country
		}
	}

	if input.CommissionRate != nil {
		commissionRate := *input.CommissionRate
		if commissionRate < 0 || commissionRate > maxCommissionPercent {
			return nil, errors.New("commission rate must be between 0 and 100")
		}
		updates["commission_rate"] = commissionRate
	}

	if input.Specialties != nil {
		specialties, err := validateStringArray(*input.Specialties, maxArrayItems, maxArrayItemLength)
		if err != nil {
			return nil, errors.New("invalid specialties")
		}
		updates["instructor_specialties"] = models.JSONStringArray(specialties)
	}

	if input.Languages != nil {
		languages, err := validateStringArray(*input.Languages, maxArrayItems, maxArrayItemLength)
		if err != nil {
			return nil, errors.New("invalid languages")
		}
		updates["instructor_languages"] = models.JSONStringArray(languages)
	}

	if input.ExperienceYears != nil {
		experienceYears := *input.ExperienceYears
		if experienceYears < 0 || experienceYears > maxExperienceYears {
			return nil, errors.New("invalid experience years")
		}
		updates["experience_years"] = strconv.Itoa(experienceYears)
	}

	return updates, nil
}

func normalizeUsername(username string) string {
	username = strings.ToLower(strings.TrimSpace(username))
	username = strings.ReplaceAll(username, " ", ".")
	username = strings.Trim(username, "._-")
	username = truncateString(username, maxUsernameLength)
	username = strings.Trim(username, "._-")
	return username
}

func validateStringArray(values []string, maxItems, maxItemLength int) ([]string, error) {
	cleaned := make([]string, 0, len(values))

	for _, value := range values {
		item := strings.TrimSpace(value)
		if item == "" {
			continue
		}
		if len(item) > maxItemLength {
			return nil, errors.New("array item is too long")
		}
		cleaned = append(cleaned, item)
	}

	if len(cleaned) > maxItems {
		return nil, errors.New("too many array items")
	}

	return cleaned, nil
}

func experienceYearsToInt(years *string) int {
	if years == nil {
		return 0
	}

	value, err := strconv.Atoi(*years)
	if err != nil {
		return 0
	}

	return value
}
