package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DirectMessage represents a one-to-one chat message between two users.
// Both parties are resolved server-side: SendChatMessage stamps SenderID
// from the JWT session and validates ReceiverID, so a client can never
// forge the sender (IDOR/BOLA hardening).
type DirectMessage struct {
	ID         string     `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SenderID   string     `gorm:"not null;type:uuid;index:idx_dm_sender_receiver_created,priority:1;column:sender_id" json:"senderId"`
	ReceiverID string     `gorm:"not null;type:uuid;index:idx_dm_receiver_sender_created,priority:1;column:receiver_id" json:"receiverId"`
	Content    string     `gorm:"not null;type:text;column:content" json:"content"`
	IsRead     bool       `gorm:"not null;default:false;column:is_read" json:"isRead"`
	ReadAt     *time.Time `gorm:"column:read_at" json:"readAt"`
	CreatedAt  time.Time  `gorm:"not null;index:idx_dm_sender_receiver_created,priority:3;index:idx_dm_receiver_sender_created,priority:3;column:created_at" json:"createdAt"`

	// Relations
	Sender   User `gorm:"foreignKey:SenderID;constraint:OnDelete:CASCADE" json:"-"`
	Receiver User `gorm:"foreignKey:ReceiverID;constraint:OnDelete:CASCADE" json:"-"`
}

func (DirectMessage) TableName() string {
	return "DirectMessage"
}

func (m *DirectMessage) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return
}
