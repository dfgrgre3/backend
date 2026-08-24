package models

import (
	"context"

	"github.com/google/uuid"
)

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

// ListSections lists sections for a course
func (s *CourseService) ListSections(ctx context.Context, courseID uuid.UUID) ([]*Section, error) {
	return s.repo.ListSections(ctx, courseID.String())
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

// ListLessons lists lessons for a section
func (s *CourseService) ListLessons(ctx context.Context, sectionID uuid.UUID) ([]*Lesson, error) {
	return s.repo.ListLessons(ctx, sectionID.String())
}
