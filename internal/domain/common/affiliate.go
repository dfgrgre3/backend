package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// JSONArray is a custom type for storing JSON arrays of strings.
type JSONArray []string

// Value implements the driver.Valuer interface
func (a JSONArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	return json.Marshal(a)
}

// Scan implements the sql.Scanner interface
func (a *JSONArray) Scan(value interface{}) error {
	if value == nil {
		*a = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, a)
}

// Affiliate represents an affiliate marketer in the referral/affiliate system.
type Affiliate struct {
	ID                string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID            string         `gorm:"not null;uniqueIndex;type:uuid;column:user_id;constraint:OnDelete:CASCADE" json:"userId"`
	Code              string         `gorm:"not null;uniqueIndex;column:code" json:"code"`
	Status            string         `gorm:"default:'PENDING';index;column:status" json:"status"`
	CommissionRate    float64        `gorm:"default:10;column:commission_rate" json:"commissionRate"`
	Tier              string         `gorm:"default:'BRONZE';index;column:tier" json:"tier"`
	TotalEarned       float64        `gorm:"default:0;column:total_earned" json:"totalEarned"`
	TotalPaid         float64        `gorm:"default:0;column:total_paid" json:"totalPaid"`
	ApprovedAt        *time.Time     `gorm:"column:approved_at" json:"approvedAt,omitempty"`
	ApprovedBy        *string        `gorm:"type:uuid;column:approved_by" json:"approvedBy,omitempty"`
	PayoutMethod      *string        `gorm:"column:payout_method" json:"payoutMethod,omitempty"`
	PayoutDetails     JSONMap        `gorm:"type:jsonb;column:payout_details;default:'{}'" json:"payoutDetails"`
	MinimumPayout     float64        `gorm:"default:0;column:minimum_payout" json:"minimumPayout"`
	HoldDays          int            `gorm:"default:0;column:hold_days" json:"holdDays"`
	ClicksCount       int            `gorm:"default:0;column:clicks_count" json:"clicksCount"`
	ConversionsCount  int            `gorm:"default:0;column:conversions_count" json:"conversionsCount"`
	LastActivityAt    *time.Time     `gorm:"column:last_activity_at" json:"lastActivityAt,omitempty"`
	Notes             *string        `gorm:"type:text;column:notes" json:"notes,omitempty"`
	CreatedAt         time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt         time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt         gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

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

// ---------------------------------------------------------------------------
// Advanced affiliate entities (migration 0112)
// ---------------------------------------------------------------------------

// AffiliateTierRule describes per-tier commission / bonus / qualification rules.
type AffiliateTierRule struct {
	ID             string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Tier           string         `gorm:"not null;uniqueIndex;column:tier" json:"tier"`
	NameAr         string         `gorm:"not null;column:name_ar" json:"nameAr"`
	CommissionRate float64        `gorm:"default:10;column:commission_rate" json:"commissionRate"`
	MinRevenue     float64        `gorm:"default:0;column:min_revenue" json:"minRevenue"`
	MinReferrals   int            `gorm:"default:0;column:min_referrals" json:"minReferrals"`
	BonusRate      float64        `gorm:"default:0;column:bonus_rate" json:"bonusRate"`
	Color          string         `gorm:"default:'amber';column:color" json:"color"`
	SortOrder      int            `gorm:"default:0;column:sort_order" json:"sortOrder"`
	Active         bool           `gorm:"default:true;column:active" json:"active"`
	Metadata       JSONMap        `gorm:"type:jsonb;column:metadata;default:'{}'" json:"metadata"`
	CreatedAt      time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (AffiliateTierRule) TableName() string { return "AffiliateTierRule" }
func (a *AffiliateTierRule) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

// AffiliateCampaign is a marketing campaign that affiliates can join.
type AffiliateCampaign struct {
	ID             string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name           string         `gorm:"not null;column:name" json:"name"`
	Slug           string         `gorm:"not null;uniqueIndex;column:slug" json:"slug"`
	Description    *string        `gorm:"type:text;column:description" json:"description,omitempty"`
	Status         string         `gorm:"default:'DRAFT';index;column:status" json:"status"`
	StartDate      *time.Time     `gorm:"column:start_date" json:"startDate,omitempty"`
	EndDate        *time.Time     `gorm:"column:end_date" json:"endDate,omitempty"`
	CommissionRate *float64       `gorm:"column:commission_rate" json:"commissionRate,omitempty"`
	Budget         *float64       `gorm:"column:budget" json:"budget,omitempty"`
	Spent          float64        `gorm:"default:0;column:spent" json:"spent"`
	BannerURL      *string        `gorm:"column:banner_url" json:"bannerUrl,omitempty"`
	LandingURL     *string        `gorm:"column:landing_url" json:"landingUrl,omitempty"`
	PromoCode      *string        `gorm:"column:promo_code" json:"promoCode,omitempty"`
	Metadata       JSONMap        `gorm:"type:jsonb;column:metadata;default:'{}'" json:"metadata"`
	CreatedBy      *string        `gorm:"type:uuid;column:created_by" json:"createdBy,omitempty"`
	CreatedAt      time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (AffiliateCampaign) TableName() string { return "AffiliateCampaign" }
func (a *AffiliateCampaign) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.Status == "" {
		a.Status = "DRAFT"
	}
	return nil
}

// AffiliateLink is a trackable link belonging to an affiliate (optionally tied to a campaign).
type AffiliateLink struct {
	ID                string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	AffiliateID       string         `gorm:"not null;index;type:uuid;column:affiliate_id;constraint:OnDelete:CASCADE" json:"affiliateId"`
	CampaignID        *string        `gorm:"type:uuid;column:campaign_id" json:"campaignId,omitempty"`
	Name              string         `gorm:"not null;column:name" json:"name"`
	Slug              string         `gorm:"not null;uniqueIndex;column:slug" json:"slug"`
	DestinationURL    string         `gorm:"not null;column:destination_url" json:"destinationUrl"`
	UTMSource         *string        `gorm:"column:utm_source" json:"utmSource,omitempty"`
	UTMMedium         *string        `gorm:"column:utm_medium" json:"utmMedium,omitempty"`
	UTMCampaign       *string        `gorm:"column:utm_campaign" json:"utmCampaign,omitempty"`
	ClicksCount       int            `gorm:"default:0;column:clicks_count" json:"clicksCount"`
	UniqueClicksCount int            `gorm:"default:0;column:unique_clicks_count" json:"uniqueClicksCount"`
	ConversionsCount  int            `gorm:"default:0;column:conversions_count" json:"conversionsCount"`
	Active            bool           `gorm:"default:true;column:active" json:"active"`
	Metadata          JSONMap        `gorm:"type:jsonb;column:metadata;default:'{}'" json:"metadata"`
	CreatedAt         time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt         time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt         gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	Affiliate *Affiliate         `gorm:"foreignKey:AffiliateID;constraint:OnDelete:CASCADE" json:"affiliate,omitempty"`
	Campaign  *AffiliateCampaign `gorm:"foreignKey:CampaignID;constraint:OnDelete:SET NULL" json:"campaign,omitempty"`
}

func (AffiliateLink) TableName() string { return "AffiliateLink" }
func (l *AffiliateLink) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}

// AffiliateLinkClick records one click event for analytics.
type AffiliateLinkClick struct {
	ID          string    `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	LinkID      string    `gorm:"not null;index;type:uuid;column:link_id;constraint:OnDelete:CASCADE" json:"linkId"`
	AffiliateID string    `gorm:"not null;index;type:uuid;column:affiliate_id;constraint:OnDelete:CASCADE" json:"affiliateId"`
	IPHash      *string   `gorm:"column:ip_hash" json:"ipHash,omitempty"`
	UserAgent   *string   `gorm:"type:text;column:user_agent" json:"userAgent,omitempty"`
	Referer     *string   `gorm:"type:text;column:referer" json:"referer,omitempty"`
	Country     *string   `gorm:"column:country" json:"country,omitempty"`
	Device      *string   `gorm:"column:device" json:"device,omitempty"`
	Converted   bool      `gorm:"default:false;column:converted" json:"converted"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (AffiliateLinkClick) TableName() string { return "AffiliateLinkClick" }
func (c *AffiliateLinkClick) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}

// AffiliatePayout records a single payout (could be batch or per-affiliate).
type AffiliatePayout struct {
	ID           string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	AffiliateID  string         `gorm:"not null;index;type:uuid;column:affiliate_id;constraint:OnDelete:CASCADE" json:"affiliateId"`
	Amount       float64        `gorm:"not null;column:amount" json:"amount"`
	Currency     string         `gorm:"default:'EGP';column:currency" json:"currency"`
	Status       string         `gorm:"default:'PENDING';index;column:status" json:"status"`
	Method       *string        `gorm:"column:method" json:"method,omitempty"`
	Reference    *string        `gorm:"column:reference" json:"reference,omitempty"`
	Notes        *string        `gorm:"type:text;column:notes" json:"notes,omitempty"`
	ProcessedBy  *string        `gorm:"type:uuid;column:processed_by" json:"processedBy,omitempty"`
	ProcessedAt  *time.Time     `gorm:"column:processed_at" json:"processedAt,omitempty"`
	ReferralIDs  JSONArray      `gorm:"type:jsonb;column:referral_ids;default:'[]'" json:"referralIds"`
	Metadata     JSONMap        `gorm:"type:jsonb;column:metadata;default:'{}'" json:"metadata"`
	CreatedAt    time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt    time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	Affiliate *Affiliate `gorm:"foreignKey:AffiliateID;constraint:OnDelete:CASCADE" json:"affiliate,omitempty"`
}

func (AffiliatePayout) TableName() string { return "AffiliatePayout" }
func (p *AffiliatePayout) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	if p.Status == "" {
		p.Status = "PENDING"
	}
	if p.Currency == "" {
		p.Currency = "EGP"
	}
	return nil
}

// AffiliateSetting is a key/value singleton for global affiliate configuration.
type AffiliateSetting struct {
	ID                     string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Key                    string         `gorm:"not null;uniqueIndex;column:key" json:"key"`
	DefaultCommissionRate  float64        `gorm:"default:10;column:default_commission_rate" json:"defaultCommissionRate"`
	DefaultTier            string         `gorm:"default:'BRONZE';column:default_tier" json:"defaultTier"`
	AutoApprove            bool           `gorm:"default:true;column:auto_approve" json:"autoApprove"`
	MinimumPayout          float64        `gorm:"default:0;column:minimum_payout" json:"minimumPayout"`
	HoldDays               int            `gorm:"default:0;column:hold_days" json:"holdDays"`
	CookieDays             int            `gorm:"default:30;column:cookie_days" json:"cookieDays"`
	AllowSelfReferral      bool           `gorm:"default:false;column:allow_self_referral" json:"allowSelfReferral"`
	EmailTemplateWelcome   *string        `gorm:"type:text;column:email_template_welcome" json:"emailTemplateWelcome,omitempty"`
	EmailTemplatePayout    *string        `gorm:"type:text;column:email_template_payout" json:"emailTemplatePayout,omitempty"`
	NotifyOnSignup         bool           `gorm:"default:true;column:notify_on_signup" json:"notifyOnSignup"`
	NotifyOnPayout         bool           `gorm:"default:true;column:notify_on_payout" json:"notifyOnPayout"`
	Metadata               JSONMap        `gorm:"type:jsonb;column:metadata;default:'{}'" json:"metadata"`
	UpdatedBy              *string        `gorm:"type:uuid;column:updated_by" json:"updatedBy,omitempty"`
	CreatedAt              time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt              time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt              gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (AffiliateSetting) TableName() string { return "AffiliateSetting" }
func (s *AffiliateSetting) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	if s.Key == "" {
		s.Key = "default"
	}
	return nil
}

// AffiliateAudit is a dedicated audit log for affiliate actions.
type AffiliateAudit struct {
	ID          string    `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	AffiliateID *string   `gorm:"type:uuid;column:affiliate_id" json:"affiliateId,omitempty"`
	ActorID     *string   `gorm:"type:uuid;column:actor_id" json:"actorId,omitempty"`
	Action      string    `gorm:"not null;index;column:action" json:"action"`
	Target      *string   `gorm:"column:target" json:"target,omitempty"`
	Details     JSONMap   `gorm:"type:jsonb;column:details;default:'{}'" json:"details"`
	IP          *string   `gorm:"column:ip" json:"ip,omitempty"`
	CreatedAt   time.Time `gorm:"index;column:created_at" json:"createdAt"`
}

func (AffiliateAudit) TableName() string { return "AffiliateAudit" }
func (a *AffiliateAudit) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}
