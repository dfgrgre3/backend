package protected

import (
	"net/http"
	"net/http/httptest"
	"testing"

	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// Regression test for the payment webhook idempotency race: a payment must
// only ever be processed once, even if handleSuccessfulPayment/
// handleFailedPayment is invoked twice for the same payment (as happens
// when Paymob retries a webhook delivery, or two callbacks arrive
// concurrently). Before this fix, the status transition was an
// unconditional UPDATE guarded only by an earlier in-memory read, so a
// second call could re-run processPaymentItems (double wallet credit,
// double enrollment-count increment). The fix makes the transition an
// atomic conditional UPDATE ... WHERE status = 'pending', so only the
// first call's update actually affects a row.
func newWebhookTestGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/payments/webhook/paymob", nil)
	return c
}

func TestHandleSuccessfulPayment_IsIdempotentUnderDuplicateCallback(t *testing.T) {
	testDB := setupTestDB(t)
	original := db.DB
	db.DB = testDB
	t.Cleanup(func() { db.DB = original })

	require.NoError(t, testDB.AutoMigrate(&models.Payment{}))

	payment := models.Payment{
		ID:        uuid.NewString(),
		UserID:    uuid.NewString(),
		Amount:    decimal.NewFromInt(100),
		Currency:  "EGP",
		Status:    models.PaymentPending,
		Method:    "PAYMOB",
		Reference: uuid.NewString(),
	}
	require.NoError(t, testDB.Create(&payment).Error)

	c := newWebhookTestGinContext()

	// First callback: should transition pending -> completed and record
	// the first transaction ID.
	handleSuccessfulPayment(c, &payment, paymobTransactionData{Success: true, OrderID: 1, TxnID: 111})

	var afterFirst models.Payment
	require.NoError(t, testDB.First(&afterFirst, "id = ?", payment.ID).Error)
	require.Equal(t, models.PaymentCompleted, afterFirst.Status)
	require.Equal(t, "111", afterFirst.ExternalTxnID)

	// Second callback for the same payment (Paymob retry / concurrent
	// delivery), with a different transaction ID. If the idempotency guard
	// were broken (unconditional update), this would overwrite
	// ExternalTxnID with "222" and re-run item processing.
	handleSuccessfulPayment(c, &afterFirst, paymobTransactionData{Success: true, OrderID: 1, TxnID: 222})

	var afterSecond models.Payment
	require.NoError(t, testDB.First(&afterSecond, "id = ?", payment.ID).Error)
	require.Equal(t, models.PaymentCompleted, afterSecond.Status)
	require.Equal(t, "111", afterSecond.ExternalTxnID, "duplicate callback must not re-process an already-completed payment")
}

func TestHandleFailedPayment_IsIdempotentUnderDuplicateCallback(t *testing.T) {
	testDB := setupTestDB(t)
	original := db.DB
	db.DB = testDB
	t.Cleanup(func() { db.DB = original })

	require.NoError(t, testDB.AutoMigrate(&models.Payment{}))

	payment := models.Payment{
		ID:        uuid.NewString(),
		UserID:    uuid.NewString(),
		Amount:    decimal.NewFromInt(100),
		Currency:  "EGP",
		Status:    models.PaymentPending,
		Method:    "PAYMOB",
		Reference: uuid.NewString(),
	}
	require.NoError(t, testDB.Create(&payment).Error)

	handleFailedPayment(&payment, 1)

	var afterFirst models.Payment
	require.NoError(t, testDB.First(&afterFirst, "id = ?", payment.ID).Error)
	require.Equal(t, models.PaymentFailed, afterFirst.Status)

	// A completed payment must never be flipped back to failed by a late
	// duplicate "failed" callback either.
	afterFirst.Status = models.PaymentCompleted
	require.NoError(t, testDB.Model(&afterFirst).Update("status", models.PaymentCompleted).Error)

	handleFailedPayment(&afterFirst, 1)

	var afterSecond models.Payment
	require.NoError(t, testDB.First(&afterSecond, "id = ?", payment.ID).Error)
	require.Equal(t, models.PaymentCompleted, afterSecond.Status, "a completed payment must not be overwritten by a late failure callback")
}
