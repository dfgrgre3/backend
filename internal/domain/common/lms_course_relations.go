package models

import (
	"github.com/google/uuid"
)

// LmsCoursePrerequisite is a join table for course prerequisites
type LmsCoursePrerequisite struct {
	CourseID             uuid.UUID `gorm:"primaryKey;type:uuid;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	PrerequisiteCourseID uuid.UUID `gorm:"primaryKey;type:uuid;column:prerequisite_course_id;constraint:OnDelete:CASCADE" json:"prerequisiteCourseId"`
}

func (LmsCoursePrerequisite) TableName() string {
	return "LmsCoursePrerequisite"
}

// LmsRelatedCourse is a join table for related courses
type LmsRelatedCourse struct {
	CourseID        uuid.UUID `gorm:"primaryKey;type:uuid;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	RelatedCourseID uuid.UUID `gorm:"primaryKey;type:uuid;column:related_course_id;constraint:OnDelete:CASCADE" json:"relatedCourseId"`
}

func (LmsRelatedCourse) TableName() string {
	return "LmsRelatedCourse"
}
