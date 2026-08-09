package keys

import (
	"fmt"
	"time"
)

// Predefined cache key prefixes
const (
	PrefixSubject    = "subj"
	PrefixUser       = "user"
	PrefixCategory   = "cat"
	PrefixExam       = "exam"
	PrefixList       = "list"
	PrefixEnrollment = "enroll"
	PrefixProgress   = "prog"
	PrefixPayment    = "pay"
	PrefixTeacher    = "teacher"
)

// Cache TTL constants
const (
	TTLSubjectList       = 5 * time.Minute
	TTLSubjectDetail     = 15 * time.Minute
	TTLSubjectCurriculum = 30 * time.Minute
	TTLUserProfile       = 10 * time.Minute
	TTLUserEnrollments   = 5 * time.Minute
	TTLUserProgress      = 2 * time.Minute
	TTLCategoryList      = 1 * time.Hour
	TTLCategoryDetail    = 30 * time.Minute
	TTLExamList          = 5 * time.Minute
	TTLExamDetail        = 15 * time.Minute
	TTLEnrollmentStatus  = 2 * time.Minute
	TTLProgress          = 1 * time.Minute
	TTLPaymentStatus     = 1 * time.Minute
	TTLTeacher           = 1 * time.Hour
)

// Builder helps build consistent cache keys
type Builder struct {
	prefix string
	parts  []string
}

func NewBuilder(prefix string) *Builder {
	return &Builder{prefix: prefix}
}

func (b *Builder) Add(part string) *Builder {
	b.parts = append(b.parts, part)
	return b
}

func (b *Builder) AddInt(part int) *Builder {
	b.parts = append(b.parts, fmt.Sprintf("%d", part))
	return b
}

func (b *Builder) AddBool(part bool) *Builder {
	if part {
		b.parts = append(b.parts, "true")
	} else {
		b.parts = append(b.parts, "false")
	}
	return b
}

func (b *Builder) Build() string {
	key := b.prefix
	for _, part := range b.parts {
		key += ":" + part
	}
	return key
}

// Subject cache keys
func SubjectList(page, limit int, filters map[string]string) string {
	b := NewBuilder(PrefixSubject + ":" + PrefixList)
	b.AddInt(page).AddInt(limit)
	for k, v := range filters {
		b.Add(k + "=" + v)
	}
	return b.Build()
}

func SubjectDetail(idOrSlug string) string {
	return NewBuilder(PrefixSubject).Add(idOrSlug).Build()
}

func SubjectCurriculum(subjectID string) string {
	return NewBuilder(PrefixSubject).Add("curriculum").Add(subjectID).Build()
}

// User cache keys
func UserProfile(userID string) string {
	return NewBuilder(PrefixUser).Add("profile").Add(userID).Build()
}

func UserEnrollments(userID string) string {
	return NewBuilder(PrefixUser).Add(PrefixEnrollment).Add(userID).Build()
}

func UserProgress(userID, subjectID string) string {
	return NewBuilder(PrefixUser).Add(PrefixProgress).Add(userID).Add(subjectID).Build()
}

// Category cache keys
func CategoryList() string {
	return NewBuilder(PrefixCategory + ":" + PrefixList).Build()
}

func CategoryDetail(categoryID string) string {
	return NewBuilder(PrefixCategory).Add(categoryID).Build()
}

// Exam cache keys
func ExamList(subjectID string, page, limit int) string {
	return NewBuilder(PrefixExam + ":" + PrefixList).
		Add(subjectID).AddInt(page).AddInt(limit).Build()
}

func ExamDetail(examID string) string {
	return NewBuilder(PrefixExam).Add(examID).Build()
}

// Enrollment cache keys
func EnrollmentStatus(userID, subjectID string) string {
	return NewBuilder(PrefixEnrollment).Add("status").Add(userID).Add(subjectID).Build()
}

func UserEnrollmentsList(userID string) string {
	return NewBuilder(PrefixEnrollment + ":" + PrefixList).Add(userID).Build()
}

// Progress cache keys
func LessonProgress(userID, lessonID string) string {
	return NewBuilder(PrefixProgress).Add("lesson").Add(userID).Add(lessonID).Build()
}

func SubjectProgress(userID, subjectID string) string {
	return NewBuilder(PrefixProgress).Add("subject").Add(userID).Add(subjectID).Build()
}

// Payment cache keys
func PaymentStatus(userID, subjectID string) string {
	return NewBuilder(PrefixPayment).Add("status").Add(userID).Add(subjectID).Build()
}

// Teacher cache keys
func TeacherList(page, limit int) string {
	return NewBuilder(PrefixTeacher + ":" + PrefixList).
		AddInt(page).AddInt(limit).Build()
}

func TeacherDetail(teacherID string) string {
	return NewBuilder(PrefixTeacher).Add(teacherID).Build()
}
