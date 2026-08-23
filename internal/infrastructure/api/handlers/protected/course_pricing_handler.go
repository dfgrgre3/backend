package protected

import (
	"net/http"
	"time"

	models "thanawy-backend/internal/domain/common"
	api_response "thanawy-backend/internal/infrastructure/api/response"

	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// =============================================================
// Course Pricing Endpoints
// =============================================================

// SetPricingRequest represents the REST request body for course pricing.
type SetPricingRequest struct {
	Type                     string   `json:"type" binding:"required"`
	Amount                   float64  `json:"amount"`
	CurrencyCode             string   `json:"currencyCode" binding:"required"`
	SubscriptionDurationDays *int     `json:"subscriptionDurationDays"`
	DiscountPrice            *float64 `json:"discountPrice"`
	DiscountStartAt          *int64   `json:"discountStartAt"`
	DiscountEndAt            *int64   `json:"discountEndAt"`
	SubscriptionPlanID       *string  `json:"subscriptionPlanId"`
}

// GetPricing returns the pricing configuration for a course.
func (h *CourseRESTHandler) GetPricing(c *gin.Context) {
	courseID := c.Param("id")
	if _, err := uuid.Parse(courseID); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid course ID")
		return
	}

	pricings, err := h.courseService.ListPricings(uuid.MustParse(courseID))
	var pricing *models.LmsPricing
	if err == nil && len(pricings) > 0 {
		pricing = &pricings[0]
	}
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			api_response.Error(c, http.StatusNotFound, "Pricing not found")
			return
		}
		api_response.Error(c, http.StatusInternalServerError, "Failed to fetch pricing")
		return
	}

	api_response.Success(c, gin.H{"pricing": pricing})
}

// SetPricing creates or updates the pricing configuration for a course. A
// course has at most one active pricing row here, so this upserts: if one
// already exists it is loaded and saved in place (preserving its ID), rather
// than inserting a new row on every save.
func (h *CourseRESTHandler) SetPricing(c *gin.Context) {
	courseID := c.Param("id")
	parsedCourseID, err := uuid.Parse(courseID)
	if err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid course ID")
		return
	}

	var req SetPricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api_response.Error(c, http.StatusBadRequest, "Invalid input: "+err.Error())
		return
	}
	if req.Amount < 0 || (req.DiscountPrice != nil && *req.DiscountPrice < 0) ||
		(req.SubscriptionDurationDays != nil && *req.SubscriptionDurationDays < 0) {
		api_response.Error(c, http.StatusBadRequest, "Pricing values cannot be negative")
		return
	}

	existing, err := h.courseService.ListPricings(parsedCourseID)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to load existing pricing")
		return
	}

	var pricing *models.LmsPricing
	if len(existing) > 0 {
		pricing = &existing[0]
	} else {
		pricing = &models.LmsPricing{CourseID: parsedCourseID}
	}

	pricing.Type = models.PriceType(req.Type)
	pricing.Amount = decimal.NewFromFloat(req.Amount)
	pricing.CurrencyCode = req.CurrencyCode
	pricing.SubscriptionDurationDays = req.SubscriptionDurationDays
	pricing.DiscountPrice = decimalPtrFromFloatPtr(req.DiscountPrice)
	pricing.SubscriptionPlanID = req.SubscriptionPlanID
	pricing.IsActive = true
	pricing.DiscountStartAt = nil
	if req.DiscountStartAt != nil {
		discountStartAt := time.Unix(*req.DiscountStartAt, 0)
		pricing.DiscountStartAt = &discountStartAt
	}
	pricing.DiscountEndAt = nil
	if req.DiscountEndAt != nil {
		discountEndAt := time.Unix(*req.DiscountEndAt, 0)
		pricing.DiscountEndAt = &discountEndAt
	}

	var result *models.LmsPricing
	if pricing.ID != uuid.Nil {
		result, err = h.courseService.UpdatePricing(pricing)
	} else {
		result, err = h.courseService.CreatePricing(pricing)
	}
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to set pricing")
		return
	}

	api_response.Success(c, gin.H{"pricing": result})
}
