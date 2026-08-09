package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LmsCourseAvailabilityWindow struct {
	ID            uuid.UUID              `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	CourseID      uuid.UUID              `gorm:"not null;type:uuid;index;column:course_id" json:"courseId"`
	WindowType    AvailabilityWindowType `gorm:"default:'PUBLISH';column:window_type" json:"windowType"`
	StartsAt      time.Time              `gorm:"column:starts_at" json:"startsAt"`
	EndsAt        *time.Time             `gorm:"column:ends_at" json:"endsAt,omitempty"`
	IsRepeating   bool                   `gorm:"default:false;column:is_repeating" json:"isRepeating"`
	RepeatPattern *string                `gorm:"column:repeat_pattern" json:"repeatPattern,omitempty"`
	IsActive      bool                   `gorm:"default:true;column:is_active" json:"isActive"`
	CreatedBy     *uuid.UUID             `gorm:"type:uuid;column:created_by" json:"createdBy,omitempty"`
	CreatedAt     time.Time              `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     time.Time              `gorm:"column:updated_at" json:"updatedAt"`

	Course *LmsCourse `gorm:"foreignKey:CourseID;constraint:OnDelete:CASCADE" json:"course,omitempty"`
}

func (LmsCourseAvailabilityWindow) TableName() string {
	return "LmsCourseAvailabilityWindow"
}

func (c *LmsCourseAvailabilityWindow) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return
}
