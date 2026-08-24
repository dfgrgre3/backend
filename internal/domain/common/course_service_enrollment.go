package models

import (
	"context"
	"time"
)

// EnrollUser enrolls a user in a course
func (s *CourseService) EnrollUser(ctx context.Context, courseID, userID string) (*DomainEnrollment, error) {
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

	enrollment := &DomainEnrollment{
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
func (s *CourseService) GetEnrollment(ctx context.Context, courseID, userID string) (*DomainEnrollment, error) {
	enrollment, err := s.repo.GetEnrollment(ctx, courseID, userID)
	if err != nil {
		return nil, ErrEnrollmentNotFound
	}
	return enrollment, nil
}

// UpdateProgress updates enrollment progress
func (s *CourseService) UpdateProgress(ctx context.Context, enrollment *DomainEnrollment) error {
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
func (s *CourseService) ListEnrollments(ctx context.Context, filter EnrollmentFilter) ([]*DomainEnrollment, int, error) {
	return s.repo.ListEnrollments(ctx, filter)
}
