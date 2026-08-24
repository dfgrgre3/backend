package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// LmsBundle represents course bundles
type LmsBundle struct {
	ID           uuid.UUID       `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Title        string          `gorm:"not null;index;column:title" json:"title"`
	Slug         string          `gorm:"uniqueIndex;not null;column:slug" json:"slug"`
	Description  *string         `gorm:"type:text;column:description" json:"description,omitempty"`
	CoverURL     *string         `gorm:"column:cover_url" json:"coverUrl,omitempty"`
	Price        decimal.Decimal `gorm:"default:0;type:numeric(19,4);column:price" json:"price"`
	CurrencyCode string          `gorm:"default:'USD';column:currency_code" json:"currencyCode"`
	IsActive     bool            `gorm:"default:true;index;column:is_active" json:"isActive"`
	CreatedAt    time.Time       `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt    time.Time       `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt    gorm.DeletedAt  `gorm:"index;column:deleted_at" json:"-"`

	// Associations
	Courses []LmsCourse `gorm:"many2many:LmsBundleCourse;" json:"courses,omitempty"`
}

func (LmsBundle) TableName() string {
	return "LmsBundle"
}

func (b *LmsBundle) BeforeCreate(tx *gorm.DB) (err error) {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return
}
