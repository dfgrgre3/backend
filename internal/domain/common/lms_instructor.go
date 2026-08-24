package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// LmsInstructor mapping with roles
type LmsInstructor struct {
	CourseID     uuid.UUID       `gorm:"primaryKey;type:uuid;index;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	InstructorID uuid.UUID       `gorm:"primaryKey;type:uuid;index;column:instructor_id;constraint:OnDelete:CASCADE" json:"instructorId"`
	Role         string          `gorm:"default:'INSTRUCTOR';column:role" json:"role"`
	Permissions  json.RawMessage `gorm:"type:jsonb;column:permissions" json:"permissions"`
	CreatedAt    time.Time       `gorm:"column:created_at" json:"createdAt"`
}

func (LmsInstructor) TableName() string {
	return "LmsInstructor"
}
