package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SearchHistory tracks user search queries for personalization
type SearchHistory struct {
	ID        string    `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID    string    `gorm:"index;type:uuid;column:user_id;not null" json:"userId"`
	Query     string    `gorm:"type:varchar(200);column:query;not null" json:"query"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (SearchHistory) TableName() string {
	return "search_history"
}

func (sh *SearchHistory) BeforeCreate(tx *gorm.DB) (err error) {
	if sh.ID == "" {
		sh.ID = uuid.New().String()
	}
	return
}
