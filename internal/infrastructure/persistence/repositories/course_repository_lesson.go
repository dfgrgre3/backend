package repositories

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	models "thanawy-backend/internal/domain/common"
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
	sectionUUID, err := parseUUID(sectionID)
	if err != nil {
		return err
	}
	return r.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range lessonIDs {
			lessonUUID, parseErr := parseUUID(id)
			if parseErr != nil {
				return parseErr
			}
			result := tx.Model(&models.LmsLesson{}).Where("id = ? AND section_id = ? AND deleted_at IS NULL", lessonUUID, sectionUUID).Update("order_index", i)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("lesson %s does not belong to section %s", id, sectionID)
			}
		}
		return nil
	})
}
