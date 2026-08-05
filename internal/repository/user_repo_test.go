package repository

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"thanawy-backend/internal/db"
)

func TestFindByID_UsesUnscopedWhenDeletedAtMissing(t *testing.T) {
	localUserCache = sync.Map{}
	userDeletedAtOnce = sync.Once{}
	userDeletedAtColumnExists = false

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	err = database.Exec(`
		CREATE TABLE "User" (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			role TEXT NOT NULL DEFAULT 'STUDENT',
			status TEXT NOT NULL DEFAULT 'ACTIVE',
			email_verified BOOLEAN NOT NULL DEFAULT FALSE,
			phone_verified BOOLEAN NOT NULL DEFAULT FALSE,
			created_at DATETIME,
			updated_at DATETIME
		);
	`).Error
	require.NoError(t, err)

	userID := uuid.NewString()
	now := time.Now().UTC()
	err = database.Exec(`
		INSERT INTO "User" (id, email, role, status, email_verified, phone_verified, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?);
	`, userID, "test.user@example.com", "STUDENT", "ACTIVE", false, false, now, now).Error
	require.NoError(t, err)

	db.DB = database

	repo := NewUserRepository(database)
	user, err := repo.FindByID(userID)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, userID, user.ID)
	assert.Equal(t, "test.user@example.com", user.Email)
}
