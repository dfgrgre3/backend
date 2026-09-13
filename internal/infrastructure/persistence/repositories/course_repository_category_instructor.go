package repositories

import (
	"context"
	"encoding/json"
	models "thanawy-backend/internal/domain/common"
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
	return r.repo.db.WithContext(ctx).Exec(
		`INSERT INTO "LmsCourseCategory" (course_id, category_id) VALUES (?, ?) ON CONFLICT DO NOTHING`,
		courseUUID, catUUID,
	).Error
}

func (r *GormRepository) RemoveCourseCategory(ctx context.Context, courseID, categoryID string) error {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return err
	}
	catUUID, err := parseUUID(categoryID)
	if err != nil {
		return err
	}
	return r.repo.db.WithContext(ctx).Where("course_id = ? AND category_id = ?", courseUUID, catUUID).
		Delete(&models.LmsCourseCategory{}).Error
}

func (r *GormRepository) ListCourseCategories(ctx context.Context, courseID string) ([]*Category, error) {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	var categories []models.LmsCategory
	err = r.repo.db.WithContext(ctx).
		Joins(`JOIN "LmsCourseCategory" cc ON cc.category_id = "LmsCategory".id`).
		Where(`cc.course_id = ? AND "LmsCategory".deleted_at IS NULL`, courseUUID).
		Order(`"LmsCategory".name ASC`).Find(&categories).Error
	if err != nil {
		return nil, err
	}
	result := make([]*Category, len(categories))
	for i := range categories {
		result[i] = &Category{
			ID: categories[i].ID, Name: categories[i].Name, Slug: categories[i].Slug,
			ParentID: categories[i].ParentID, CreatedAt: categories[i].CreatedAt, UpdatedAt: categories[i].UpdatedAt,
		}
	}
	return result, nil
}

// Instructor operations
func (r *GormRepository) AddCourseInstructor(ctx context.Context, courseID, instructorID string, role string) error {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return err
	}
	instructorUUID, err := parseUUID(instructorID)
	if err != nil {
		return err
	}
	if role == "" {
		role = "INSTRUCTOR"
	}
	return r.repo.db.WithContext(ctx).Exec(
		`INSERT INTO "LmsInstructor" (course_id, instructor_id, role, permissions) VALUES (?, ?, ?, ?::jsonb) ON CONFLICT (course_id, instructor_id) DO UPDATE SET role = EXCLUDED.role`,
		courseUUID, instructorUUID, role, json.RawMessage(`{}`),
	).Error
}

func (r *GormRepository) RemoveCourseInstructor(ctx context.Context, courseID, instructorID string) error {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return err
	}
	instructorUUID, err := parseUUID(instructorID)
	if err != nil {
		return err
	}
	return r.repo.db.WithContext(ctx).Where("course_id = ? AND instructor_id = ?", courseUUID, instructorUUID).
		Delete(&models.LmsInstructor{}).Error
}

func (r *GormRepository) ListCourseInstructors(ctx context.Context, courseID string) ([]*Instructor, error) {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	var instructors []models.LmsInstructor
	if err := r.repo.db.WithContext(ctx).Where("course_id = ?", courseUUID).
		Order("created_at ASC").Find(&instructors).Error; err != nil {
		return nil, err
	}
	result := make([]*Instructor, len(instructors))
	for i := range instructors {
		result[i] = &Instructor{
			CourseID: instructors[i].CourseID, InstructorID: instructors[i].InstructorID,
			Role: instructors[i].Role, Permissions: []byte(instructors[i].Permissions), CreatedAt: instructors[i].CreatedAt,
		}
	}
	return result, nil
}
