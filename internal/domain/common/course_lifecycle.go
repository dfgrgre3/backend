package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EnrollmentType defines how students can enroll in a course
type EnrollmentType string

const (
	EnrollmentTypeOpen     EnrollmentType = "OPEN"
	EnrollmentTypeLimited  EnrollmentType = "LIMITED"
	EnrollmentTypeApproval EnrollmentType = "BY_APPROVAL"
)

// CourseTag is a tag that can be attached to many courses (many-to-many).
type CourseTag struct {
	ID        string    `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name      string    `gorm:"uniqueIndex;not null;column:name" json:"name"`
	Slug      string    `gorm:"uniqueIndex;not null;column:slug" json:"slug"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (CourseTag) TableName() string {
	return "CourseTag"
}

func (t *CourseTag) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return
}

// SubjectTag is the join table between Subject and CourseTag.
type SubjectTag struct {
	SubjectID string `gorm:"primaryKey;type:uuid;column:subject_id" json:"subjectId"`
	TagID     string `gorm:"primaryKey;type:uuid;column:tag_id" json:"tagId"`
}

func (SubjectTag) TableName() string {
	return "SubjectTag"
}

// CourseReviewComment holds reviewer comments during the workflow review process.
type CourseReviewComment struct {
	ID         string    `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SubjectID  string    `gorm:"index;type:uuid;column:subject_id" json:"subjectId"`
	ReviewerID string    `gorm:"index;type:uuid;column:reviewer_id" json:"reviewerId"`
	Comment    string    `gorm:"type:text;not null;column:comment" json:"comment"`
	Status     string    `gorm:"default:'pending';column:status" json:"status"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (CourseReviewComment) TableName() string {
	return "CourseReviewComment"
}

func (rc *CourseReviewComment) BeforeCreate(tx *gorm.DB) (err error) {
	if rc.ID == "" {
		rc.ID = uuid.New().String()
	}
	return
}

// RelatedCourse links two courses with a relation type (related or prerequisite).
type RelatedCourse struct {
	CourseID        string `gorm:"primaryKey;type:uuid;column:course_id" json:"courseId"`
	RelatedCourseID string `gorm:"primaryKey;type:uuid;column:related_course_id" json:"relatedCourseId"`
	RelationType    string `gorm:"default:'related';column:relation_type" json:"relationType"`
}

func (RelatedCourse) TableName() string {
	return "RelatedCourse"
}
