package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Affiliate represents an affiliate marketer in the referral/affiliate system.
type Affiliate struct {
	ID             string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID         string         `gorm:"not null;uniqueIndex;type:uuid;column:user_id;constraint:OnDelete:CASCADE" json:"userId"`
	Code           string         `gorm:"not null;uniqueIndex;column:code" json:"code"`
	Status         string         `gorm:"default:'ACTIVE';index;column:status" json:"status"`
	CommissionRate float64        `gorm:"default:10;column:commission_rate" json:"commissionRate"`
	Tier           string         `gorm:"default:'BRONZE';index;column:tier" json:"tier"`
	TotalEarned    float64        `gorm:"default:0;column:total_earned" json:"totalEarned"`
	TotalPaid      float64        `gorm:"default:0;column:total_paid" json:"totalPaid"`
	CreatedAt      time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt      time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	User *User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user"`
}

// AffiliateReferral represents a single referral/commission record for an affiliate.
type AffiliateReferral struct {
	ID          string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	AffiliateID string         `gorm:"not null;index;type:uuid;column:affiliate_id;constraint:OnDelete:CASCADE" json:"affiliateId"`
	UserID      string         `gorm:"index;type:uuid;column:user_id;constraint:OnDelete:SET NULL" json:"userId"`
	Amount      float64        `gorm:"not null;column:amount" json:"amount"`
	Commission  float64        `gorm:"not null;column:commission" json:"commission"`
	Status      string         `gorm:"default:'PENDING';index;column:status" json:"status"`
	CreatedAt   time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt   time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	Affiliate *Affiliate `gorm:"foreignKey:AffiliateID;constraint:OnDelete:CASCADE" json:"-"`
	User      *User      `gorm:"foreignKey:UserID;constraint:OnDelete:SET NULL" json:"user"`
}

func (Affiliate) TableName() string { return "Affiliate" }
func (a *Affiliate) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.Status == "" {
		a.Status = "ACTIVE"
	}
	if a.Tier == "" {
		a.Tier = "BRONZE"
	}
	return nil
}

func (AffiliateReferral) TableName() string { return "AffiliateReferral" }
func (r *AffiliateReferral) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	if r.Status == "" {
		r.Status = "PENDING"
	}
	return nil
}
