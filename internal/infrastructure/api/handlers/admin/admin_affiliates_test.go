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

// ---------------------------------------------------------------------------
// Campaigns
// ---------------------------------------------------------------------------

func TestAdminCreateAffiliateCampaign_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouterWithUser("admin-user-id")
	router.POST("/campaigns", protected.AdminCreateAffiliateCampaign)

	body := map[string]interface{}{
		"name":           "Back to School 2026",
		"description":    "حملة العودة إلى المدارس",
		"status":         "active",
		"commissionRate": 15.0,
		"budget":         5000.0,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/campaigns", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, _ := resp["data"].(map[string]interface{})
	require.NotNil(t, data)
	assert.Equal(t, "back-to-school-2026", data["slug"])
	assert.Equal(t, "ACTIVE", data["status"])
}

func TestAdminCreateAffiliateCampaign_DuplicateSlug(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.AffiliateCampaign{
		Name:   "First Campaign",
		Slug:   "dup-slug",
		Status: "DRAFT",
	})

	router := setupTestRouter()
	router.POST("/campaigns", protected.AdminCreateAffiliateCampaign)

	body := map[string]interface{}{
		"name": "Second Campaign",
		"slug": "dup-slug",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/campaigns", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestAdminListAffiliateCampaigns_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.AffiliateCampaign{Name: "Alpha", Slug: "alpha", Status: "ACTIVE"})
	testDB.Create(&models.AffiliateCampaign{Name: "Beta", Slug: "beta", Status: "DRAFT"})

	router := setupTestRouter()
	router.GET("/campaigns", protected.AdminListAffiliateCampaigns)

	req := httptest.NewRequest(http.MethodGet, "/campaigns", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, _ := resp["data"].([]interface{})
	assert.Len(t, data, 2)
}

func TestAdminUpdateAffiliateCampaign_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	c := &models.AffiliateCampaign{Name: "Old", Slug: "old-name", Status: "DRAFT"}
	testDB.Create(c)

	router := setupTestRouter()
	router.PATCH("/campaigns/:id", protected.AdminUpdateAffiliateCampaign)

	body := map[string]interface{}{
		"name":   "Renamed",
		"status": "active",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/campaigns/"+c.ID, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, _ := resp["data"].(map[string]interface{})
	assert.Equal(t, "Renamed", data["name"])
	assert.Equal(t, "ACTIVE", data["status"])
}

func TestAdminDeleteAffiliateCampaign_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	c := &models.AffiliateCampaign{Name: "To Delete", Slug: "to-delete", Status: "DRAFT"}
	testDB.Create(c)

	router := setupTestRouter()
	router.DELETE("/campaigns/:id", protected.AdminDeleteAffiliateCampaign)

	req := httptest.NewRequest(http.MethodDelete, "/campaigns/"+c.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var count int64
	testDB.Model(&models.AffiliateCampaign{}).Where("id = ?", c.ID).Count(&count)
	// With soft-delete, count via Unscoped should be 1 but the default count is 0.
	assert.Equal(t, int64(0), count)
}

// ---------------------------------------------------------------------------
// Links
// ---------------------------------------------------------------------------

func TestAdminCreateAffiliateLink_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	aff := &models.Affiliate{UserID: "u1", Code: "AFF001", Status: "ACTIVE", Tier: "BRONZE"}
	testDB.Create(aff)

	router := setupTestRouter()
	router.POST("/links", protected.AdminCreateAffiliateLink)

	body := map[string]interface{}{
		"affiliateId":    aff.ID,
		"name":           "Homepage Banner",
		"destinationUrl": "https://example.com",
		"utmSource":      "twitter",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/links", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, _ := resp["data"].(map[string]interface{})
	assert.Equal(t, "homepage-banner", data["slug"])
}

func TestAdminCreateAffiliateLink_AffiliateNotFound(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/links", protected.AdminCreateAffiliateLink)

	body := map[string]interface{}{
		"affiliateId":    "non-existent-id",
		"name":           "Test Link",
		"destinationUrl": "https://example.com",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/links", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAdminListAffiliateLinks_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	aff := &models.Affiliate{UserID: "u1", Code: "AFF002", Status: "ACTIVE"}
	testDB.Create(aff)

	link1 := &models.AffiliateLink{
		AffiliateID:    aff.ID,
		Name:           "Link A",
		Slug:           "link-a",
		DestinationURL: "https://a.example.com",
		Active:         true,
	}
	link2 := &models.AffiliateLink{
		AffiliateID:    aff.ID,
		Name:           "Link B",
		Slug:           "link-b",
		DestinationURL: "https://b.example.com",
		Active:         true,
	}
	testDB.Create(link1)
	testDB.Create(link2)

	router := setupTestRouter()
	router.GET("/links", protected.AdminListAffiliateLinks)

	req := httptest.NewRequest(http.MethodGet, "/links", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, _ := resp["data"].([]interface{})
	assert.Len(t, data, 2)
}

func TestAdminUpdateAffiliateLink_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	aff := &models.Affiliate{UserID: "u1", Code: "AFF003"}
	testDB.Create(aff)

	link := &models.AffiliateLink{
		AffiliateID:    aff.ID,
		Name:           "Original",
		Slug:           "orig-link",
		DestinationURL: "https://orig.example.com",
		Active:         true,
	}
	testDB.Create(link)

	router := setupTestRouter()
	router.PATCH("/links/:id", protected.AdminUpdateAffiliateLink)

	body := map[string]interface{}{
		"name":   "Updated",
		"active": false,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/links/"+link.ID, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, _ := resp["data"].(map[string]interface{})
	assert.Equal(t, "Updated", data["name"])
	assert.Equal(t, false, data["active"])
}

func TestAdminDeleteAffiliateLink_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	aff := &models.Affiliate{UserID: "u1", Code: "AFF004"}
	testDB.Create(aff)

	link := &models.AffiliateLink{
		AffiliateID:    aff.ID,
		Name:           "To Delete",
		Slug:           "del-link",
		DestinationURL: "https://del.example.com",
		Active:         true,
	}
	testDB.Create(link)

	router := setupTestRouter()
	router.DELETE("/links/:id", protected.AdminDeleteAffiliateLink)

	req := httptest.NewRequest(http.MethodDelete, "/links/"+link.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminTrackAffiliateClick_IncrementsCounters(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	aff := &models.Affiliate{UserID: "u1", Code: "AFF005", ClicksCount: 0, ConversionsCount: 0}
	testDB.Create(aff)

	link := &models.AffiliateLink{
		AffiliateID:    aff.ID,
		Name:           "Tracked",
		Slug:           "track-link",
		DestinationURL: "https://track.example.com",
		Active:         true,
		ClicksCount:    0,
	}
	testDB.Create(link)

	router := setupTestRouter()
	router.POST("/links/:id/click", protected.AdminTrackAffiliateClick)

	body := map[string]interface{}{
		"ipHash":    "abc123",
		"userAgent": "Mozilla/5.0",
		"device":    "desktop",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/links/"+link.ID+"/click", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var updated models.AffiliateLink
	require.NoError(t, testDB.First(&updated, "id = ?", link.ID).Error)
	assert.Equal(t, 1, updated.ClicksCount)

	var updatedAff models.Affiliate
	require.NoError(t, testDB.First(&updatedAff, "id = ?", aff.ID).Error)
	assert.Equal(t, 1, updatedAff.ClicksCount)
}

// ---------------------------------------------------------------------------
// Payouts
// ---------------------------------------------------------------------------

func TestAdminListAffiliatePayouts_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	aff := &models.Affiliate{UserID: "u1", Code: "AFF006"}
	testDB.Create(aff)

	testDB.Create(&models.AffiliatePayout{AffiliateID: aff.ID, Amount: 100, Currency: "EGP", Status: "PENDING"})
	testDB.Create(&models.AffiliatePayout{AffiliateID: aff.ID, Amount: 200, Currency: "EGP", Status: "PAID"})

	router := setupTestRouter()
	router.GET("/payouts", protected.AdminListAffiliatePayouts)

	req := httptest.NewRequest(http.MethodGet, "/payouts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, _ := resp["data"].([]interface{})
	assert.Len(t, data, 2)
}

func TestAdminCreateAffiliatePayout_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	aff := &models.Affiliate{UserID: "u1", Code: "AFF007"}
	testDB.Create(aff)

	router := setupTestRouterWithUser("admin-user-id")
	router.POST("/payouts", protected.AdminCreateAffiliatePayout)

	body := map[string]interface{}{
		"affiliateId": aff.ID,
		"amount":      250.0,
		"currency":    "egp",
		"method":      "bank_transfer",
		"reference":   "REF-001",
		"notes":       "manual payout",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payouts", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, _ := resp["data"].(map[string]interface{})
	assert.Equal(t, "EGP", data["currency"])
	assert.Equal(t, "PENDING", data["status"])
}

func TestAdminCreateAffiliatePayout_AffiliateNotFound(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/payouts", protected.AdminCreateAffiliatePayout)

	body := map[string]interface{}{
		"affiliateId": "missing",
		"amount":      100.0,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payouts", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAdminMarkAffiliatePayoutPaid_UpdatesAffiliateTotals(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	aff := &models.Affiliate{
		UserID:      "u1",
		Code:        "AFF008",
		TotalEarned: 500,
		TotalPaid:   0,
	}
	testDB.Create(aff)

	payout := &models.AffiliatePayout{
		AffiliateID: aff.ID,
		Amount:      300,
		Currency:    "EGP",
		Status:      "PENDING",
	}
	testDB.Create(payout)

	router := setupTestRouterWithUser("admin-user-id")
	router.POST("/payouts/:id/status", protected.AdminMarkAffiliatePayoutPaid)

	body := map[string]interface{}{
		"status":    "paid",
		"reference": "TX-001",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payouts/"+payout.ID+"/status", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var updatedAff models.Affiliate
	require.NoError(t, testDB.First(&updatedAff, "id = ?", aff.ID).Error)
	assert.Equal(t, float64(300), updatedAff.TotalPaid)
}

func TestAdminMarkAffiliatePayoutPaid_InvalidStatus(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	aff := &models.Affiliate{UserID: "u1", Code: "AFF009"}
	testDB.Create(aff)

	payout := &models.AffiliatePayout{
		AffiliateID: aff.ID,
		Amount:      100,
		Currency:    "EGP",
		Status:      "PENDING",
	}
	testDB.Create(payout)

	router := setupTestRouter()
	router.POST("/payouts/:id/status", protected.AdminMarkAffiliatePayoutPaid)

	body := map[string]interface{}{
		"status": "BOGUS",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/payouts/"+payout.ID+"/status", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminProcessAffiliatePayouts_ConsumesPendingReferrals(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	aff := &models.Affiliate{UserID: "u1", Code: "AFF010", TotalEarned: 200, TotalPaid: 0}
	testDB.Create(aff)

	testDB.Create(&models.AffiliateReferral{AffiliateID: aff.ID, Amount: 100, Commission: 75, Status: "PENDING"})
	testDB.Create(&models.AffiliateReferral{AffiliateID: aff.ID, Amount: 100, Commission: 125, Status: "PENDING"})

	router := setupTestRouterWithUser("admin-user-id")
	router.POST("/payouts/process/:id", protected.AdminProcessAffiliatePayouts)

	req := httptest.NewRequest(http.MethodPost, "/payouts/process/"+aff.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// All pending referrals should now be PAID
	var pendingCount int64
	testDB.Model(&models.AffiliateReferral{}).
		Where("affiliate_id = ? AND status = ?", aff.ID, "PENDING").
		Count(&pendingCount)
	assert.Equal(t, int64(0), pendingCount)

	var updatedAff models.Affiliate
	require.NoError(t, testDB.First(&updatedAff, "id = ?", aff.ID).Error)
	assert.Equal(t, float64(200), updatedAff.TotalPaid)
}

func TestAdminProcessAffiliatePayouts_NoPending(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	aff := &models.Affiliate{UserID: "u1", Code: "AFF011"}
	testDB.Create(aff)

	router := setupTestRouter()
	router.POST("/payouts/process/:id", protected.AdminProcessAffiliatePayouts)

	req := httptest.NewRequest(http.MethodPost, "/payouts/process/"+aff.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// Tiers
// ---------------------------------------------------------------------------

func TestAdminUpsertAffiliateTier_Creates(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.POST("/tiers", protected.AdminUpsertAffiliateTier)

	body := map[string]interface{}{
		"tier":           "GOLD",
		"nameAr":         "الذهبي",
		"commissionRate": 20,
		"minRevenue":     1000,
		"minReferrals":   10,
		"color":          "yellow",
		"sortOrder":      2,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/tiers", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, _ := resp["data"].(map[string]interface{})
	assert.Equal(t, "GOLD", data["tier"])
	assert.Equal(t, "الذهبي", data["nameAr"])
}

func TestAdminUpsertAffiliateTier_UpdatesExisting(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.AffiliateTierRule{
		Tier:           "SILVER",
		NameAr:         "الفضي",
		CommissionRate: 15,
		SortOrder:      1,
		Color:          "gray",
		Active:         true,
	})

	router := setupTestRouter()
	router.POST("/tiers", protected.AdminUpsertAffiliateTier)

	body := map[string]interface{}{
		"tier":           "silver",
		"nameAr":         "الفضي المحدث",
		"commissionRate": 18,
		"minReferrals":   5,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/tiers", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var count int64
	testDB.Model(&models.AffiliateTierRule{}).Where("tier = ?", "SILVER").Count(&count)
	assert.Equal(t, int64(1), count)

	var tier models.AffiliateTierRule
	require.NoError(t, testDB.Where("tier = ?", "SILVER").First(&tier).Error)
	assert.Equal(t, "الفضي المحدث", tier.NameAr)
	assert.Equal(t, float64(18), tier.CommissionRate)
	assert.Equal(t, 5, tier.MinReferrals)
}

func TestAdminListAffiliateTiers_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	testDB.Create(&models.AffiliateTierRule{Tier: "BRONZE", NameAr: "برونزي", CommissionRate: 10, SortOrder: 0})
	testDB.Create(&models.AffiliateTierRule{Tier: "GOLD", NameAr: "ذهبي", CommissionRate: 20, SortOrder: 2})
	testDB.Create(&models.AffiliateTierRule{Tier: "SILVER", NameAr: "فضي", CommissionRate: 15, SortOrder: 1})

	router := setupTestRouter()
	router.GET("/tiers", protected.AdminListAffiliateTiers)

	req := httptest.NewRequest(http.MethodGet, "/tiers", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, _ := resp["data"].([]interface{})
	require.Len(t, data, 3)

	// Verify sort order: BRONZE → SILVER → GOLD
	first, _ := data[0].(map[string]interface{})
	mid, _ := data[1].(map[string]interface{})
	last, _ := data[2].(map[string]interface{})
	assert.Equal(t, "BRONZE", first["tier"])
	assert.Equal(t, "SILVER", mid["tier"])
	assert.Equal(t, "GOLD", last["tier"])
}

func TestAdminDeleteAffiliateTier_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	tier := &models.AffiliateTierRule{
		Tier:           "PLATINUM",
		NameAr:         "بلاتيني",
		CommissionRate: 25,
		SortOrder:      3,
	}
	testDB.Create(tier)

	router := setupTestRouter()
	router.DELETE("/tiers/:id", protected.AdminDeleteAffiliateTier)

	req := httptest.NewRequest(http.MethodDelete, "/tiers/"+tier.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ---------------------------------------------------------------------------
// Settings
// ---------------------------------------------------------------------------

func TestAdminGetAffiliateSettings_ReturnsDefaults(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	router := setupTestRouter()
	router.GET("/settings", protected.AdminGetAffiliateSettings)

	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, _ := resp["data"].(map[string]interface{})
	assert.Equal(t, "default", data["key"])
	assert.Equal(t, float64(10), data["defaultCommissionRate"])
}

func TestAdminUpdateAffiliateSettings_Success(t *testing.T) {
	testDB := setupTestDB(t)
	db.DB = testDB

	// Seed the singleton row so the update has something to mutate
	testDB.Create(&models.AffiliateSetting{
		Key:                   "default",
		DefaultCommissionRate: 10,
		DefaultTier:           "BRONZE",
		AutoApprove:           true,
		MinimumPayout:         100,
		HoldDays:              14,
		CookieDays:            30,
		AllowSelfReferral:     false,
		NotifyOnSignup:        true,
		NotifyOnPayout:        true,
	})

	router := setupTestRouterWithUser("admin-user-id")
	router.PATCH("/settings", protected.AdminUpdateAffiliateSettings)

	body := map[string]interface{}{
		"defaultCommissionRate": 12.5,
		"minimumPayout":         150,
		"holdDays":              21,
		"cookieDays":            45,
		"autoApprove":           false,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/settings", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var s models.AffiliateSetting
	require.NoError(t, testDB.Where("key = ?", "default").First(&s).Error)
	assert.Equal(t, float64(12.5), s.DefaultCommissionRate)
	assert.Equal(t, float64(150), s.MinimumPayout)
	assert.Equal(t, 21, s.HoldDays)
	assert.Equal(t, 45, s.CookieDays)
	assert.Equal(t, false, s.AutoApprove)
}