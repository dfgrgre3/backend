package repositories

import (
	"context"

	"github.com/google/uuid"
)

// Category operations
func (r *GormRepository) AddCourseCategory(ctx context.Context, courseID, categoryID string) error {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return err
	}
	catUUID, err := parseUUID(categoryID)
	if err != nil {
		return err
	}
	return r.repo.SetCourseCategories(courseUUID, []uuid.UUID{catUUID})
}

func (r *GormRepository) RemoveCourseCategory(ctx context.Context, courseID, categoryID string) error {
	return nil
}

func (r *GormRepository) ListCourseCategories(ctx context.Context, courseID string) ([]*Category, error) {
	return nil, nil
}

// Instructor operations
func (r *GormRepository) AddCourseInstructor(ctx context.Context, courseID, instructorID string, role string) error {
	return nil
}

func (r *GormRepository) RemoveCourseInstructor(ctx context.Context, courseID, instructorID string) error {
	return nil
}

func (r *GormRepository) ListCourseInstructors(ctx context.Context, courseID string) ([]*Instructor, error) {
	return nil, nil
}
