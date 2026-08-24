package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CourseVersion stores immutable curriculum snapshots for versioning.
type CourseVersion struct {
	ID                 string    `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SubjectID          string    `gorm:"index;type:uuid;column:subject_id" json:"subjectId"`
	Version            string    `gorm:"not null;column:version" json:"version"`
	VersionNumber      int       `gorm:"default:1;column:version_number" json:"versionNumber"`
	ChangeSummary      *string   `gorm:"column:change_summary" json:"changeSummary,omitempty"`
	ChangeSummaryAr    *string   `gorm:"column:change_summary_ar" json:"changeSummaryAr,omitempty"`
	CurriculumSnapshot []byte    `gorm:"type:jsonb;column:curriculum_snapshot" json:"-"`
	CreatedBy          *string   `gorm:"type:uuid;column:created_by" json:"createdBy,omitempty"`
	CreatedAt          time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (CourseVersion) TableName() string { return "course_versions" }

func (cv *CourseVersion) BeforeCreate(tx *gorm.DB) error {
	if cv.ID == "" {
		cv.ID = uuid.New().String()
	}
	return nil
}
