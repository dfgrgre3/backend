package db

import (
	"fmt"
	"log"
	"os"

	"thanawy-backend/internal/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Seed populates initial application data using an explicit database dependency.
// It deliberately lives outside db.go so connection lifecycle and data bootstrap
// remain separate responsibilities.
func Seed(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("seed database is required")
	}
	log.Println("Seeding database...")

	if err := seedCategories(database); err != nil {
		return err
	}
	if err := seedSystemSettings(database); err != nil {
		return err
	}
	return seedAdminUser(database)
}

func tableExists(database *gorm.DB, tableName string) bool {
	return database.Migrator().HasTable(tableName)
}

func seedCategories(database *gorm.DB) error {
	if !tableExists(database, "Category") {
		log.Println("Category table not found, skipping category seeding")
		return nil
	}

	categories := []models.Category{
		{Name: "كتب مدرسية", Slug: "textbooks", Type: models.CategoryTypeLibrary},
		{Name: "ملخصات", Slug: "summaries", Type: models.CategoryTypeLibrary},
		{Name: "مراجعات نهائية", Slug: "final-reviews", Type: models.CategoryTypeLibrary},
		{Name: "أسئلة واختبارات", Slug: "questions-and-exams", Type: models.CategoryTypeLibrary},
	}

	for index := range categories {
		category := &categories[index]
		var existing models.Category
		err := database.Where("slug = ? AND type = ?", category.Slug, category.Type).First(&existing).Error
		if err == nil {
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return fmt.Errorf("find category %q: %w", category.Slug, err)
		}
		if err := database.Create(category).Error; err != nil {
			return fmt.Errorf("create category %q: %w", category.Slug, err)
		}
		log.Printf("Created library category: %s", category.Name)
	}
	return nil
}

func seedSystemSettings(database *gorm.DB) error {
	if !tableExists(database, "SystemSetting") {
		log.Println("SystemSetting table not found, skipping settings seeding")
		return nil
	}

	var setting models.SystemSetting
	err := database.Where("key = ?", "admin_settings").First(&setting).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("find default system settings: %w", err)
	}

	defaultSettings := `{"siteName":"Thanawy","siteDescription":"منصة تعليمية لإدارة التعلم والمحتوى.","features":{"registration":true,"emailVerification":true,"engagement":true,"forum":true,"blog":true,"events":true,"aiAssistant":true}}`
	if err := database.Create(&models.SystemSetting{Key: "admin_settings", Value: defaultSettings}).Error; err != nil {
		return fmt.Errorf("create default system settings: %w", err)
	}
	log.Println("Created default admin settings")
	return nil
}

func seedAdminUser(database *gorm.DB) error {
	if !tableExists(database, "User") {
		log.Println("User table not found, skipping admin user seeding")
		return nil
	}

	email := os.Getenv("DEFAULT_ADMIN_EMAIL")
	if email == "" {
		email = "admin@thanawy.app"
	}
	password := os.Getenv("DEFAULT_ADMIN_PASSWORD")
	if password == "" {
		log.Println("WARNING: DEFAULT_ADMIN_PASSWORD not set. Skipping default admin user creation.")
		return nil
	}

	var admin models.User
	err := database.Unscoped().Where("email = ?", email).First(&admin).Error
	if err == nil {
		log.Printf("Default admin user already exists: %s", email)
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("find default admin: %w", err)
	}

	bcryptCost := 12
	if configured, value := getEnvInt("BCRYPT_COST"); configured {
		bcryptCost = value
	}
	if bcryptCost < bcrypt.MinCost || bcryptCost > 14 {
		return fmt.Errorf("BCRYPT_COST must be between %d and 14", bcrypt.MinCost)
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("hash default admin password: %w", err)
	}

	admin = models.User{Email: email, Role: models.RoleAdmin, Status: models.StatusActive}
	if err := database.Transaction(func(tx *gorm.DB) error {
		// User must be inserted first so its BeforeCreate hook assigns the UUID
		// referenced by UserCredential.UserID.
		if err := tx.Create(&admin).Error; err != nil {
			return fmt.Errorf("create default admin: %w", err)
		}
		credential := models.UserCredential{UserID: admin.ID, PasswordHash: string(hashedPassword)}
		if err := tx.Create(&credential).Error; err != nil {
			return fmt.Errorf("create default admin credential: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	log.Printf("Created default admin user: %s", email)
	return nil
}
