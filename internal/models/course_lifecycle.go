package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

// CourseStatus represents the lifecycle state of a course.
type CourseStatus string

const (
	CourseStatusDraft         CourseStatus = "DRAFT"
	CourseStatusUnderReview  CourseStatus = "UNDER_REVIEW"
	CourseStatusPublished     CourseStatus = "PUBLISHED"
	CourseStatusArchived      CourseStatus = "ARCHIVED"
	CourseStatusRejected      CourseStatus = "REJECTED"
)

// ValidTransitions defines allowed course status changes
var ValidTransitions = map[CourseStatus][]CourseStatus{
	CourseStatusDraft:         {CourseStatusUnderReview, CourseStatusArchived},
	CourseStatusUnderReview:  {CourseStatusPublished, CourseStatusRejected, CourseStatusDraft},
	CourseStatusPublished:    {CourseStatusArchived, CourseStatusDraft},
	CourseStatusArchived:     {CourseStatusDraft},
	CourseStatusRejected:     {CourseStatusDraft},
}

// CanTransitionTo checks if a course can transition from current status to target
func (s CourseStatus) CanTransitionTo(target CourseStatus) bool {
	validTargets, exists := ValidTransitions[s]
	if !exists {
		return false
	}
	for _, t := range validTargets {
		if t == target {
			return true
		}
	}
	return false
}

// EnrollmentType defines how students can enroll in a course
type EnrollmentType string

const (
	EnrollmentTypeOpen       EnrollmentType = "OPEN"
	EnrollmentTypeLimited   EnrollmentType = "LIMITED"
	EnrollmentTypeApproval  EnrollmentType = "BY_APPROVAL"
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

// CourseChangelog records every field change on a course for audit purposes.
type CourseChangelog struct {
	ID        string    `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	SubjectID string    `gorm:"index;type:uuid;column:subject_id" json:"subjectId"`
	UserID    string    `gorm:"index;type:uuid;column:user_id" json:"userId"`
	FieldName string    `gorm:"not null;column:field_name" json:"fieldName"`
	OldValue  *string   `gorm:"column:old_value" json:"oldValue"`
	NewValue  *string   `gorm:"column:new_value" json:"newValue"`
	Action    string    `gorm:"default:'update';column:action" json:"action"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (CourseChangelog) TableName() string {
	return "CourseChangelog"
}

func (cl *CourseChangelog) BeforeCreate(tx *gorm.DB) (err error) {
	if cl.ID == "" {
		cl.ID = uuid.New().String()
	}
	return
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
	CourseID         string `gorm:"primaryKey;type:uuid;column:course_id" json:"courseId"`
	RelatedCourseID  string `gorm:"primaryKey;type:uuid;column:related_course_id" json:"relatedCourseId"`
	RelationType     string `gorm:"default:'related';column:relation_type" json:"relationType"`
}

func (RelatedCourse) TableName() string {
	return "RelatedCourse"
}
