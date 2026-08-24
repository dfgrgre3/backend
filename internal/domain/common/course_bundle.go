package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CourseBundle represents a group of courses sold together.
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

func (CourseBundle) TableName() string { return "course_bundles" }

func (cb *CourseBundle) BeforeCreate(tx *gorm.DB) error {
	if cb.ID == "" {
		cb.ID = uuid.New().String()
	}
	return nil
}

// BundleEnrollment tracks student purchases of bundles.
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

func (BundleEnrollment) TableName() string { return "bundle_enrollments" }

func (be *BundleEnrollment) BeforeCreate(tx *gorm.DB) error {
	if be.ID == "" {
		be.ID = uuid.New().String()
	}
	return nil
}

// BundleCourse is the explicit junction table for bundle-courses.
type BundleCourse struct {
	BundleID  string    `gorm:"primaryKey;type:uuid;column:bundle_id" json:"bundleId"`
	CourseID  string    `gorm:"primaryKey;type:uuid;column:course_id" json:"courseId"`
	SortOrder int       `gorm:"default:0;column:sort_order" json:"sortOrder"`
	AddedAt   time.Time `gorm:"column:added_at" json:"addedAt"`

	// Relations
	Bundle *CourseBundle `gorm:"foreignKey:BundleID;constraint:OnDelete:CASCADE" json:"bundle,omitempty"`
	Course *Subject      `gorm:"foreignKey:CourseID;constraint:OnDelete:CASCADE" json:"course,omitempty"`
}

func (BundleCourse) TableName() string { return "bundle_courses" }
