package protected

import (
	"testing"
	"time"

	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Regression test: the Updates(map[string]interface{}{"activeSubscriptionId": ...})
// call used the struct's JSON tag names instead of the real DB columns
// (active_subscription_id, subscription_expires_at). GORM can't resolve an
// unrecognized map key to a schema field, so it fell back to treating it as
// a literal (wrong) column name and the whole UPDATE — and therefore the
// enclosing transaction — failed. This affected subscription creation,
// cancellation, and Paymob-driven subscription payment processing.
func TestCreateSubscriptionAndPayment_LinksUserToSubscription(t *testing.T) {
	testDB := setupTestDB(t)
	original := db.DB
	db.DB = testDB
	t.Cleanup(func() { db.DB = original })

	require.NoError(t, testDB.AutoMigrate(&models.User{}, &models.UserSubscription{}, &models.Payment{}, &models.Invoice{}))

	name := "Test User"
	user := models.User{ID: uuid.NewString(), Email: uuid.NewString() + "@example.com", Name: &name}
	require.NoError(t, testDB.Create(&user).Error)

	plan := models.SubscriptionPlan{ID: uuid.NewString(), Interval: "monthly", Currency: "EGP"}

	err := testDB.Transaction(func(tx *gorm.DB) error {
		return createSubscriptionRecords(tx, user.ID, plan, 99.0, uuid.NewString())
	})
	require.NoError(t, err, "subscription creation must not fail due to a bad column name")

	var after models.User
	require.NoError(t, testDB.First(&after, "id = ?", user.ID).Error)
	require.NotNil(t, after.ActiveSubscriptionID, "user must be linked to the newly created subscription")
	require.WithinDuration(t, time.Now().Add(30*24*time.Hour), *after.SubscriptionExpiresAt, 5*time.Second)
}
