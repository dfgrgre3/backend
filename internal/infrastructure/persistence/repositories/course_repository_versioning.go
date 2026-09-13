package repositories

import (
	"context"
	models "thanawy-backend/internal/domain/common"
	"time"
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
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	userUUID, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	snapshot, err := r.repo.SnapshotCourse(courseUUID)
	if err != nil {
		return nil, err
	}
	var next int
	if err := r.repo.db.WithContext(ctx).Model(&models.LmsCourseVersion{}).
		Where("course_id = ?", courseUUID).Select("COALESCE(MAX(version_number), 0) + 1").Scan(&next).Error; err != nil {
		return nil, err
	}
	model := &models.LmsCourseVersion{CourseID: courseUUID, VersionNumber: next, Snapshot: snapshot, CreatedAt: time.Now()}
	if err := r.repo.db.WithContext(ctx).Create(model).Error; err != nil {
		return nil, err
	}
	return &CourseVersion{ID: model.ID, CourseID: model.CourseID, VersionNumber: model.VersionNumber, Snapshot: []byte(model.Snapshot), CreatedBy: userUUID, CreatedAt: model.CreatedAt}, nil
}

func (r *GormRepository) ListVersions(ctx context.Context, courseID string) ([]*CourseVersion, error) {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	var modelsList []models.LmsCourseVersion
	if err := r.repo.db.WithContext(ctx).Where("course_id = ?", courseUUID).
		Order("version_number DESC").Find(&modelsList).Error; err != nil {
		return nil, err
	}
	result := make([]*CourseVersion, len(modelsList))
	for i := range modelsList {
		result[i] = &CourseVersion{ID: modelsList[i].ID, CourseID: modelsList[i].CourseID, VersionNumber: modelsList[i].VersionNumber, Snapshot: []byte(modelsList[i].Snapshot), CreatedAt: modelsList[i].CreatedAt}
	}
	return result, nil
}

func (r *GormRepository) RestoreVersion(ctx context.Context, courseID string, versionNumber int, userID string) (*Course, error) {
	if _, err := parseUUID(userID); err != nil {
		return nil, err
	}
	modelCourse, err := r.repo.RestoreVersion(courseID, versionNumber, userID)
	if err != nil {
		return nil, err
	}
	return r.toDomainCourse(modelCourse), nil
}

func (r *GormRepository) GetChangelog(ctx context.Context, courseID string) ([]*CourseChangelog, error) {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	var logs []models.LmsCourseChangelog
	if err := r.repo.db.WithContext(ctx).Where("course_id = ?", courseUUID).
		Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, err
	}
	result := make([]*CourseChangelog, len(logs))
	for i := range logs {
		result[i] = &CourseChangelog{ID: logs[i].ID, CourseID: logs[i].CourseID, UserID: logs[i].UserID, Field: logs[i].Field, OldValue: logs[i].OldValue, NewValue: logs[i].NewValue, CreatedAt: logs[i].CreatedAt}
	}
	return result, nil
}
