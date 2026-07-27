package course

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// parseUUID is a helper to convert string to uuid.UUID with error handling
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

var (
	ErrCourseNotFound      = errors.New("course not found")
	ErrSectionNotFound     = errors.New("section not found")
	ErrLessonNotFound      = errors.New("lesson not found")
	ErrEnrollmentNotFound  = errors.New("enrollment not found")
	ErrInvalidStatus       = errors.New("invalid status transition")
	ErrDuplicateEnrollment = errors.New("user already enrolled")
)

// Service defines the business logic interface for courses
type Service interface {
	// Course lifecycle
	CreateCourse(ctx context.Context, course *Course) (*Course, error)
	GetCourse(ctx context.Context, id string) (*Course, error)
	UpdateCourse(ctx context.Context, course *Course) (*Course, error)
	DeleteCourse(ctx context.Context, id string) error
	ListCourses(ctx context.Context, filter CourseFilter) ([]*Course, int, error)
	
	// Course workflow
	SubmitForReview(ctx context.Context, courseID string) error
	ApproveCourse(ctx context.Context, courseID string, reviewerID string, notes string) error
	RejectCourse(ctx context.Context, courseID string, reviewerID string, reason string) error
	ArchiveCourse(ctx context.Context, courseID string) error
	UnarchiveCourse(ctx context.Context, courseID string) error
	
	// Section management
	CreateSection(ctx context.Context, courseID string, section *Section) (*Section, error)
	UpdateSection(ctx context.Context, section *Section) (*Section, error)
	DeleteSection(ctx context.Context, sectionID string) error
	ReorderSections(ctx context.Context, courseID string, sectionIDs []string) error
	
	// Lesson management
	CreateLesson(ctx context.Context, sectionID string, lesson *Lesson) (*Lesson, error)
	UpdateLesson(ctx context.Context, lesson *Lesson) (*Lesson, error)
	DeleteLesson(ctx context.Context, lessonID string) error
	ReorderLessons(ctx context.Context, sectionID string, lessonIDs []string) error
	
	// Enrollment management
	EnrollUser(ctx context.Context, courseID, userID string) (*Enrollment, error)
	GetEnrollment(ctx context.Context, courseID, userID string) (*Enrollment, error)
	UpdateProgress(ctx context.Context, enrollment *Enrollment) error
	CompleteCourse(ctx context.Context, courseID, userID string) error
	ListEnrollments(ctx context.Context, filter EnrollmentFilter) ([]*Enrollment, int, error)
	
	// Pricing management
	SetPricing(ctx context.Context, courseID string, pricing *Pricing) (*Pricing, error)
	GetPricing(ctx context.Context, courseID string) (*Pricing, error)
	
	// Category management
	AddCategory(ctx context.Context, courseID, categoryID string) error
	RemoveCategory(ctx context.Context, courseID, categoryID string) error
	
	// Instructor management
	AddInstructor(ctx context.Context, courseID, instructorID string, role string) error
	RemoveInstructor(ctx context.Context, courseID, instructorID string) error
	
	// Review management
	CreateReview(ctx context.Context, review *Review) (*Review, error)
	UpdateReview(ctx context.Context, review *Review) (*Review, error)
	GetReviews(ctx context.Context, courseID string) ([]*Review, error)
	
	// Certificate management
	IssueCertificate(ctx context.Context, courseID, userID string) (*Certificate, error)
	GetCertificate(ctx context.Context, courseID, userID string) (*Certificate, error)
	GetUserCertificates(ctx context.Context, userID string) ([]*Certificate, error)
}

// CourseService implements the course business logic
type CourseService struct {
	repo Repository
}

// NewCourseService creates a new course service
func NewCourseService(repo Repository) *CourseService {
	return &CourseService{repo: repo}
}

// CreateCourse creates a new course
func (s *CourseService) CreateCourse(ctx context.Context, course *Course) (*Course, error) {
	if err := s.repo.CreateCourse(ctx, course); err != nil {
		return nil, err
	}
	return course, nil
}

// GetCourse retrieves a course by ID
func (s *CourseService) GetCourse(ctx context.Context, id string) (*Course, error) {
	course, err := s.repo.GetCourseByID(ctx, id)
	if err != nil {
		return nil, ErrCourseNotFound
	}
	return course, nil
}

// UpdateCourse updates an existing course
func (s *CourseService) UpdateCourse(ctx context.Context, course *Course) (*Course, error) {
	if err := s.repo.UpdateCourse(ctx, course); err != nil {
		return nil, err
	}
	return course, nil
}

// DeleteCourse deletes a course
func (s *CourseService) DeleteCourse(ctx context.Context, id string) error {
	return s.repo.DeleteCourse(ctx, id)
}

// ListCourses lists courses with filters
func (s *CourseService) ListCourses(ctx context.Context, filter CourseFilter) ([]*Course, int, error) {
	return s.repo.ListCourses(ctx, filter)
}

// SubmitForReview submits a course for review
func (s *CourseService) SubmitForReview(ctx context.Context, courseID string) error {
	course, err := s.repo.GetCourseByID(ctx, courseID)
	if err != nil {
		return ErrCourseNotFound
	}
	
	if !canTransition(course.Status, CourseStatusUnderReview) {
		return ErrInvalidStatus
	}
	
	course.Status = CourseStatusUnderReview
	return s.repo.UpdateCourse(ctx, course)
}

// ApproveCourse approves a course for publication
func (s *CourseService) ApproveCourse(ctx context.Context, courseID string, reviewerID string, notes string) error {
	course, err := s.repo.GetCourseByID(ctx, courseID)
	if err != nil {
		return ErrCourseNotFound
	}
	
	if !canTransition(course.Status, CourseStatusPublished) {
		return ErrInvalidStatus
	}
	
	course.Status = CourseStatusPublished
	now := time.Now()
	course.UpdatedAt = now
	
	return s.repo.UpdateCourse(ctx, course)
}

// RejectCourse rejects a course
func (s *CourseService) RejectCourse(ctx context.Context, courseID string, reviewerID string, reason string) error {
	course, err := s.repo.GetCourseByID(ctx, courseID)
	if err != nil {
		return ErrCourseNotFound
	}
	
	if !canTransition(course.Status, CourseStatusRejected) {
		return ErrInvalidStatus
	}
	
	course.Status = CourseStatusRejected
	return s.repo.UpdateCourse(ctx, course)
}

// ArchiveCourse archives a course
func (s *CourseService) ArchiveCourse(ctx context.Context, courseID string) error {
	course, err := s.repo.GetCourseByID(ctx, courseID)
	if err != nil {
		return ErrCourseNotFound
	}
	
	if !canTransition(course.Status, CourseStatusArchived) {
		return ErrInvalidStatus
	}
	
	course.Status = CourseStatusArchived
	return s.repo.UpdateCourse(ctx, course)
}

// UnarchiveCourse restores a course from archive
func (s *CourseService) UnarchiveCourse(ctx context.Context, courseID string) error {
	course, err := s.repo.GetCourseByID(ctx, courseID)
	if err != nil {
		return ErrCourseNotFound
	}
	
	if !canTransition(course.Status, CourseStatusDraft) {
		return ErrInvalidStatus
	}
	
	course.Status = CourseStatusDraft
	return s.repo.UpdateCourse(ctx, course)
}

// CreateSection creates a new section
func (s *CourseService) CreateSection(ctx context.Context, courseID string, section *Section) (*Section, error) {
	id, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	section.CourseID = id
	if err := s.repo.CreateSection(ctx, section); err != nil {
		return nil, err
	}
	return section, nil
}

// UpdateSection updates a section
func (s *CourseService) UpdateSection(ctx context.Context, section *Section) (*Section, error) {
	if err := s.repo.UpdateSection(ctx, section); err != nil {
		return nil, err
	}
	return section, nil
}

// DeleteSection deletes a section
func (s *CourseService) DeleteSection(ctx context.Context, sectionID string) error {
	return s.repo.DeleteSection(ctx, sectionID)
}

// ReorderSections reorders sections
func (s *CourseService) ReorderSections(ctx context.Context, courseID string, sectionIDs []string) error {
	return s.repo.ReorderSections(ctx, courseID, sectionIDs)
}

// CreateLesson creates a new lesson
func (s *CourseService) CreateLesson(ctx context.Context, sectionID string, lesson *Lesson) (*Lesson, error) {
	id, err := parseUUID(sectionID)
	if err != nil {
		return nil, err
	}
	lesson.SectionID = id
	if err := s.repo.CreateLesson(ctx, lesson); err != nil {
		return nil, err
	}
	return lesson, nil
}

// UpdateLesson updates a lesson
func (s *CourseService) UpdateLesson(ctx context.Context, lesson *Lesson) (*Lesson, error) {
	if err := s.repo.UpdateLesson(ctx, lesson); err != nil {
		return nil, err
	}
	return lesson, nil
}

// DeleteLesson deletes a lesson
func (s *CourseService) DeleteLesson(ctx context.Context, lessonID string) error {
	return s.repo.DeleteLesson(ctx, lessonID)
}

// ReorderLessons reorders lessons
func (s *CourseService) ReorderLessons(ctx context.Context, sectionID string, lessonIDs []string) error {
	return s.repo.ReorderLessons(ctx, sectionID, lessonIDs)
}

// EnrollUser enrolls a user in a course
func (s *CourseService) EnrollUser(ctx context.Context, courseID, userID string) (*Enrollment, error) {
	// Check if already enrolled
	_, err := s.repo.GetEnrollment(ctx, courseID, userID)
	if err == nil {
		return nil, ErrDuplicateEnrollment
	}
	
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	userUUID, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}

	enrollment := &Enrollment{
		CourseID:   courseUUID,
		UserID:     userUUID,
		Progress:   0,
		EnrolledAt: time.Now(),
	}
	
	if err := s.repo.CreateEnrollment(ctx, enrollment); err != nil {
		return nil, err
	}
	return enrollment, nil
}

// GetEnrollment retrieves user enrollment
func (s *CourseService) GetEnrollment(ctx context.Context, courseID, userID string) (*Enrollment, error) {
	enrollment, err := s.repo.GetEnrollment(ctx, courseID, userID)
	if err != nil {
		return nil, ErrEnrollmentNotFound
	}
	return enrollment, nil
}

// UpdateProgress updates enrollment progress
func (s *CourseService) UpdateProgress(ctx context.Context, enrollment *Enrollment) error {
	return s.repo.UpdateEnrollmentProgress(ctx, enrollment)
}

// CompleteCourse marks a course as completed
func (s *CourseService) CompleteCourse(ctx context.Context, courseID, userID string) error {
	enrollment, err := s.GetEnrollment(ctx, courseID, userID)
	if err != nil {
		return err
	}
	
	enrollment.Progress = 100
	now := time.Now()
	enrollment.CompletedAt = &now
	
	return s.repo.UpdateEnrollmentProgress(ctx, enrollment)
}

// ListEnrollments lists enrollments
func (s *CourseService) ListEnrollments(ctx context.Context, filter EnrollmentFilter) ([]*Enrollment, int, error) {
	return s.repo.ListEnrollments(ctx, filter)
}

// SetPricing sets course pricing
func (s *CourseService) SetPricing(ctx context.Context, courseID string, pricing *Pricing) (*Pricing, error) {
	id, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	pricing.CourseID = id
	
	// Try to get existing pricing
	existing, err := s.repo.GetPricing(ctx, courseID)
	if err == nil {
		pricing.ID = existing.ID
		return pricing, s.repo.UpdatePricing(ctx, pricing)
	}
	
	return pricing, s.repo.CreatePricing(ctx, pricing)
}

// GetPricing gets course pricing
func (s *CourseService) GetPricing(ctx context.Context, courseID string) (*Pricing, error) {
	return s.repo.GetPricing(ctx, courseID)
}

// AddCategory adds a category to a course
func (s *CourseService) AddCategory(ctx context.Context, courseID, categoryID string) error {
	return s.repo.AddCourseCategory(ctx, courseID, categoryID)
}

// RemoveCategory removes a category from a course
func (s *CourseService) RemoveCategory(ctx context.Context, courseID, categoryID string) error {
	return s.repo.RemoveCourseCategory(ctx, courseID, categoryID)
}

// AddInstructor adds an instructor to a course
func (s *CourseService) AddInstructor(ctx context.Context, courseID, instructorID string, role string) error {
	return s.repo.AddCourseInstructor(ctx, courseID, instructorID, role)
}

// RemoveInstructor removes an instructor from a course
func (s *CourseService) RemoveInstructor(ctx context.Context, courseID, instructorID string) error {
	return s.repo.RemoveCourseInstructor(ctx, courseID, instructorID)
}

// CreateReview creates a course review
func (s *CourseService) CreateReview(ctx context.Context, review *Review) (*Review, error) {
	if err := s.repo.CreateReview(ctx, review); err != nil {
		return nil, err
	}
	return review, nil
}

// UpdateReview updates a course review
func (s *CourseService) UpdateReview(ctx context.Context, review *Review) (*Review, error) {
	if err := s.repo.UpdateReview(ctx, review); err != nil {
		return nil, err
	}
	return review, nil
}

// GetReviews gets course reviews
func (s *CourseService) GetReviews(ctx context.Context, courseID string) ([]*Review, error) {
	return s.repo.ListReviews(ctx, courseID)
}

// IssueCertificate issues a certificate
func (s *CourseService) IssueCertificate(ctx context.Context, courseID, userID string) (*Certificate, error) {
	// Check if already has certificate
	_, err := s.repo.GetCertificate(ctx, courseID, userID)
	if err == nil {
		return nil, errors.New("certificate already issued")
	}
	
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	userUUID, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}

	certificate := &Certificate{
		CourseID: courseUUID,
		UserID:   userUUID,
		IssuedAt: time.Now(),
	}
	
	if err := s.repo.CreateCertificate(ctx, certificate); err != nil {
		return nil, err
	}
	return certificate, nil
}

// GetCertificate gets a certificate
func (s *CourseService) GetCertificate(ctx context.Context, courseID, userID string) (*Certificate, error) {
	return s.repo.GetCertificate(ctx, courseID, userID)
}

// GetUserCertificates gets all certificates for a user
func (s *CourseService) GetUserCertificates(ctx context.Context, userID string) ([]*Certificate, error) {
	return s.repo.ListCertificates(ctx, userID)
}

// canTransition checks if a status transition is valid
func canTransition(from, to CourseStatus) bool {
	validTransitions := map[CourseStatus][]CourseStatus{
		CourseStatusDraft:         {CourseStatusUnderReview, CourseStatusArchived},
		CourseStatusUnderReview:  {CourseStatusPublished, CourseStatusRejected, CourseStatusDraft},
		CourseStatusPublished:    {CourseStatusArchived, CourseStatusDraft},
		CourseStatusArchived:     {CourseStatusDraft},
		CourseStatusRejected:     {CourseStatusDraft},
	}
	
	validTargets, exists := validTransitions[from]
	if !exists {
		return false
	}
	
	for _, t := range validTargets {
		if t == to {
			return true
		}
	}
	return false
}
