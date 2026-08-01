package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"thanawy-backend/internal/models"
	"thanawy-backend/internal/repository"
)

// LmsService contains business logic for the LMS module.
type LmsService struct {
	repo *repository.LmsRepository
}

// NewLmsService creates a new LmsService.
func NewLmsService(repo *repository.LmsRepository) *LmsService {
	return &LmsService{repo: repo}
}

// ----------------------------
// Course
// ----------------------------

// CreateCourse creates a new course.
func (s *LmsService) CreateCourse(title, slug string, instructorID uuid.UUID) (*models.LmsCourse, error) {
	if title == "" || slug == "" {
		return nil, errors.New("title and slug are required")
	}
	course := &models.LmsCourse{
		Title:               title,
		Slug:                slug,
		PrimaryInstructorID: instructorID,
		Status:              models.CourseStatusDraft,
	}
	if err := s.repo.CreateCourse(course); err != nil {
		return nil, fmt.Errorf("failed to create course: %w", err)
	}
	return course, nil
}

// GetCourse retrieves a course by ID.
func (s *LmsService) GetCourse(id uuid.UUID) (*models.LmsCourse, error) {
	return s.repo.GetCourseByID(id)
}

// GetCourseBySlug retrieves a course by slug.
func (s *LmsService) GetCourseBySlug(slug string) (*models.LmsCourse, error) {
	return s.repo.GetCourseBySlug(slug)
}

// UpdateCourse updates a course.
func (s *LmsService) UpdateCourse(course *models.LmsCourse) error {
	return s.repo.UpdateCourse(course)
}

// DeleteCourse soft-deletes a course.
func (s *LmsService) DeleteCourse(id uuid.UUID) error {
	return s.repo.DeleteCourse(id)
}

// ListCourses returns paginated courses.
func (s *LmsService) ListCourses(page, pageSize int, status, level, language string) ([]models.LmsCourse, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return s.repo.ListCourses(page, pageSize, status, level, language)
}

// ----------------------------
// Workflow
// ----------------------------

// SubmitForReview transitions a course from draft to pending_review.
func (s *LmsService) SubmitForReview(id uuid.UUID, userID uuid.UUID) error {
	course, err := s.repo.GetCourseByID(id)
	if err != nil {
		return err
	}
	if course.Status != models.CourseStatusDraft {
		return errors.New("course must be in draft state")
	}
	if err := s.repo.UpdateCourseStatus(id, models.CourseStatusUnderReview); err != nil {
		return err
	}
	s.addChangelog(id, userID, "status", string(course.Status), string(models.CourseStatusUnderReview))
	return nil
}

// ApproveCourse transitions a course from pending_review to published.
func (s *LmsService) ApproveCourse(id uuid.UUID, userID uuid.UUID) error {
	course, err := s.repo.GetCourseByID(id)
	if err != nil {
		return err
	}
	if course.Status != models.CourseStatusUnderReview {
		return errors.New("course must be in pending_review state")
	}
	if err := s.repo.UpdateCourseStatus(id, models.CourseStatusPublished); err != nil {
		return err
	}
	s.addChangelog(id, userID, "status", string(course.Status), string(models.CourseStatusPublished))
	return nil
}

// RejectCourse transitions a course from pending_review back to draft.
func (s *LmsService) RejectCourse(id uuid.UUID, userID uuid.UUID, reason string) error {
	course, err := s.repo.GetCourseByID(id)
	if err != nil {
		return err
	}
	if course.Status != models.CourseStatusUnderReview {
		return errors.New("course must be in pending_review state")
	}
	if err := s.repo.UpdateCourseStatus(id, models.CourseStatusDraft); err != nil {
		return err
	}
	s.addChangelog(id, userID, "status", string(course.Status), string(models.CourseStatusDraft))
	return nil
}

// ArchiveCourse transitions a published course to archived.
func (s *LmsService) ArchiveCourse(id uuid.UUID, userID uuid.UUID) error {
	course, err := s.repo.GetCourseByID(id)
	if err != nil {
		return err
	}
	if course.Status != models.CourseStatusPublished {
		return errors.New("course must be published")
	}
	if err := s.repo.UpdateCourseStatus(id, models.CourseStatusArchived); err != nil {
		return err
	}
	s.addChangelog(id, userID, "status", string(course.Status), string(models.CourseStatusArchived))
	return nil
}

// RestoreCourse transitions an archived course back to published.
func (s *LmsService) RestoreCourse(id uuid.UUID, userID uuid.UUID) error {
	course, err := s.repo.GetCourseByID(id)
	if err != nil {
		return err
	}
	if course.Status != models.CourseStatusArchived {
		return errors.New("course must be archived")
	}
	if err := s.repo.UpdateCourseStatus(id, models.CourseStatusPublished); err != nil {
		return err
	}
	s.addChangelog(id, userID, "status", string(course.Status), string(models.CourseStatusPublished))
	return nil
}

// ----------------------------
// Sections & Lessons
// ----------------------------

func (s *LmsService) CreateSection(courseID uuid.UUID, title string, orderIndex int) (*models.LmsSection, error) {
	section := &models.LmsSection{
		CourseID:   courseID,
		Title:      title,
		OrderIndex: orderIndex,
	}
	if err := s.repo.CreateSection(section); err != nil {
		return nil, err
	}
	return section, nil
}

func (s *LmsService) ListSections(courseID uuid.UUID) ([]models.LmsSection, error) {
	return s.repo.ListSectionsByCourseID(courseID)
}

func (s *LmsService) CreateLesson(sectionID uuid.UUID, title string, lessonType models.LessonType, orderIndex int) (*models.LmsLesson, error) {
	lesson := &models.LmsLesson{
		SectionID:  sectionID,
		Title:      title,
		Type:       lessonType,
		OrderIndex: orderIndex,
	}
	if err := s.repo.CreateLesson(lesson); err != nil {
		return nil, err
	}
	return lesson, nil
}

func (s *LmsService) GetLesson(id uuid.UUID) (*models.LmsLesson, error) {
	return s.repo.GetLessonByID(id)
}

func (s *LmsService) ListLessons(sectionID uuid.UUID) ([]models.LmsLesson, error) {
	return s.repo.ListLessonsBySectionID(sectionID)
}

// ----------------------------
// Enrollment & Progress
// ----------------------------

// EnrollUser enrolls a user in a course.
func (s *LmsService) EnrollUser(courseID, userID uuid.UUID) (*models.LmsEnrollment, error) {
	existing, err := s.repo.GetEnrollment(courseID, userID)
	if err == nil && existing != nil {
		return existing, nil
	}
	enrollment := &models.LmsEnrollment{
		CourseID:   courseID,
		UserID:     userID,
		EnrolledAt: time.Now().UTC(),
	}
	if err := s.repo.CreateEnrollment(enrollment); err != nil {
		return nil, err
	}
	return enrollment, nil
}

// EnrollUserInBundle enrolls a user in all courses of a bundle.
func (s *LmsService) EnrollUserInBundle(bundleID, userID uuid.UUID) error {
	return s.repo.EnrollUserInBundle(bundleID, userID)
}

// UpdateProgress updates the user's progress in a course.
func (s *LmsService) UpdateProgress(courseID, userID uuid.UUID, progress float64) error {
	return s.repo.UpdateEnrollmentProgress(courseID, userID, progress)
}

// CompleteCourse marks a course as completed and generates a certificate if applicable.
func (s *LmsService) CompleteCourse(courseID, userID uuid.UUID) error {
	course, err := s.repo.GetCourseByID(courseID)
	if err != nil {
		return err
	}
	if err := s.repo.CompleteEnrollment(courseID, userID); err != nil {
		return err
	}
	if course.HasCertificate {
		return s.generateCertificate(courseID, userID)
	}
	return nil
}

// ListUserEnrollments returns all enrollments for a user.
func (s *LmsService) ListUserEnrollments(userID uuid.UUID) ([]models.LmsEnrollment, error) {
	return s.repo.ListEnrollmentsByUserID(userID)
}

// ----------------------------
// Lesson Interaction & Video Notes
// ----------------------------

// UpdateLessonInteraction upserts a lesson interaction record.
func (s *LmsService) UpdateLessonInteraction(lessonID, userID uuid.UUID, watchedDuration, lastPosition int, playCount int, isCompleted bool, quizAnswers json.RawMessage) error {
	interaction := &models.LmsLessonInteraction{
		LessonID:           lessonID,
		UserID:             userID,
		WatchedDurationSec: watchedDuration,
		LastPositionSec:    lastPosition,
		PlayCount:          playCount,
		IsCompleted:        isCompleted,
		QuizAnswers:        quizAnswers,
	}
	return s.repo.UpsertLessonInteraction(interaction)
}

// GetLessonInteraction retrieves a user's interaction with a lesson.
func (s *LmsService) GetLessonInteraction(lessonID, userID uuid.UUID) (*models.LmsLessonInteraction, error) {
	return s.repo.GetLessonInteraction(lessonID, userID)
}

// AddVideoNote adds a timestamped note for a lesson.
func (s *LmsService) AddVideoNote(lessonID, userID uuid.UUID, timestampSec int, note string) (*models.LmsVideoNote, error) {
	n := &models.LmsVideoNote{
		LessonID:     lessonID,
		UserID:       userID,
		TimestampSec: timestampSec,
		Note:         note,
	}
	if err := s.repo.CreateVideoNote(n); err != nil {
		return nil, err
	}
	return n, nil
}

// ListVideoNotes returns all video notes for a user in a lesson.
func (s *LmsService) ListVideoNotes(lessonID, userID uuid.UUID) ([]models.LmsVideoNote, error) {
	return s.repo.ListVideoNotes(lessonID, userID)
}

// ----------------------------
// Attachments & Subtitles
// ----------------------------

func (s *LmsService) AddAttachment(lessonID uuid.UUID, title, fileURL, fileType string, fileSize *int64) (*models.LmsAttachment, error) {
	a := &models.LmsAttachment{
		LessonID: lessonID,
		Title:    title,
		FileURL:  fileURL,
		FileType: fileType,
		FileSize: fileSize,
	}
	if err := s.repo.CreateAttachment(a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *LmsService) ListAttachments(lessonID uuid.UUID) ([]models.LmsAttachment, error) {
	return s.repo.ListAttachments(lessonID)
}

func (s *LmsService) AddSubtitle(lessonID uuid.UUID, language, vttURL string) (*models.LmsSubtitle, error) {
	sub := &models.LmsSubtitle{
		LessonID: lessonID,
		Language: language,
		VTTURL:   vttURL,
	}
	if err := s.repo.CreateSubtitle(sub); err != nil {
		return nil, err
	}
	return sub, nil
}

func (s *LmsService) ListSubtitles(lessonID uuid.UUID) ([]models.LmsSubtitle, error) {
	return s.repo.ListSubtitles(lessonID)
}

// ----------------------------
// Interactive Quizzes
// ----------------------------

func (s *LmsService) AddQuiz(lessonID uuid.UUID, timestampSec int, question string, options json.RawMessage, correctIndex int) (*models.LmsInteractiveQuiz, error) {
	q := &models.LmsInteractiveQuiz{
		LessonID:     lessonID,
		TimestampSec: timestampSec,
		Question:     question,
		Options:      options,
		CorrectIndex: correctIndex,
	}
	if err := s.repo.CreateQuiz(q); err != nil {
		return nil, err
	}
	return q, nil
}

func (s *LmsService) ListQuizzes(lessonID uuid.UUID) ([]models.LmsInteractiveQuiz, error) {
	return s.repo.ListQuizzes(lessonID)
}

// ----------------------------
// Pricing
// ----------------------------

func (s *LmsService) AddPricing(courseID uuid.UUID, priceType models.PriceType, amount float64, currencyCode string, subDurationDays *int) (*models.LmsPricing, error) {
	decimalAmount := decimal.NewFromFloat(amount)
	p := &models.LmsPricing{
		CourseID:                 courseID,
		Type:                     priceType,
		Amount:                   decimalAmount,
		CurrencyCode:             currencyCode,
		SubscriptionDurationDays: subDurationDays,
		IsActive:                 true,
	}
	if err := s.repo.CreatePricing(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *LmsService) ListPricings(courseID uuid.UUID) ([]models.LmsPricing, error) {
	return s.repo.ListPricingsByCourseID(courseID)
}

// ----------------------------
// Bundles
// ----------------------------

func (s *LmsService) CreateBundle(title, slug string, price float64, currencyCode string) (*models.LmsBundle, error) {
	decimalPrice := decimal.NewFromFloat(price)
	b := &models.LmsBundle{
		Title:        title,
		Slug:         slug,
		Price:        decimalPrice,
		CurrencyCode: currencyCode,
		IsActive:     true,
	}
	if err := s.repo.CreateBundle(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *LmsService) GetBundle(id uuid.UUID) (*models.LmsBundle, error) {
	return s.repo.GetBundleByID(id)
}

func (s *LmsService) ListBundles(page, pageSize int) ([]models.LmsBundle, int64, error) {
	return s.repo.ListBundles(page, pageSize)
}

func (s *LmsService) AddCourseToBundle(bundleID, courseID uuid.UUID) error {
	return s.repo.AddCourseToBundle(bundleID, courseID)
}

func (s *LmsService) RemoveCourseFromBundle(bundleID, courseID uuid.UUID) error {
	return s.repo.RemoveCourseFromBundle(bundleID, courseID)
}

// ----------------------------
// Categories & Tags
// ----------------------------

func (s *LmsService) CreateCategory(name, slug string, parentID *uuid.UUID) (*models.LmsCategory, error) {
	c := &models.LmsCategory{
		Name:     name,
		Slug:     slug,
		ParentID: parentID,
	}
	if err := s.repo.CreateCategory(c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *LmsService) ListCategories() ([]models.LmsCategory, error) {
	return s.repo.ListCategories()
}

func (s *LmsService) CreateTag(name string) (*models.LmsTag, error) {
	t := &models.LmsTag{Name: name}
	if err := s.repo.CreateTag(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *LmsService) ListTags() ([]models.LmsTag, error) {
	return s.repo.ListTags()
}

func (s *LmsService) SetCourseCategories(courseID uuid.UUID, categoryIDs []uuid.UUID) error {
	return s.repo.SetCourseCategories(courseID, categoryIDs)
}

func (s *LmsService) SetCourseTags(courseID uuid.UUID, tagIDs []uuid.UUID) error {
	return s.repo.SetCourseTags(courseID, tagIDs)
}

// ----------------------------
// Instructors
// ----------------------------

func (s *LmsService) AddInstructor(courseID, instructorID uuid.UUID, role string, permissions json.RawMessage) error {
	i := &models.LmsInstructor{
		CourseID:     courseID,
		InstructorID: instructorID,
		Role:         role,
		Permissions:  permissions,
	}
	return s.repo.AddInstructor(i)
}

func (s *LmsService) RemoveInstructor(courseID, instructorID uuid.UUID) error {
	return s.repo.RemoveInstructor(courseID, instructorID)
}

func (s *LmsService) ListInstructors(courseID uuid.UUID) ([]models.LmsInstructor, error) {
	return s.repo.ListInstructors(courseID)
}

// ----------------------------
// Changelog & Versions
// ----------------------------

func (s *LmsService) ListChangelogs(courseID uuid.UUID) ([]models.LmsCourseChangelog, error) {
	return s.repo.ListChangelogs(courseID)
}

func (s *LmsService) ListVersions(courseID uuid.UUID) ([]models.LmsCourseVersion, error) {
	return s.repo.ListVersions(courseID)
}

// CreateVersion snapshots the current course state as a new version.
func (s *LmsService) CreateVersion(courseID uuid.UUID, versionNumber int) (*models.LmsCourseVersion, error) {
	snapshot, err := s.repo.SnapshotCourse(courseID)
	if err != nil {
		return nil, err
	}
	v := &models.LmsCourseVersion{
		CourseID:      courseID,
		VersionNumber: versionNumber,
		Snapshot:      snapshot,
	}
	if err := s.repo.CreateVersion(v); err != nil {
		return nil, err
	}
	return v, nil
}

// CloneCourse duplicates a course into a new draft.
func (s *LmsService) CloneCourse(srcID uuid.UUID, newTitle string) (*models.LmsCourse, error) {
	return s.repo.CloneCourse(srcID, newTitle)
}

// ----------------------------
// Certificates
// ----------------------------

// generateCertificate creates a certificate for a completed course.
func (s *LmsService) generateCertificate(courseID, userID uuid.UUID) error {
	certNo := fmt.Sprintf("CERT-%s-%d", courseID.String()[:8], time.Now().Unix())
	cert := &models.LmsCertificate{
		CourseID:      courseID,
		UserID:        userID,
		CertificateNo: certNo,
		PDFURL:        fmt.Sprintf("/certificates/%s.pdf", certNo),
	}
	return s.repo.CreateCertificate(cert)
}

func (s *LmsService) GetCertificate(courseID, userID uuid.UUID) (*models.LmsCertificate, error) {
	return s.repo.GetCertificate(courseID, userID)
}

func (s *LmsService) ListUserCertificates(userID uuid.UUID) ([]models.LmsCertificate, error) {
	return s.repo.ListCertificatesByUser(userID)
}

// ----------------------------
// Reviews
// ----------------------------

func (s *LmsService) CreateReview(courseID, userID uuid.UUID, rating int, comment string) (*models.LmsReview, error) {
	if rating < 1 || rating > 5 {
		return nil, errors.New("rating must be between 1 and 5")
	}
	rev := &models.LmsReview{
		CourseID: courseID,
		UserID:   userID,
		Rating:   rating,
		Comment:  comment,
		Status:   "PENDING",
	}
	if err := s.repo.CreateReview(rev); err != nil {
		return nil, err
	}
	return rev, nil
}

func (s *LmsService) ListReviews(courseID uuid.UUID, status string) ([]models.LmsReview, error) {
	return s.repo.ListReviews(courseID, status)
}

func (s *LmsService) UpdateReviewStatus(id uuid.UUID, status string) error {
	return s.repo.UpdateReviewStatus(id, status)
}

func (s *LmsService) ReplyToReview(id uuid.UUID, reply string) error {
	return s.repo.ReplyToReview(id, reply)
}

// ----------------------------
// Review Comments (Workflow)
// ----------------------------

func (s *LmsService) AddReviewComment(courseID, reviewerID uuid.UUID, comment, status string) (*models.LmsReviewComment, error) {
	rc := &models.LmsReviewComment{
		CourseID:   courseID,
		ReviewerID: reviewerID,
		Comment:    comment,
		Status:     status,
	}
	if err := s.repo.CreateReviewComment(rc); err != nil {
		return nil, err
	}
	return rc, nil
}

func (s *LmsService) ListReviewComments(courseID uuid.UUID) ([]models.LmsReviewComment, error) {
	return s.repo.ListReviewComments(courseID)
}

// ----------------------------
// Helpers
// ----------------------------

func (s *LmsService) addChangelog(courseID, userID uuid.UUID, field, oldVal, newVal string) {
	oldPtr := &oldVal
	newPtr := &newVal
	entry := &models.LmsCourseChangelog{
		CourseID: courseID,
		UserID:   userID,
		Field:    field,
		OldValue: oldPtr,
		NewValue: newPtr,
	}
	_ = s.repo.AddChangelog(entry)
}
