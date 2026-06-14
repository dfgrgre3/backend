package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type TransactionType string

const (
	TxTypeDeposit   TransactionType = "DEPOSIT"
	TxTypeWithdraw  TransactionType = "WITHDRAW"
	TxTypeRefund    TransactionType = "REFUND"
	TxTypeAiUsage   TransactionType = "AI_USAGE"
	TxTypeExamUsage TransactionType = "EXAM_USAGE"
)

type WalletTransaction struct {
	ID          string          `gorm:"primaryKey;type:uuid" json:"id"`
	UserID      string          `gorm:"not null;index;type:uuid;constraint:OnDelete:CASCADE" json:"userId"`
	Type        TransactionType `gorm:"not null;index" json:"type"`
	Amount      float64         `gorm:"not null" json:"amount"`
	Currency    string          `gorm:"-" json:"currency"` // Virtual field (default 'EGP'), not in DB
	WalletType  string          `gorm:"column:walletId;not null;default:'BALANCE'" json:"walletType"`
	Description string          `json:"description"`
	ReferenceID *string         `gorm:"column:paymentId;index" json:"referenceId"`
	Status      string          `gorm:"column:status;default:'COMPLETED'" json:"status"`
	CreatedAt   time.Time       `gorm:"index;column:created_at" json:"createdAt"`
	UpdatedAt   time.Time       `gorm:"column:updatedAt" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt  `gorm:"index;column:deleted_at" json:"-"`

	// Relations
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

func (WalletTransaction) TableName() string {
	return "WalletTransaction"
}

func (w *WalletTransaction) BeforeCreate(tx *gorm.DB) (err error) {
	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	if w.Currency == "" {
		w.Currency = "EGP"
	}
	return
}
