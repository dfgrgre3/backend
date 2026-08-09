package admin

import (
	"testing"

	models "thanawy-backend/internal/domain/common"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Discard,
	})
	require.NoError(t, err)

	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	err = database.AutoMigrate(
		&models.Category{},
		&models.Subject{},
		&models.User{},
		&models.UserCredential{},
		&models.Achievement{},
		&models.Reward{},
		&models.Season{},
		&models.Coupon{},
		&models.Challenge{},
		&models.BlogPost{},
		&models.ForumTopic{},
		&models.ForumCategory{},
		&models.ABExperiment{},
		&models.Book{},
		&models.Campaign{},
		&models.Automation{},
		&models.AuditLog{},
	)
	require.NoError(t, err)

	return database
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// ptr returns a pointer to the given string value (test helper).
func ptr(s string) *string {
	return &s
}
