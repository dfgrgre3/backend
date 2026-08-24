package models

import (
	"time"
)

type CacheEntry struct {
	Key          string     `gorm:"primaryKey;column:key" json:"key"`
	Type         string     `gorm:"not null;column:type" json:"type"`
	Value        []byte     `gorm:"type:bytea;column:value" json:"-"`
	Size         int64      `gorm:"not null;column:size" json:"size"`
	Hits         int64      `gorm:"not null;default:0;column:hits" json:"hits"`
	Misses       int64      `gorm:"not null;default:0;column:misses" json:"misses"`
	LastAccessed time.Time  `gorm:"not null;index;column:last_accessed" json:"lastAccessed"`
	ExpiresAt    *time.Time `gorm:"index;column:expires_at" json:"expiresAt"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

func (CacheEntry) TableName() string {
	return "CacheEntry"
}
