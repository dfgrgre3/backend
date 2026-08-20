package cache

import (
	"fmt"
	"time"
)

// Predefined cache key prefixes
const (
	CachePrefixSubject    = "subj"
	CachePrefixUser       = "user"
	CachePrefixCategory   = "cat"
	CachePrefixExam       = "exam"
	CachePrefixList       = "list"
	CachePrefixEnrollment = "enroll"
	CachePrefixProgress   = "prog"
	CachePrefixPayment    = "pay"
	CachePrefixTeacher    = "teacher"
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

// CacheKeyBuilder helps build consistent cache keys
type CacheKeyBuilder struct {
	prefix string
	parts  []string
}

func NewCacheKeyBuilder(prefix string) *CacheKeyBuilder {
	return &CacheKeyBuilder{prefix: prefix}
}

func (b *CacheKeyBuilder) Add(part string) *CacheKeyBuilder {
	b.parts = append(b.parts, part)
	return b
}

func (b *CacheKeyBuilder) AddInt(part int) *CacheKeyBuilder {
	b.parts = append(b.parts, fmt.Sprintf("%d", part))
	return b
}

func (b *CacheKeyBuilder) AddBool(part bool) *CacheKeyBuilder {
	if part {
		b.parts = append(b.parts, "true")
	} else {
		b.parts = append(b.parts, "false")
	}
	return b
}

func (b *CacheKeyBuilder) Build() string {
	key := b.prefix
	for _, part := range b.parts {
		key += ":" + part
	}
	return key
}

// Subject cache keys
func SubjectListKey(page, limit int, filters map[string]string) string {
	b := NewCacheKeyBuilder(CachePrefixSubject + ":" + CachePrefixList)
	b.AddInt(page).AddInt(limit)
	for k, v := range filters {
		b.Add(k + "=" + v)
	}
	return b.Build()
}

func SubjectDetailKey(idOrSlug string) string {
	return NewCacheKeyBuilder(CachePrefixSubject).Add(idOrSlug).Build()
}

func SubjectCurriculumKey(subjectID string) string {
	return NewCacheKeyBuilder(CachePrefixSubject).Add("curriculum").Add(subjectID).Build()
}

// User cache keys
func UserProfileKey(userID string) string {
	return NewCacheKeyBuilder(CachePrefixUser).Add("profile").Add(userID).Build()
}

func UserEnrollmentsKey(userID string) string {
	return NewCacheKeyBuilder(CachePrefixUser).Add(CachePrefixEnrollment).Add(userID).Build()
}

func UserProgressKey(userID, subjectID string) string {
	return NewCacheKeyBuilder(CachePrefixUser).Add(CachePrefixProgress).Add(userID).Add(subjectID).Build()
}

// Category cache keys
func CategoryListKey() string {
	return NewCacheKeyBuilder(CachePrefixCategory + ":" + CachePrefixList).Build()
}

func CategoryDetailKey(categoryID string) string {
	return NewCacheKeyBuilder(CachePrefixCategory).Add(categoryID).Build()
}

// Exam cache keys
func ExamListKey(subjectID string, page, limit int) string {
	return NewCacheKeyBuilder(CachePrefixExam + ":" + CachePrefixList).
		Add(subjectID).AddInt(page).AddInt(limit).Build()
}

func ExamDetailKey(examID string) string {
	return NewCacheKeyBuilder(CachePrefixExam).Add(examID).Build()
}

// Enrollment cache keys
func EnrollmentStatusKey(userID, subjectID string) string {
	return NewCacheKeyBuilder(CachePrefixEnrollment).Add("status").Add(userID).Add(subjectID).Build()
}

func UserEnrollmentsListKey(userID string) string {
	return NewCacheKeyBuilder(CachePrefixEnrollment + ":" + CachePrefixList).Add(userID).Build()
}

// Progress cache keys
func LessonProgressKey(userID, lessonID string) string {
	return NewCacheKeyBuilder(CachePrefixProgress).Add("lesson").Add(userID).Add(lessonID).Build()
}

func SubjectProgressKey(userID, subjectID string) string {
	return NewCacheKeyBuilder(CachePrefixProgress).Add("subject").Add(userID).Add(subjectID).Build()
}

// Payment cache keys
func PaymentStatusKey(userID, subjectID string) string {
	return NewCacheKeyBuilder(CachePrefixPayment).Add("status").Add(userID).Add(subjectID).Build()
}

// Teacher cache keys
func TeacherListKey(page, limit int) string {
	return NewCacheKeyBuilder(CachePrefixTeacher + ":" + CachePrefixList).
		AddInt(page).AddInt(limit).Build()
}

func TeacherDetailKey(teacherID string) string {
	return NewCacheKeyBuilder(CachePrefixTeacher).Add(teacherID).Build()
}
