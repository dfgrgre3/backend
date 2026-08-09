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

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestAdminCreateCoupon_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/coupons", protected.AdminCreateCoupon)

	body := map[string]interface{}{
		"code":          "SAVE10",
		"discountType":  "percentage",
		"discountValue": 10.0,
		"isActive":      true,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/coupons", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAdminCreateCoupon_Duplicate(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.Coupon{
		Code:          "SAVE10",
		DiscountType:  "percentage",
		DiscountValue: decimal.NewFromFloat(10.0),
		IsActive:      true,
	})

	router := setupTestRouter()
	router.POST("/coupons", protected.AdminCreateCoupon)

	body := map[string]interface{}{
		"code":          "SAVE10",
		"discountType":  "percentage",
		"discountValue": 10.0,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/coupons", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestAdminGetCoupons_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.Coupon{
		Code:          "TEST10",
		DiscountType:  "percentage",
		DiscountValue: decimal.NewFromFloat(10.0),
		IsActive:      true,
	})

	router := setupTestRouter()
	router.GET("/coupons", protected.AdminGetCoupons)

	req := httptest.NewRequest(http.MethodGet, "/coupons", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminUpdateCoupon_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	coupon := models.Coupon{
		Code:          "OLD10",
		DiscountType:  "percentage",
		DiscountValue: decimal.NewFromFloat(10.0),
		IsActive:      true,
	}
	testDB.Create(&coupon)

	router := setupTestRouter()
	router.PATCH("/coupons/:id", protected.AdminUpdateCoupon)

	body := map[string]interface{}{
		"code": "NEW20",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/coupons/"+coupon.ID, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminDeleteCoupon_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	coupon := models.Coupon{
		Code:          "DEL10",
		DiscountType:  "percentage",
		DiscountValue: decimal.NewFromFloat(10.0),
		IsActive:      true,
	}
	testDB.Create(&coupon)

	router := setupTestRouter()
	router.DELETE("/coupons/:id", protected.AdminDeleteCoupon)

	req := httptest.NewRequest(http.MethodDelete, "/coupons/"+coupon.ID, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
