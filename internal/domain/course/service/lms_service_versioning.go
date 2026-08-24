package courseservice

import (
	"encoding/json"
	"fmt"
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
)

// ----------------------------
// Changelog & Versions
// ----------------------------

func (s *LmsService) ListChangelogs(courseID uuid.UUID) ([]models.LmsCourseChangelog, error) {
	return s.repo.ListChangelogs(courseID)
}

func (s *LmsService) ListVersions(courseID uuid.UUID) ([]models.LmsCourseVersion, error) {
	return s.repo.ListVersions(courseID)
}

// CreateVersion snapshots the current course state as a new version, using
// the next sequential version number for this course (existing versions'
// max + 1), so every snapshot gets a distinct, orderable number instead of
// every call minting a duplicate "version 1".
func (s *LmsService) CreateVersion(courseID uuid.UUID) (*models.LmsCourseVersion, error) {
	existing, err := s.repo.ListVersions(courseID)
	if err != nil {
		return nil, err
	}
	nextVersion := 1
	for _, v := range existing {
		if v.VersionNumber >= nextVersion {
			nextVersion = v.VersionNumber + 1
		}
	}

	snapshot, err := s.repo.SnapshotCourse(courseID)
	if err != nil {
		return nil, err
	}
	v := &models.LmsCourseVersion{
		CourseID:      courseID,
		VersionNumber: nextVersion,
		Snapshot:      snapshot,
	}
	if err := s.repo.CreateVersion(v); err != nil {
		return nil, err
	}
	return v, nil
}

// RestoreVersion overwrites a course's top-level (scalar) fields with the
// values captured in a prior version snapshot. Nested associations
// (sections/lessons/pricing/instructors) are intentionally left untouched —
// restoring those safely requires a transactional replace of every nested
// row, which is out of scope until this endpoint has a real caller; doing a
// partial job silently would be worse than restoring the fields that are
// safe to overwrite in place.
func (s *LmsService) RestoreVersion(courseID uuid.UUID, versionNumber int) (*models.LmsCourse, error) {
	version, err := s.repo.GetVersion(courseID, versionNumber)
	if err != nil {
		return nil, err
	}

	var snapshot models.LmsCourse
	if err := json.Unmarshal(version.Snapshot, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to parse version snapshot: %w", err)
	}

	current, err := s.repo.GetCourseByID(courseID)
	if err != nil {
		return nil, err
	}

	current.Title = snapshot.Title
	current.Slug = snapshot.Slug
	current.ShortDescription = snapshot.ShortDescription
	current.LongDescription = snapshot.LongDescription
	current.CoverImageURL = snapshot.CoverImageURL
	current.PromoVideoURL = snapshot.PromoVideoURL
	current.Status = snapshot.Status
	current.Level = snapshot.Level
	current.Language = snapshot.Language
	current.EstimatedDurationMins = snapshot.EstimatedDurationMins
	current.HasCertificate = snapshot.HasCertificate
	current.CertificateTemplate = snapshot.CertificateTemplate
	current.MaxStudents = snapshot.MaxStudents
	current.IsFeatured = snapshot.IsFeatured
	current.IsTrending = snapshot.IsTrending
	current.IsNew = snapshot.IsNew
	current.NewFrom = snapshot.NewFrom
	current.NewUntil = snapshot.NewUntil
	current.SEOTitle = snapshot.SEOTitle
	current.SEODescription = snapshot.SEODescription
	current.SEOKeywords = snapshot.SEOKeywords
	current.PrerequisitesText = snapshot.PrerequisitesText
	current.TargetAudience = snapshot.TargetAudience
	current.LearningOutcomes = snapshot.LearningOutcomes
	current.PrimaryInstructorID = snapshot.PrimaryInstructorID
	current.AvailableFrom = snapshot.AvailableFrom
	current.AvailableUntil = snapshot.AvailableUntil

	// Prevent GORM's default association-save behavior from touching nested
	// rows — Save() only writes columns on the LmsCourse row itself.
	current.Sections = nil
	current.Pricings = nil
	current.Instructors = nil
	current.AvailabilityWindows = nil

	if err := s.repo.UpdateCourse(current); err != nil {
		return nil, err
	}
	return current, nil
}

// CloneCourse duplicates a course into a new draft.
func (s *LmsService) CloneCourse(srcID uuid.UUID, newTitle string) (*models.LmsCourse, error) {
	return s.repo.CloneCourse(srcID, newTitle)
}

// ----------------------------
// Helpers
// ----------------------------

func (s *LmsService) addChangelog(courseID, userID uuid.UUID, field, oldVal, newVal string) {
	oldPtr := &oldVal
	newPtr := &newVal
	entry := &models.LmsCourseChangelog{
		CourseID: courseID,
		UserID:   userID,
		Field:    field,
		OldValue: oldPtr,
		NewValue: newPtr,
	}
	_ = s.repo.AddChangelog(entry)
}
