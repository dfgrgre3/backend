package cache

import (
	"thanawy-backend/internal/infrastructure/cache/keys"
)

// Compatibility facade over the cache/keys package.
// New code should import cache/keys directly.

const (
	CachePrefixSubject    = keys.PrefixSubject
	CachePrefixUser       = keys.PrefixUser
	CachePrefixCategory   = keys.PrefixCategory
	CachePrefixExam       = keys.PrefixExam
	CachePrefixList       = keys.PrefixList
	CachePrefixEnrollment = keys.PrefixEnrollment
	CachePrefixProgress   = keys.PrefixProgress
	CachePrefixPayment    = keys.PrefixPayment
	CachePrefixTeacher    = keys.PrefixTeacher
)

const (
	TTLSubjectList       = keys.TTLSubjectList
	TTLSubjectDetail     = keys.TTLSubjectDetail
	TTLSubjectCurriculum = keys.TTLSubjectCurriculum
	TTLUserProfile       = keys.TTLUserProfile
	TTLUserEnrollments   = keys.TTLUserEnrollments
	TTLUserProgress      = keys.TTLUserProgress
	TTLCategoryList      = keys.TTLCategoryList
	TTLCategoryDetail    = keys.TTLCategoryDetail
	TTLExamList          = keys.TTLExamList
	TTLExamDetail        = keys.TTLExamDetail
	TTLEnrollmentStatus  = keys.TTLEnrollmentStatus
	TTLProgress          = keys.TTLProgress
	TTLPaymentStatus     = keys.TTLPaymentStatus
	TTLTeacher           = keys.TTLTeacher
	// Aliases for backward compatibility
	CacheTTLSubject  = keys.TTLSubjectDetail
	CacheTTLList     = keys.TTLSubjectList
	CacheTTLCategory = keys.TTLCategoryDetail
)

type CacheKeyBuilder = keys.Builder

func NewCacheKeyBuilder(prefix string) *CacheKeyBuilder { return keys.NewBuilder(prefix) }

func SubjectListKey(page, limit int, filters map[string]string) string {
	return keys.SubjectList(page, limit, filters)
}

func SubjectDetailKey(idOrSlug string) string { return keys.SubjectDetail(idOrSlug) }

func SubjectCurriculumKey(subjectID string) string { return keys.SubjectCurriculum(subjectID) }

func UserProfileKey(userID string) string { return keys.UserProfile(userID) }

func UserEnrollmentsKey(userID string) string { return keys.UserEnrollments(userID) }

func UserProgressKey(userID, subjectID string) string {
	return keys.UserProgress(userID, subjectID)
}

func CategoryListKey() string { return keys.CategoryList() }

func CategoryDetailKey(categoryID string) string { return keys.CategoryDetail(categoryID) }

func ExamListKey(subjectID string, page, limit int) string {
	return keys.ExamList(subjectID, page, limit)
}

func ExamDetailKey(examID string) string { return keys.ExamDetail(examID) }

func EnrollmentStatusKey(userID, subjectID string) string {
	return keys.EnrollmentStatus(userID, subjectID)
}

func UserEnrollmentsListKey(userID string) string { return keys.UserEnrollmentsList(userID) }

func LessonProgressKey(userID, lessonID string) string {
	return keys.LessonProgress(userID, lessonID)
}

func SubjectProgressKey(userID, subjectID string) string {
	return keys.SubjectProgress(userID, subjectID)
}

func PaymentStatusKey(userID, subjectID string) string {
	return keys.PaymentStatus(userID, subjectID)
}

func TeacherListKey(page, limit int) string { return keys.TeacherList(page, limit) }

func TeacherDetailKey(teacherID string) string { return keys.TeacherDetail(teacherID) }
