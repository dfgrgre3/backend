package models

import (
	"context"
)

// CloneCourse duplicates a course with all sections/lessons into a new draft
func (s *CourseService) CloneCourse(ctx context.Context, courseID string, newTitle string) (*Course, error) {
	return s.repo.CloneCourse(ctx, courseID, newTitle)
}

// CreateVersion creates a new version snapshot of the course
func (s *CourseService) CreateVersion(ctx context.Context, courseID string, userID string) (*DomainCourseVersion, error) {
	return s.repo.CreateVersion(ctx, courseID, userID)
}

// ListVersions lists all versions of a course
func (s *CourseService) ListVersions(ctx context.Context, courseID string) ([]*DomainCourseVersion, error) {
	return s.repo.ListVersions(ctx, courseID)
}

// RestoreVersion restores a course to a specific version
func (s *CourseService) RestoreVersion(ctx context.Context, courseID string, versionNumber int, userID string) (*Course, error) {
	return s.repo.RestoreVersion(ctx, courseID, versionNumber, userID)
}

// GetChangelog returns the changelog for a course
func (s *CourseService) GetChangelog(ctx context.Context, courseID string) ([]*CourseChangelog, error) {
	return s.repo.GetChangelog(ctx, courseID)
}

// canTransition checks if a status transition is valid
func canTransition(from, to CourseStatus) bool {
	validTransitions := map[CourseStatus][]CourseStatus{
		CourseStatusDraft:       {CourseStatusUnderReview, CourseStatusArchived},
		CourseStatusUnderReview: {CourseStatusPublished, CourseStatusRejected, CourseStatusDraft},
		CourseStatusPublished:   {CourseStatusArchived, CourseStatusDraft},
		CourseStatusArchived:    {CourseStatusDraft},
		CourseStatusRejected:    {CourseStatusDraft},
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
