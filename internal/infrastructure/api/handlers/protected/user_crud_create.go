package protected

import (
	"net/http"
	"strings"
	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"
	db "thanawy-backend/internal/infrastructure/database"

	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func CreateUser(c *gin.Context) {
	var input struct {
		Email       string  `json:"email" binding:"required,email"`
		Password    string  `json:"password" binding:"required,min=8"`
		Name        *string `json:"name"`
		Username    *string `json:"username"`
		Role        string  `json:"role"`
		Phone       *string `json:"phone"`
		Status      string  `json:"status"`
		FirstName   *string `json:"firstName"`
		LastName    *string `json:"lastName"`
		Country     *string `json:"country"`
		City        *string `json:"city"`
		DateOfBirth *string `json:"dateOfBirth"`
		Gender      *string `json:"gender"`
		Language    *string `json:"language"`
		Timezone    *string `json:"timezone"`
		Bio         *string `json:"bio"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		api_response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	role := models.RoleStudent
	if input.Role != "" {
		validRoles := map[string]bool{"STUDENT": true, "TEACHER": true, "MODERATOR": true, "ADMIN": true, "SUPPORT": true, "SUPER_ADMIN": true, "PARENT": true}
		if !validRoles[input.Role] {
			api_response.Error(c, http.StatusBadRequest, "Invalid role")
			return
		}
		role = models.UserRole(input.Role)
	}

	status := models.StatusActive
	if input.Status != "" {
		validStatuses := map[string]bool{"ACTIVE": true, "INACTIVE": true, "SUSPENDED": true, "BANNED": true, "PENDING_VERIFICATION": true}
		if !validStatuses[input.Status] {
			api_response.Error(c, http.StatusBadRequest, "Invalid status")
			return
		}
		status = models.UserStatus(input.Status)
	}

	// Build name from firstName + lastName if provided, otherwise use input.Name
	name := input.Name
	if input.FirstName != nil || input.LastName != nil {
		firstName := ""
		lastName := ""
		if input.FirstName != nil {
			firstName = *input.FirstName
		}
		if input.LastName != nil {
			lastName = *input.LastName
		}
		fullName := strings.TrimSpace(firstName + " " + lastName)
		if fullName != "" {
			name = &fullName
		}
	}

	// Parse date of birth if provided
	var dateOfBirth *time.Time
	if input.DateOfBirth != nil && *input.DateOfBirth != "" {
		parsed, err := time.Parse("2006-01-02", *input.DateOfBirth)
		if err != nil {
			api_response.Error(c, http.StatusBadRequest, "Invalid date of birth format. Use YYYY-MM-DD")
			return
		}
		dateOfBirth = &parsed
	}

	// Validate gender if provided
	if input.Gender != nil && *input.Gender != "" {
		validGenders := map[string]bool{"male": true, "female": true, "other": true}
		if !validGenders[*input.Gender] {
			api_response.Error(c, http.StatusBadRequest, "Invalid gender. Must be male, female, or other")
			return
		}
	}

	// Local provisioning
	user := models.User{
		Email:       input.Email,
		Name:        name,
		Username:    input.Username,
		Role:        role,
		Status:      status,
		Phone:       input.Phone,
		Country:     input.Country,
		DateOfBirth: dateOfBirth,
		Bio:         input.Bio,
	}

	var existingUser models.User
	if err := db.DB.Where("email = ?", input.Email).First(&existingUser).Error; err == nil {
		api_response.Error(c, http.StatusConflict, "User with this email already exists")
		return
	}

	// Check username uniqueness if provided
	if input.Username != nil && *input.Username != "" {
		var existingUsername models.User
		if err := db.DB.Where("username = ?", *input.Username).First(&existingUsername).Error; err == nil {
			api_response.Error(c, http.StatusConflict, "User with this username already exists")
			return
		}
	}

	// Check phone uniqueness if provided
	if input.Phone != nil && *input.Phone != "" {
		var existingPhone models.User
		if err := db.DB.Where("phone = ?", *input.Phone).First(&existingPhone).Error; err == nil {
			api_response.Error(c, http.StatusConflict, "User with this phone number already exists")
			return
		}
	}

	credential := models.UserCredential{}
	if err := credential.SetPassword(input.Password); err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to create user")
		return
	}

	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := SafeCreate(tx, &user); err != nil {
			return err
		}
		credential.UserID = user.ID
		return SafeCreate(tx, &credential)
	}); err != nil {
		if IsDuplicateKeyError(err) {
			api_response.Error(c, http.StatusConflict, "User with this email already exists")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to create user")
		return
	}

	LogAudit(c, "CREATE", "user", user.ID, gin.H{"email": user.Email, "role": user.Role})
	api_response.Created(c, user)
}
