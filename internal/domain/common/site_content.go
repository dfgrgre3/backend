package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Banner is a promotional banner shown across the public site.
type Banner struct {
	ID           string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Title        string         `gorm:"not null;column:title" json:"title" binding:"required"`
	ImageURL     string         `gorm:"not null;column:image_url" json:"imageUrl" binding:"required"`
	LinkURL      string         `gorm:"column:link_url" json:"linkUrl"`
	Position     string         `gorm:"not null;default:'home_top';column:position" json:"position"`
	IsActive     bool           `gorm:"not null;default:true;column:is_active" json:"isActive"`
	DisplayOrder int            `gorm:"not null;default:0;column:display_order" json:"displayOrder"`
	StartDate    *time.Time     `gorm:"column:start_date" json:"startDate"`
	EndDate      *time.Time     `gorm:"column:end_date" json:"endDate"`
	CreatedAt    time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (Banner) TableName() string { return "Banner" }

func (b *Banner) BeforeCreate(tx *gorm.DB) (err error) {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	if b.Position == "" {
		b.Position = "home_top"
	}
	return
}

// FAQ is a frequently-asked question shown on the public site.
type FAQ struct {
	ID           string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Question     string         `gorm:"not null;column:question" json:"question" binding:"required"`
	Answer       string         `gorm:"not null;type:text;column:answer" json:"answer" binding:"required"`
	Category     string         `gorm:"not null;default:'general';column:category" json:"category"`
	DisplayOrder int            `gorm:"not null;default:0;column:display_order" json:"displayOrder"`
	IsActive     bool           `gorm:"not null;default:true;column:is_active" json:"isActive"`
	CreatedAt    time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (FAQ) TableName() string { return "FAQ" }

func (f *FAQ) BeforeCreate(tx *gorm.DB) (err error) {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	if f.Category == "" {
		f.Category = "general"
	}
	return
}

// HomepageSection is an ordered, toggleable block of the public homepage.
type HomepageSection struct {
	ID           string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Key          string         `gorm:"uniqueIndex;not null;column:key" json:"key" binding:"required"`
	Type         string         `gorm:"not null;column:type" json:"type" binding:"required"`
	Title        string         `gorm:"column:title" json:"title"`
	Content      string         `gorm:"type:text;column:content" json:"content"` // JSON-serialized section payload
	IsActive     bool           `gorm:"not null;default:true;column:is_active" json:"isActive"`
	DisplayOrder int            `gorm:"not null;default:0;column:display_order" json:"displayOrder"`
	CreatedAt    time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (HomepageSection) TableName() string { return "HomepageSection" }

func (h *HomepageSection) BeforeCreate(tx *gorm.DB) (err error) {
	if h.ID == "" {
		h.ID = uuid.New().String()
	}
	return
}

// CMSPage is a freeform static page (About, Terms, Privacy, ...).
type CMSPage struct {
	ID              string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Slug            string         `gorm:"uniqueIndex;not null;column:slug" json:"slug" binding:"required"`
	Title           string         `gorm:"not null;column:title" json:"title" binding:"required"`
	Content         string         `gorm:"type:text;column:content" json:"content"`
	Status          string         `gorm:"not null;default:'draft';column:status" json:"status" binding:"omitempty,oneof=draft published archived"`
	MetaTitle       string         `gorm:"column:meta_title" json:"metaTitle"`
	MetaDescription string         `gorm:"column:meta_description" json:"metaDescription"`
	CreatedAt       time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt       time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (CMSPage) TableName() string { return "CMSPage" }

func (p *CMSPage) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	if p.Status == "" {
		p.Status = "draft"
	}
	return
}
