package repositories

import (
	"context"
)

// Course operations
func (r *GormRepository) CreateCourse(ctx context.Context, course *Course) error {
	return r.repo.CreateCourse(r.toModelCourse(course))
}

func (r *GormRepository) GetCourseByID(ctx context.Context, id string) (*Course, error) {
	courseUUID, err := parseUUID(id)
	if err != nil {
		return nil, err
	}
	modelCourse, err := r.repo.GetCourseByID(courseUUID)
	if err != nil {
		return nil, err
	}
	return r.toDomainCourse(modelCourse), nil
}

func (r *GormRepository) GetCourseBySlug(ctx context.Context, slug string) (*Course, error) {
	modelCourse, err := r.repo.GetCourseBySlug(slug)
	if err != nil {
		return nil, err
	}
	return r.toDomainCourse(modelCourse), nil
}

func (r *GormRepository) UpdateCourse(ctx context.Context, course *Course) error {
	return r.repo.UpdateCourse(r.toModelCourse(course))
}

func (r *GormRepository) DeleteCourse(ctx context.Context, id string) error {
	courseUUID, err := parseUUID(id)
	if err != nil {
		return err
	}
	return r.repo.DeleteCourse(courseUUID)
}

func (r *GormRepository) ListCourses(ctx context.Context, filter CourseFilter) ([]*Course, int, error) {
	var status, level, language *string
	if filter.Status != nil {
		s := string(*filter.Status)
		status = &s
	}
	if filter.Level != nil {
		l := string(*filter.Level)
		level = &l
	}
	if filter.Language != nil {
		language = filter.Language
	}

	modelCourses, total, err := r.repo.ListCourses(filter.Page, filter.Limit,
		derefString(status), derefString(level), derefString(language))
	if err != nil {
		return nil, 0, err
	}

	courses := make([]*Course, len(modelCourses))
	for i, mc := range modelCourses {
		courses[i] = r.toDomainCourse(&mc)
	}
	return courses, int(total), nil
}
