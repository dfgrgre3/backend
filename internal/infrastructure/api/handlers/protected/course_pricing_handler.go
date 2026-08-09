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

// SetPricing creates or updates the pricing configuration for a course.
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

	pricing := &models.LmsPricing{
		CourseID:                 parsedCourseID,
		Type:                     models.PriceType(req.Type),
		Amount:                   decimal.NewFromFloat(req.Amount),
		CurrencyCode:             req.CurrencyCode,
		SubscriptionDurationDays: req.SubscriptionDurationDays,
		DiscountPrice:            decimalPtrFromFloatPtr(req.DiscountPrice),
		SubscriptionPlanID:       req.SubscriptionPlanID,
		IsActive:                 true,
	}
	if req.DiscountStartAt != nil {
		discountStartAt := time.Unix(*req.DiscountStartAt, 0)
		pricing.DiscountStartAt = &discountStartAt
	}
	if req.DiscountEndAt != nil {
		discountEndAt := time.Unix(*req.DiscountEndAt, 0)
		pricing.DiscountEndAt = &discountEndAt
	}

	result, err := h.courseService.AddPricing(parsedCourseID, pricing.Type, pricing.Amount.InexactFloat64(), pricing.CurrencyCode, pricing.SubscriptionDurationDays)
	if err != nil {
		api_response.Error(c, http.StatusInternalServerError, "Failed to set pricing")
		return
	}

	api_response.Success(c, gin.H{"pricing": result})
}
