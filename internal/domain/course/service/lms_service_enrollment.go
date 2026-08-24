package courseservice

import (
	models "thanawy-backend/internal/domain/common"
	"time"

	"github.com/google/uuid"
)

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

// ListCourseEnrollments returns all enrollments for a course.
func (s *LmsService) ListCourseEnrollments(courseID uuid.UUID) ([]models.LmsEnrollment, error) {
	return s.repo.ListEnrollmentsByCourseID(courseID)
}
