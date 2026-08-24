package repositories

import (
	"context"
)

// Lesson operations
func (r *GormRepository) CreateLesson(ctx context.Context, lesson *Lesson) error {
	return r.repo.CreateLesson(r.toModelLesson(lesson))
}

func (r *GormRepository) GetLessonByID(ctx context.Context, id string) (*Lesson, error) {
	lessonUUID, err := parseUUID(id)
	if err != nil {
		return nil, err
	}
	modelLesson, err := r.repo.GetLessonByID(lessonUUID)
	if err != nil {
		return nil, err
	}
	return r.toDomainLesson(modelLesson), nil
}

func (r *GormRepository) UpdateLesson(ctx context.Context, lesson *Lesson) error {
	return r.repo.UpdateLesson(r.toModelLesson(lesson))
}

func (r *GormRepository) DeleteLesson(ctx context.Context, id string) error {
	lessonUUID, err := parseUUID(id)
	if err != nil {
		return err
	}
	return r.repo.DeleteLesson(lessonUUID)
}

func (r *GormRepository) ListLessons(ctx context.Context, sectionID string) ([]*Lesson, error) {
	sectionUUID, err := parseUUID(sectionID)
	if err != nil {
		return nil, err
	}
	modelLessons, err := r.repo.ListLessonsBySectionID(sectionUUID)
	if err != nil {
		return nil, err
	}

	lessons := make([]*Lesson, len(modelLessons))
	for i, ml := range modelLessons {
		lessons[i] = r.toDomainLesson(&ml)
	}
	return lessons, nil
}

func (r *GormRepository) ReorderLessons(ctx context.Context, sectionID string, lessonIDs []string) error {
	// Implementation would update order_index for each lesson
	return nil
}
