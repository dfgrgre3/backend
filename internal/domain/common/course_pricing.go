package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PricingType defines how a course is priced.
type PricingType string

const (
	PricingFree         PricingType = "FREE"
	PricingOneTime      PricingType = "ONE_TIME"
	PricingSubscription PricingType = "SUBSCRIPTION"
	PricingBundle       PricingType = "BUNDLE"
)

// Currency supported for course payments.
type Currency string

const (
	CurrencyEGP Currency = "EGP"
	CurrencyUSD Currency = "USD"
	CurrencyEUR Currency = "EUR"
	CurrencySAR Currency = "SAR"
	CurrencyAED Currency = "AED"
	CurrencyGBP Currency = "GBP"
)

// CoursePricing stores the complete pricing configuration for a course.
type CoursePricing struct {
	ID                 string      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SubjectID          string      `gorm:"uniqueIndex;type:uuid;column:subject_id" json:"subjectId"`
	PricingType        PricingType `gorm:"default:'ONE_TIME';column:pricing_type" json:"pricingType"`
	Price              float64     `gorm:"default:0;column:price" json:"price"`
	Currency           Currency    `gorm:"default:'EGP';column:currency" json:"currency"`
	DiscountPrice      *float64    `gorm:"column:discount_price" json:"discountPrice,omitempty"`
	DiscountStartAt    *time.Time  `gorm:"column:discount_start_at" json:"discountStartAt,omitempty"`
	DiscountEndAt      *time.Time  `gorm:"column:discount_end_at" json:"discountEndAt,omitempty"`
	SubscriptionPlanID *string     `gorm:"type:uuid;column:subscription_plan_id" json:"subscriptionPlanId,omitempty"`
	IsActive           bool        `gorm:"default:true;column:is_active" json:"isActive"`
	CreatedAt          time.Time   `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt          time.Time   `gorm:"column:updated_at" json:"updatedAt"`

	// Relations
	Subject       *Subject          `gorm:"foreignKey:SubjectID;constraint:OnDelete:CASCADE" json:"subject,omitempty"`
	Subscription  *SubscriptionPlan `gorm:"foreignKey:SubscriptionPlanID;constraint:OnDelete:SET NULL" json:"subscription,omitempty"`
	BundleCourses []BundleCourse    `gorm:"foreignKey:BundleID;constraint:OnDelete:CASCADE" json:"bundleCourses,omitempty"`
}

func (CoursePricing) TableName() string { return "course_pricing" }

func (cp *CoursePricing) BeforeCreate(tx *gorm.DB) error {
	if cp.ID == "" {
		cp.ID = uuid.New().String()
	}
	return nil
}

// IsDiscountActive returns true if a discount is currently active.
func (cp *CoursePricing) IsDiscountActive() bool {
	if cp.DiscountPrice == nil || *cp.DiscountPrice <= 0 {
		return false
	}
	now := time.Now()
	if cp.DiscountStartAt != nil && now.Before(*cp.DiscountStartAt) {
		return false
	}
	if cp.DiscountEndAt != nil && now.After(*cp.DiscountEndAt) {
		return false
	}
	return true
}

// EffectivePrice returns the discounted price if active, otherwise the regular price.
func (cp *CoursePricing) EffectivePrice() float64 {
	if cp.IsDiscountActive() {
		return *cp.DiscountPrice
	}
	return cp.Price
}
