package repositories

import (
	"context"
)

// Course versioning & cloning
func (r *GormRepository) CloneCourse(ctx context.Context, courseID string, newTitle string) (*Course, error) {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	modelCourse, err := r.repo.CloneCourse(courseUUID, newTitle)
	if err != nil {
		return nil, err
	}
	return r.toDomainCourse(modelCourse), nil
}

func (r *GormRepository) CreateVersion(ctx context.Context, courseID string, userID string) (*CourseVersion, error) {
	return nil, nil
}

func (r *GormRepository) ListVersions(ctx context.Context, courseID string) ([]*CourseVersion, error) {
	return nil, nil
}

func (r *GormRepository) RestoreVersion(ctx context.Context, courseID string, versionNumber int, userID string) (*Course, error) {
	return nil, nil
}

func (r *GormRepository) GetChangelog(ctx context.Context, courseID string) ([]*CourseChangelog, error) {
	return nil, nil
}
