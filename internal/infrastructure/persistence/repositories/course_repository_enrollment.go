package repositories

import (
	"context"
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
	return nil, 0, nil
}
