package protected

import (
	"sync"
	"testing"

	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
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
		&models.Book{},
		&models.AuditLog{},
	)
	require.NoError(t, err)

	return database
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestResolveBookListOrder(t *testing.T) {
	original := db.DB
	t.Cleanup(func() {
		db.DB = original
		bookListOrderOnce = sync.Once{}
		bookListOrderValue = ""
	})

	db.DB = setupTestDB(t)
	bookListOrderOnce = sync.Once{}
	resolveBookListOrder()
	assert.Equal(t, "created_at DESC", bookListOrderValue)

	db.DB = nil
	bookListOrderOnce = sync.Once{}
	resolveBookListOrder()
	assert.Equal(t, `"createdAt" DESC`, bookListOrderValue)
}
