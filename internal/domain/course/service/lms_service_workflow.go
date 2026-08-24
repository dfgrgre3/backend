package courseservice

import (
	"errors"
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
)

// ----------------------------
// Workflow
// ----------------------------

// SubmitForReview transitions a course from draft to pending_review.
func (s *LmsService) SubmitForReview(courseID uuid.UUID) error {
	course, err := s.repo.GetCourseByID(courseID)
	if err != nil {
		return err
	}
	if course.Status != models.CourseStatusDraft {
		return errors.New("course must be in draft state")
	}
	if err := s.repo.UpdateCourseStatus(courseID, models.CourseStatusUnderReview); err != nil {
		return err
	}
	s.addChangelog(courseID, uuid.Nil, "status", string(course.Status), string(models.CourseStatusUnderReview))
	return nil
}

// ApproveCourse transitions a course from pending_review to published.
func (s *LmsService) ApproveCourse(courseID, reviewerID uuid.UUID, notes string) error {
	course, err := s.repo.GetCourseByID(courseID)
	if err != nil {
		return err
	}
	if course.Status != models.CourseStatusUnderReview {
		return errors.New("course must be in pending_review state")
	}
	if err := s.repo.UpdateCourseStatus(courseID, models.CourseStatusPublished); err != nil {
		return err
	}
	s.addChangelog(courseID, reviewerID, "status", string(course.Status), string(models.CourseStatusPublished))
	return nil
}

// RejectCourse transitions a course from pending_review back to draft.
func (s *LmsService) RejectCourse(courseID, reviewerID uuid.UUID, reason string) error {
	course, err := s.repo.GetCourseByID(courseID)
	if err != nil {
		return err
	}
	if course.Status != models.CourseStatusUnderReview {
		return errors.New("course must be in pending_review state")
	}
	if err := s.repo.UpdateCourseStatus(courseID, models.CourseStatusDraft); err != nil {
		return err
	}
	s.addChangelog(courseID, reviewerID, "status", string(course.Status), string(models.CourseStatusDraft))
	return nil
}

// ArchiveCourse transitions a published course to archived.
func (s *LmsService) ArchiveCourse(courseID, userID uuid.UUID) error {
	course, err := s.repo.GetCourseByID(courseID)
	if err != nil {
		return err
	}
	if course.Status != models.CourseStatusPublished {
		return errors.New("course must be published")
	}
	if err := s.repo.UpdateCourseStatus(courseID, models.CourseStatusArchived); err != nil {
		return err
	}
	s.addChangelog(courseID, userID, "status", string(course.Status), string(models.CourseStatusArchived))
	return nil
}

// UnarchiveCourse transitions an archived course back to draft.
func (s *LmsService) UnarchiveCourse(courseID uuid.UUID) error {
	course, err := s.repo.GetCourseByID(courseID)
	if err != nil {
		return err
	}
	if course.Status != models.CourseStatusArchived {
		return errors.New("course must be archived")
	}
	if err := s.repo.UpdateCourseStatus(courseID, models.CourseStatusDraft); err != nil {
		return err
	}
	s.addChangelog(courseID, uuid.Nil, "status", string(course.Status), string(models.CourseStatusDraft))
	return nil
}
