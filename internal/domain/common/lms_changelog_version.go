package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LmsCourseChangelog tracks detailed modifications
type LmsCourseChangelog struct {
	ID        uuid.UUID `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	CourseID  uuid.UUID `gorm:"not null;type:uuid;index;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	UserID    uuid.UUID `gorm:"not null;type:uuid;index;column:user_id;constraint:OnDelete:CASCADE" json:"userId"`
	Field     string    `gorm:"not null;column:field" json:"field"`
	OldValue  *string   `gorm:"column:old_value" json:"oldValue,omitempty"`
	NewValue  *string   `gorm:"column:new_value" json:"newValue,omitempty"`
	CreatedAt time.Time `gorm:"index;column:created_at" json:"createdAt"`
}

func (LmsCourseChangelog) TableName() string {
	return "LmsCourseChangelog"
}

func (cl *LmsCourseChangelog) BeforeCreate(tx *gorm.DB) (err error) {
	if cl.ID == uuid.Nil {
		cl.ID = uuid.New()
	}
	return
}

// LmsCourseVersion stores version snapshot
type LmsCourseVersion struct {
	ID            uuid.UUID       `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	CourseID      uuid.UUID       `gorm:"not null;type:uuid;index;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	VersionNumber int             `gorm:"not null;column:version_number" json:"versionNumber"`
	Snapshot      json.RawMessage `gorm:"type:jsonb;column:snapshot" json:"snapshot"`
	CreatedAt     time.Time       `gorm:"index;column:created_at" json:"createdAt"`
}

func (LmsCourseVersion) TableName() string {
	return "LmsCourseVersion"
}

func (v *LmsCourseVersion) BeforeCreate(tx *gorm.DB) (err error) {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	return
}
