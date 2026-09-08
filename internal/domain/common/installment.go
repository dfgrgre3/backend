package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type InstallmentStatus string

const (
	InstallmentPending   InstallmentStatus = "pending"
	InstallmentPaid      InstallmentStatus = "paid"
	InstallmentOverdue   InstallmentStatus = "overdue"
	InstallmentCancelled InstallmentStatus = "cancelled"
)

// Installment represents one scheduled payment of a Payment that was split
// into multiple due dates (e.g. a yearly plan paid over several months).
type Installment struct {
	ID                string            `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID            string            `gorm:"not null;index;type:uuid;column:user_id" json:"userId" binding:"required,uuid"`
	PaymentID         *string           `gorm:"index;type:uuid;column:payment_id" json:"paymentId"`
	InstallmentNumber int               `gorm:"not null;column:installment_number" json:"installmentNumber"`
	TotalInstallments int               `gorm:"not null;column:total_installments" json:"totalInstallments"`
	Amount            decimal.Decimal   `gorm:"not null;type:numeric(19,4);column:amount" json:"amount" binding:"required,gt=0"`
	Currency          string            `gorm:"not null;default:'EGP';column:currency" json:"currency"`
	Status            InstallmentStatus `gorm:"not null;default:'pending';index;column:status" json:"status" binding:"omitempty,oneof=pending paid overdue cancelled"`
	DueDate           time.Time         `gorm:"not null;index;column:due_date" json:"dueDate" binding:"required"`
	PaidAt            *time.Time        `gorm:"column:paid_at" json:"paidAt"`
	Method            string            `gorm:"column:method" json:"method"`
	Reference         string            `gorm:"column:reference" json:"reference"`
	Notes             string            `gorm:"column:notes" json:"notes"`
	CreatedAt         time.Time         `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt         time.Time         `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt         gorm.DeletedAt    `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	User    User     `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	Payment *Payment `gorm:"foreignKey:PaymentID;constraint:OnDelete:SET NULL" json:"payment,omitempty"`
}

func (Installment) TableName() string {
	return "Installment"
}

func (i *Installment) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == "" {
		i.ID = uuid.New().String()
	}
	if i.Currency == "" {
		i.Currency = "EGP"
	}
	if i.Status == "" {
		i.Status = InstallmentPending
	}
	return
}
