package protected

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	models "thanawy-backend/internal/domain/common"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTeachingListStudentsRejectsNonOwnerBeforeReadingEnrollments(t *testing.T) {
	originalDB := db.DB
	t.Cleanup(func() { db.DB = originalDB })

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&models.Subject{}))
	db.DB = database

	instructorID := "instructor-1"
	require.NoError(t, database.Create(&models.Subject{
		ID:           "course-1",
		Name:         "Course",
		InstructorId: &instructorID,
	}).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "id", Value: "course-1"}}
	context.Set("userId", "another-instructor")
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/teaching/courses/course-1/students", nil)

	TeachingListStudents(context)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestPaymentHistoryOnlyReturnsAuthenticatedUsersPayments(t *testing.T) {
	originalDB := db.DB
	t.Cleanup(func() { db.DB = originalDB })

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&models.Payment{}))
	db.DB = database

	for _, payment := range []models.Payment{
		{ID: "payment-1", UserID: "user-1", PlanID: "plan-1", Amount: decimal.NewFromInt(100), Currency: "EGP", Status: models.PaymentPending, Method: "card", Reference: "ref-user-1"},
		{ID: "payment-2", UserID: "user-2", PlanID: "plan-2", Amount: decimal.NewFromInt(200), Currency: "EGP", Status: models.PaymentPending, Method: "card", Reference: "ref-user-2"},
	} {
		require.NoError(t, database.Create(&payment).Error)
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set("userId", "user-1")
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/payments/history", nil)

	GetPaymentHistory(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Data []models.Payment `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Data, 1)
	assert.Equal(t, "user-1", response.Data[0].UserID)
	assert.Equal(t, "ref-user-1", response.Data[0].Reference)
}

func TestDeleteUploadRejectsManagedKeyWithoutResourcesPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/upload", bytes.NewBufferString(`{"fileKey":"avatars/another-user.png"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("permissions", []string{"resources:view"})

	DeleteUpload(context)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestLessonProgressDoesNotExposeAnotherUsersProgress(t *testing.T) {
	originalDB := db.DB
	t.Cleanup(func() { db.DB = originalDB })

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&models.LessonProgress{}))
	db.DB = database
	require.NoError(t, database.Create(&models.LessonProgress{
		UserID:    "user-1",
		LessonID:  "lesson-1",
		Completed: true,
		Status:    models.ProgressStatusCompleted,
	}).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "id", Value: "lesson-1"}}
	context.Set("userId", "user-2")
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/courses/lessons/lesson-1/progress", nil)

	GetLessonProgress(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Data struct {
			Completed bool `json:"completed"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Data.Completed)
}
