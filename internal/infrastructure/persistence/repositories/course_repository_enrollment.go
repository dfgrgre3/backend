package repositories

import (
	"context"
	models "thanawy-backend/internal/domain/common"
)

// Enrollment operations
func (r *GormRepository) CreateEnrollment(ctx context.Context, enrollment *Enrollment) error {
	return r.repo.CreateEnrollment(r.toModelEnrollment(enrollment))
}

func (r *GormRepository) GetEnrollment(ctx context.Context, courseID, userID string) (*Enrollment, error) {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	userUUID, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	modelEnrollment, err := r.repo.GetEnrollment(courseUUID, userUUID)
	if err != nil {
		return nil, err
	}
	return r.toDomainEnrollment(modelEnrollment), nil
}

func (r *GormRepository) UpdateEnrollmentProgress(ctx context.Context, enrollment *Enrollment) error {
	courseUUID, err := parseUUID(enrollment.CourseID.String())
	if err != nil {
		return err
	}
	userUUID, err := parseUUID(enrollment.UserID.String())
	if err != nil {
		return err
	}
	return r.repo.UpdateEnrollmentProgress(courseUUID, userUUID, float64(enrollment.Progress))
}

func (r *GormRepository) ListEnrollments(ctx context.Context, filter EnrollmentFilter) ([]*Enrollment, int, error) {
	query := r.repo.db.WithContext(ctx).Model(&models.LmsEnrollment{})

	if filter.CourseID != nil && *filter.CourseID != "" {
		courseUUID, err := parseUUID(*filter.CourseID)
		if err != nil {
			return nil, 0, err
		}
		query = query.Where("course_id = ?", courseUUID)
	}
	if filter.UserID != nil && *filter.UserID != "" {
		userUUID, err := parseUUID(*filter.UserID)
		if err != nil {
			return nil, 0, err
		}
		query = query.Where("user_id = ?", userUUID)
	}
	if filter.Status != nil && *filter.Status != "" {
		switch *filter.Status {
		case "completed", "COMPLETED":
			query = query.Where("completed_at IS NOT NULL")
		case "active", "ACTIVE", "in_progress", "IN_PROGRESS":
			query = query.Where("completed_at IS NULL")
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := (page - 1) * limit

	var modelsList []models.LmsEnrollment
	if err := query.Order("enrolled_at DESC").Offset(offset).Limit(limit).Find(&modelsList).Error; err != nil {
		return nil, 0, err
	}

	result := make([]*Enrollment, len(modelsList))
	for i := range modelsList {
		result[i] = r.toDomainEnrollment(&modelsList[i])
	}
	return result, int(total), nil
}
