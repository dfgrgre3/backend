package models

import (
	"context"
	"time"
)

// CourseService implements the course business logic
type CourseService struct {
	repo DomainRepository
}

// NewCourseService creates a new course service
func NewCourseService(repo DomainRepository) *CourseService {
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

// GetCourseBySlug retrieves a course by slug
func (s *CourseService) GetCourseBySlug(ctx context.Context, slug string) (*Course, error) {
	course, err := s.repo.GetCourseBySlug(ctx, slug)
	if err != nil {
		return nil, err
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
	course.UpdatedAt = time.Now()
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
	course.UpdatedAt = time.Now()
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
	course.UpdatedAt = time.Now()
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
	course.UpdatedAt = time.Now()
	return s.repo.UpdateCourse(ctx, course)
}
