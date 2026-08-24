package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LmsCategory represents category tree hierarchy
type LmsCategory struct {
	ID        uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name      string         `gorm:"not null;column:name" json:"name"`
	Slug      string         `gorm:"uniqueIndex;not null;column:slug" json:"slug"`
	ParentID  *uuid.UUID     `gorm:"index;type:uuid;column:parent_id" json:"parentId,omitempty"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (LmsCategory) TableName() string {
	return "LmsCategory"
}

func (c *LmsCategory) BeforeCreate(tx *gorm.DB) (err error) {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return
}

// LmsTag represents course tags
type LmsTag struct {
	ID        uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name      string         `gorm:"uniqueIndex;not null;column:name" json:"name"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"createdAt"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (LmsTag) TableName() string {
	return "LmsTag"
}

func (t *LmsTag) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return
}

// LmsCourseCategory is a join table for course-category many-to-many
type LmsCourseCategory struct {
	CourseID   uuid.UUID `gorm:"primaryKey;type:uuid;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	CategoryID uuid.UUID `gorm:"primaryKey;type:uuid;column:category_id;constraint:OnDelete:CASCADE" json:"categoryId"`
}

func (LmsCourseCategory) TableName() string {
	return "LmsCourseCategory"
}

// LmsCourseTag is a join table for course-tag many-to-many
type LmsCourseTag struct {
	CourseID uuid.UUID `gorm:"primaryKey;type:uuid;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	TagID    uuid.UUID `gorm:"primaryKey;type:uuid;column:tag_id;constraint:OnDelete:CASCADE" json:"tagId"`
}

func (LmsCourseTag) TableName() string {
	return "LmsCourseTag"
}
