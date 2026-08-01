package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PricingType defines how a course is priced
type PricingType string

const (
	PricingFree         PricingType = "FREE"
	PricingOneTime      PricingType = "ONE_TIME"
	PricingSubscription PricingType = "SUBSCRIPTION"
	PricingBundle       PricingType = "BUNDLE"
)

// Currency supported for course payments
type Currency string

const (
	CurrencyEGP Currency = "EGP"
	CurrencyUSD Currency = "USD"
	CurrencyEUR Currency = "EUR"
	CurrencySAR Currency = "SAR"
	CurrencyAED Currency = "AED"
	CurrencyGBP Currency = "GBP"
)

// CoursePricing stores the complete pricing configuration for a course
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

func (CoursePricing) TableName() string {
	return "course_pricing"
}

func (cp *CoursePricing) BeforeCreate(tx *gorm.DB) error {
	if cp.ID == "" {
		cp.ID = uuid.New().String()
	}
	return nil
}

// IsDiscountActive returns true if a discount is currently active
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

// EffectivePrice returns the discounted price if active, otherwise the regular price
func (cp *CoursePricing) EffectivePrice() float64 {
	if cp.IsDiscountActive() {
		return *cp.DiscountPrice
	}
	return cp.Price
}

// Bundle represents a group of courses sold together
type CourseBundle struct {
	ID                 string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name               string         `gorm:"not null;column:name" json:"name"`
	NameAr             *string        `gorm:"column:name_ar" json:"nameAr,omitempty"`
	Description        *string        `gorm:"column:description" json:"description,omitempty"`
	DescriptionAr      *string        `gorm:"column:description_ar" json:"descriptionAr,omitempty"`
	Price              float64        `gorm:"default:0;column:price" json:"price"`
	Currency           Currency       `gorm:"default:'EGP';column:currency" json:"currency"`
	DiscountPrice      *float64       `gorm:"column:discount_price" json:"discountPrice,omitempty"`
	DiscountPercentage *float64       `gorm:"column:discount_percentage" json:"discountPercentage,omitempty"`
	DiscountStartAt    *time.Time     `gorm:"column:discount_start_at" json:"discountStartAt,omitempty"`
	DiscountEndAt      *time.Time     `gorm:"column:discount_end_at" json:"discountEndAt,omitempty"`
	CourseIDs          PGStringArray  `gorm:"type:text[];column:course_ids" json:"courseIds"`
	ThumbnailUrl       *string        `gorm:"column:thumbnail_url" json:"thumbnailUrl,omitempty"`
	IsActive           bool           `gorm:"default:true;column:is_active" json:"isActive"`
	IsFeatured         bool           `gorm:"default:false;column:is_featured" json:"isFeatured"`
	FeaturedUntil      *time.Time     `gorm:"column:featured_until" json:"featuredUntil,omitempty"`
	TotalCourses       int            `gorm:"default:0;column:total_courses" json:"totalCourses"`
	TotalStudents      int            `gorm:"default:0;column:total_students" json:"totalStudents"`
	TotalRevenue       float64        `gorm:"default:0;column:total_revenue" json:"totalRevenue"`
	CreatedAt          time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt          time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt          gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Non-DB relations
	Courses []Subject `gorm:"-" json:"courses,omitempty"`
}

func (CourseBundle) TableName() string {
	return "course_bundles"
}

func (cb *CourseBundle) BeforeCreate(tx *gorm.DB) error {
	if cb.ID == "" {
		cb.ID = uuid.New().String()
	}
	return nil
}

// BundleEnrollment tracks student purchases of bundles
type BundleEnrollment struct {
	ID         string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID     string         `gorm:"uniqueIndex:idx_bundle_enroll_user_bundle;type:uuid;column:user_id" json:"userId"`
	BundleID   string         `gorm:"uniqueIndex:idx_bundle_enroll_user_bundle;type:uuid;column:bundle_id" json:"bundleId"`
	PaymentID  *string        `gorm:"type:uuid;column:payment_id" json:"paymentId,omitempty"`
	PricePaid  float64        `gorm:"default:0;column:price_paid" json:"pricePaid"`
	Currency   Currency       `gorm:"default:'EGP';column:currency" json:"currency"`
	Status     string         `gorm:"default:'ACTIVE';column:status" json:"status"` // ACTIVE, EXPIRED, CANCELLED, REFUNDED
	EnrolledAt time.Time      `gorm:"column:enrolled_at" json:"enrolledAt"`
	ExpiresAt  *time.Time     `gorm:"column:expires_at" json:"expiresAt,omitempty"`
	CreatedAt  time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt  time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt  gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	User   *User         `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
	Bundle *CourseBundle `gorm:"foreignKey:BundleID;constraint:OnDelete:CASCADE" json:"bundle,omitempty"`
}

func (BundleEnrollment) TableName() string {
	return "bundle_enrollments"
}

func (be *BundleEnrollment) BeforeCreate(tx *gorm.DB) error {
	if be.ID == "" {
		be.ID = uuid.New().String()
	}
	return nil
}

// BundleCourse is the explicit junction table for bundle-courses
type BundleCourse struct {
	BundleID  string    `gorm:"primaryKey;type:uuid;column:bundle_id" json:"bundleId"`
	CourseID  string    `gorm:"primaryKey;type:uuid;column:course_id" json:"courseId"`
	SortOrder int       `gorm:"default:0;column:sort_order" json:"sortOrder"`
	AddedAt   time.Time `gorm:"column:added_at" json:"addedAt"`

	// Relations
	Bundle *CourseBundle `gorm:"foreignKey:BundleID;constraint:OnDelete:CASCADE" json:"bundle,omitempty"`
	Course *Subject      `gorm:"foreignKey:CourseID;constraint:OnDelete:CASCADE" json:"course,omitempty"`
}

func (BundleCourse) TableName() string {
	return "bundle_courses"
}

// CourseVersion stores immutable curriculum snapshots for versioning
type CourseVersion struct {
	ID                 string    `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SubjectID          string    `gorm:"index;type:uuid;column:subject_id" json:"subjectId"`
	Version            string    `gorm:"not null;column:version" json:"version"`
	VersionNumber      int       `gorm:"default:1;column:version_number" json:"versionNumber"`
	ChangeSummary      *string   `gorm:"column:change_summary" json:"changeSummary,omitempty"`
	ChangeSummaryAr    *string   `gorm:"column:change_summary_ar" json:"changeSummaryAr,omitempty"`
	CurriculumSnapshot []byte    `gorm:"type:jsonb;column:curriculum_snapshot" json:"-"`
	CreatedBy          *string   `gorm:"type:uuid;column:created_by" json:"createdBy,omitempty"`
	CreatedAt          time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (CourseVersion) TableName() string {
	return "course_versions"
}

func (cv *CourseVersion) BeforeCreate(tx *gorm.DB) error {
	if cv.ID == "" {
		cv.ID = uuid.New().String()
	}
	return nil
}

// Request/Response DTOs for course pricing API

type UpdatePricingRequest struct {
	PricingType        PricingType `json:"pricingType"`
	Price              float64     `json:"price"`
	Currency           Currency    `json:"currency"`
	DiscountPrice      *float64    `json:"discountPrice,omitempty"`
	DiscountStartAt    *time.Time  `json:"discountStartAt,omitempty"`
	DiscountEndAt      *time.Time  `json:"discountEndAt,omitempty"`
	SubscriptionPlanID *string     `json:"subscriptionPlanId,omitempty"`
}

type CreateBundleRequest struct {
	Name          string   `json:"name" binding:"required"`
	NameAr        *string  `json:"nameAr,omitempty"`
	Description   *string  `json:"description,omitempty"`
	DescriptionAr *string  `json:"descriptionAr,omitempty"`
	Price         float64  `json:"price"`
	Currency      Currency `json:"currency"`
	CourseIDs     []string `json:"courseIds"`
	ThumbnailUrl  *string  `json:"thumbnailUrl,omitempty"`
	IsFeatured    bool     `json:"isFeatured"`
	FeaturedUntil *string  `json:"featuredUntil,omitempty"`
}

type UpdateBundleRequest struct {
	Name          *string   `json:"name,omitempty"`
	NameAr        *string   `json:"nameAr,omitempty"`
	Description   *string   `json:"description,omitempty"`
	DescriptionAr *string   `json:"descriptionAr,omitempty"`
	Price         *float64  `json:"price,omitempty"`
	Currency      *Currency `json:"currency,omitempty"`
	DiscountPrice *float64  `json:"discountPrice,omitempty"`
	ThumbnailUrl  *string   `json:"thumbnailUrl,omitempty"`
	IsActive      *bool     `json:"isActive,omitempty"`
	IsFeatured    *bool     `json:"isFeatured,omitempty"`
	FeaturedUntil *string   `json:"featuredUntil,omitempty"`
}

type AddBundleCoursesRequest struct {
	CourseIDs []string `json:"courseIds" binding:"required,min=1"`
}
