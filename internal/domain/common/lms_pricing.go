package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// LmsPricing for multi-currency & price types
type LmsPricing struct {
	ID                       uuid.UUID        `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	CourseID                 uuid.UUID        `gorm:"not null;type:uuid;index;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	Type                     PriceType        `gorm:"default:'FREE';index;column:type" json:"type"`
	Amount                   decimal.Decimal  `gorm:"default:0;type:numeric(19,4);column:amount" json:"amount"`
	CurrencyCode             string           `gorm:"default:'USD';index;column:currency_code" json:"currencyCode"`
	SubscriptionDurationDays *int             `gorm:"column:subscription_duration_days" json:"subscriptionDurationDays,omitempty"`
	DiscountPrice            *decimal.Decimal `gorm:"type:numeric(19,4);column:discount_price" json:"discountPrice,omitempty"`
	DiscountStartAt          *time.Time       `gorm:"column:discount_start_at" json:"discountStartAt,omitempty"`
	DiscountEndAt            *time.Time       `gorm:"column:discount_end_at" json:"discountEndAt,omitempty"`
	SubscriptionPlanID       *string          `gorm:"column:subscription_plan_id" json:"subscriptionPlanId,omitempty"`
	IsActive                 bool             `gorm:"default:true;column:is_active" json:"isActive"`
	CreatedAt                time.Time        `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt                time.Time        `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt                gorm.DeletedAt   `gorm:"index;column:deleted_at" json:"-"`
}

func (LmsPricing) TableName() string {
	return "LmsPricing"
}

func (p *LmsPricing) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return
}
