package models

import "time"

// Request/Response DTOs for course pricing and bundle API.

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
