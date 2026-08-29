package models

import (
	"fmt"
	"strings"

	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type PaymentStatus string

const (
	PaymentPending   PaymentStatus = "pending"
	PaymentCompleted PaymentStatus = "completed"
	PaymentFailed    PaymentStatus = "failed"
	PaymentRefunded  PaymentStatus = "refunded"
	PaymentCancelled PaymentStatus = "cancelled"
)

type Payment struct {
	ID        string          `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID    string          `gorm:"not null;index:idx_payment_user_subject,priority:1;type:uuid;column:user_id" json:"userId" binding:"required,uuid"`
	SubjectID *string         `gorm:"index:idx_payment_user_subject,priority:2;type:uuid;column:subject_id" json:"subjectId" binding:"omitempty,uuid"`
	PlanID    string          `gorm:"index;type:uuid;column:plan_id" json:"planId" binding:"required,uuid"`
	Amount    decimal.Decimal `gorm:"not null;type:numeric(19,4);check:amount >= 0;column:amount" json:"amount" binding:"required,gt=0"`
	Currency  string          `gorm:"not null;default:'EGP';column:currency" json:"currency" binding:"required,len=3"`
	Status    PaymentStatus   `gorm:"not null;default:'pending';index;column:status" json:"status" binding:"required,oneof=pending completed failed refunded cancelled"`
	Method    string          `gorm:"not null;column:method" json:"method" binding:"required,min=2,max=50"` // PAYMOB, WALLET, etc.
	Reference string          `gorm:"uniqueIndex;not null;column:reference" json:"reference" binding:"required,min=5,max=100"`

	// Paymob specific fields
	PaymobOrderID int64     `gorm:"index;column:paymob_order_id" json:"paymobOrderId"`
	ExternalTxnID string    `gorm:"index;column:external_txn_id" json:"externalTxnId"`
	CompletedAt   time.Time `gorm:"column:completed_at" json:"completedAt"`

	// OrderID links this payment back to the cart Order it was part of
	// (nil for a direct single-course checkout via subject_checkout.go).
	// Lets the Paymob webhook update Order.Status once every item's payment
	// has been resolved — see payment_handler_webhook.go.
	OrderID *string `gorm:"index;type:uuid;column:order_id;constraint:OnDelete:SET NULL" json:"orderId,omitempty"`

	CreatedAt time.Time      `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	User    User     `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	Subject *Subject `gorm:"foreignKey:SubjectID;constraint:OnDelete:SET NULL" json:"subject,omitempty"`
}

type Invoice struct {
	ID            string         `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	PaymentID     string         `gorm:"uniqueIndex;not null;type:uuid;column:payment_id" json:"paymentId"`
	UserID        string         `gorm:"index;not null;type:uuid;column:user_id" json:"userId"`
	InvoiceNumber string         `gorm:"uniqueIndex;not null;column:invoice_number" json:"invoiceNumber"`
	PdfUrl        string         `gorm:"column:pdf_url" json:"pdfUrl"`
	CreatedAt     time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	Payment Payment `gorm:"foreignKey:PaymentID;constraint:OnDelete:CASCADE" json:"payment,omitempty"`
	User    User    `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

func (Payment) TableName() string {
	return "Payment"
}

func (p *Payment) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	// Ensure amount is non-negative
	if p.Amount.LessThan(decimal.Zero) {
		return fmt.Errorf("payment amount cannot be negative")
	}
	// Trim reference
	p.Reference = strings.TrimSpace(p.Reference)
	return
}

func (p *Payment) BeforeUpdate(tx *gorm.DB) (err error) {
	// Prevent changing reference after creation
	if tx.Statement.Changed("Reference") && p.Reference != "" {
		return fmt.Errorf("payment reference cannot be changed after creation")
	}
	// Ensure amount is non-negative
	if tx.Statement.Changed("Amount") && p.Amount.LessThan(decimal.Zero) {
		return fmt.Errorf("payment amount cannot be negative")
	}
	return
}

func (Invoice) TableName() string {
	return "Invoice"
}

func (i *Invoice) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == "" {
		i.ID = uuid.New().String()
	}
	return
}
