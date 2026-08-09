package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	models "thanawy-backend/internal/domain/common"
	protected "thanawy-backend/internal/infrastructure/api/handlers/protected"
	db "thanawy-backend/internal/infrastructure/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateUser_CreatesCredential(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/users", protected.CreateUser)

	body := map[string]interface{}{
		"email":    "new.user@example.com",
		"name":     "New User",
		"password": "secure-password",
		"role":     "STUDENT",
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var user models.User
	require.NoError(t, testDB.Where("email = ?", body["email"]).First(&user).Error)

	var credential models.UserCredential
	require.NoError(t, testDB.Where("user_id = ?", user.ID).First(&credential).Error)
	assert.True(t, credential.CheckPassword(body["password"].(string)))
	assert.NotEqual(t, body["password"], credential.PasswordHash)
}

func TestVerifyUserEmailAndPhone_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	user := models.User{
		Email:         "verify.user@example.com",
		Name:          ptr("Verify User"),
		EmailVerified: false,
		PhoneVerified: false,
	}
	require.NoError(t, testDB.Create(&user).Error)

	router := setupTestRouter()
	router.POST("/users/:id/verify-email", protected.VerifyUserEmail)
	router.POST("/users/:id/verify-phone", protected.VerifyUserPhone)

	emailReq := httptest.NewRequest(http.MethodPost, "/users/"+user.ID+"/verify-email", nil)
	emailResp := httptest.NewRecorder()
	router.ServeHTTP(emailResp, emailReq)
	assert.Equal(t, http.StatusOK, emailResp.Code)

	var updatedEmailUser models.User
	require.NoError(t, testDB.First(&updatedEmailUser, "id = ?", user.ID).Error)
	assert.True(t, updatedEmailUser.EmailVerified)

	phoneReq := httptest.NewRequest(http.MethodPost, "/users/"+user.ID+"/verify-phone", nil)
	phoneResp := httptest.NewRecorder()
	router.ServeHTTP(phoneResp, phoneReq)
	assert.Equal(t, http.StatusOK, phoneResp.Code)

	var updatedPhoneUser models.User
	require.NoError(t, testDB.First(&updatedPhoneUser, "id = ?", user.ID).Error)
	assert.True(t, updatedPhoneUser.PhoneVerified)
}

func TestCreateUser_InvalidPasswordDoesNotCreateUser(t *testing.T) {
	testCases := []struct {
		name     string
		password interface{}
	}{
		{name: "missing", password: nil},
		{name: "too short", password: "short"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testDB := setupTestDB(t)
			db.DB = testDB

			router := setupTestRouter()
			router.POST("/users", protected.CreateUser)

			body := map[string]interface{}{
				"email": "invalid.password@example.com",
				"name":  "Invalid Password",
			}
			if testCase.password != nil {
				body["password"] = testCase.password
			}
			bodyBytes, err := json.Marshal(body)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var userCount int64
			require.NoError(t, testDB.Model(&models.User{}).Where("email = ?", body["email"]).Count(&userCount).Error)
			assert.Zero(t, userCount)
		})
	}
}
