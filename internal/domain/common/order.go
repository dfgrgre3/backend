package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Wishlist is a student's saved-for-later course, independent of the cart.
type Wishlist struct {
	ID        string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID    string         `gorm:"not null;index:idx_user_subject_wishlist,unique;type:uuid;column:user_id;constraint:OnDelete:CASCADE" json:"userId"`
	SubjectID string         `gorm:"not null;index:idx_user_subject_wishlist,unique;type:uuid;column:subject_id;constraint:OnDelete:CASCADE" json:"subjectId"`
	CreatedAt time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	Subject Subject `gorm:"foreignKey:SubjectID" json:"subject,omitempty"`
}

func (Wishlist) TableName() string {
	return "Wishlist"
}

func (w *Wishlist) BeforeCreate(tx *gorm.DB) (err error) {
	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	return
}

// CartItem is a course a student has added to their cart, ahead of
// multi-course checkout (Order/OrderItem below).
type CartItem struct {
	ID        string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID    string         `gorm:"not null;index:idx_user_subject_cart,unique;type:uuid;column:user_id;constraint:OnDelete:CASCADE" json:"userId"`
	SubjectID string         `gorm:"not null;index:idx_user_subject_cart,unique;type:uuid;column:subject_id;constraint:OnDelete:CASCADE" json:"subjectId"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"createdAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	Subject Subject `gorm:"foreignKey:SubjectID" json:"subject,omitempty"`
}

func (CartItem) TableName() string {
	return "CartItem"
}

func (ci *CartItem) BeforeCreate(tx *gorm.DB) (err error) {
	if ci.ID == "" {
		ci.ID = uuid.New().String()
	}
	return
}

type OrderStatus string

const (
	OrderPending    OrderStatus = "PENDING"
	OrderProcessing OrderStatus = "PROCESSING"
	OrderCompleted  OrderStatus = "COMPLETED"
	OrderCancelled  OrderStatus = "CANCELLED"
	OrderRefunded   OrderStatus = "REFUNDED"
)

// Order is a completed-or-in-progress cart checkout, matching the shape the
// admin panel's Orders page (d:/admin .../admin/orders/page.tsx) already
// expects — that UI existed before any backend route served it.
type Order struct {
	ID             string          `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	OrderNumber    string          `gorm:"uniqueIndex;not null;column:order_number" json:"orderNumber"`
	UserID         string          `gorm:"not null;index;type:uuid;column:user_id" json:"userId"`
	Status         OrderStatus     `gorm:"not null;default:'PENDING';index;column:status" json:"status"`
	Total          decimal.Decimal `gorm:"not null;type:numeric(19,4);column:total" json:"total"`
	Currency       string          `gorm:"not null;default:'EGP';column:currency" json:"currency"`
	PaymentMethod  *string         `gorm:"column:payment_method" json:"paymentMethod,omitempty"`
	TransactionID  *string         `gorm:"column:transaction_id" json:"transactionId,omitempty"`
	CouponCode     *string         `gorm:"column:coupon_code" json:"couponCode,omitempty"`
	DiscountAmount decimal.Decimal `gorm:"default:0;type:numeric(19,4);column:discount_amount" json:"discountAmount"`
	CreatedAt      time.Time       `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt      time.Time       `gorm:"column:updated_at" json:"updatedAt"`

	// Relations
	User  User        `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Items []OrderItem `gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE" json:"items,omitempty"`
}

func (Order) TableName() string {
	return "Order"
}

func (o *Order) BeforeCreate(tx *gorm.DB) (err error) {
	if o.ID == "" {
		o.ID = uuid.New().String()
	}
	return
}

// OrderItem is a line item snapshot (title/price at purchase time) so later
// price changes on the Subject don't retroactively change historical orders.
type OrderItem struct {
	ID        string          `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	OrderID   string          `gorm:"not null;index;type:uuid;column:order_id;constraint:OnDelete:CASCADE" json:"orderId"`
	SubjectID string          `gorm:"not null;type:uuid;column:subject_id" json:"subjectId"`
	Title     string          `gorm:"not null;column:title" json:"title"`
	Type      string          `gorm:"not null;default:'COURSE';column:type" json:"type"`
	Price     decimal.Decimal `gorm:"not null;type:numeric(19,4);column:price" json:"price"`
}

func (OrderItem) TableName() string {
	return "OrderItem"
}

func (oi *OrderItem) BeforeCreate(tx *gorm.DB) (err error) {
	if oi.ID == "" {
		oi.ID = uuid.New().String()
	}
	return
}
