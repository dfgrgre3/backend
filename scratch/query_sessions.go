//go:build ignore

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type UserSession struct {
	ID               string    `gorm:"column:id"`
	UserID           string    `gorm:"column:user_id"`
	RefreshTokenHash string    `gorm:"column:refresh_token_hash"`
	IsActive         bool      `gorm:"column:is_active"`
	Status           string    `gorm:"column:status"`
	ExpiresAt        time.Time `gorm:"column:expires_at"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (UserSession) TableName() string {
	return "UserSession"
}

func main() {
	dsn := "postgresql://postgres:Khaled@2008@127.0.0.1:5433/thanawy?sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	var sessions []UserSession
	if err := db.Order("created_at DESC").Limit(5).Find(&sessions).Error; err != nil {
		log.Fatalf("failed to query sessions: %v", err)
	}

	fmt.Println("Recent Sessions:")
	for _, s := range sessions {
		fmt.Printf("ID: %s, UserID: %s, Active: %t, Status: %s, ExpiresAt: %s, Hash: %s\n",
			s.ID, s.UserID, s.IsActive, s.Status, s.ExpiresAt.Format(time.RFC3339), s.RefreshTokenHash)
	}

	// Hash of the user's refresh token
	token := "fa636736c4d45e3d03240eb3d031e54de7d1ebafc2cfcb2056aca6ce91082b6c"
	h := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(h[:])
	fmt.Printf("\nSearching for hash of '%s': %s\n", token, hash)

	var matched []UserSession
	if err := db.Where("refresh_token_hash = ?", hash).Find(&matched).Error; err != nil {
		log.Fatalf("failed to search: %v", err)
	}
	for _, s := range matched {
		fmt.Printf("MATCHED - ID: %s, UserID: %s, Active: %t, Status: %s, ExpiresAt: %s\n",
			s.ID, s.UserID, s.IsActive, s.Status, s.ExpiresAt.Format(time.RFC3339))
	}
}
