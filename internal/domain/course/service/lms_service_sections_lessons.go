package courseservice

import (
	"errors"
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
)

// ErrLessonSectionMismatch is returned when a lesson-scoped operation is
// called with a sectionID that does not actually own the lesson — e.g. a
// stale/cross-chapter link request.
var ErrLessonSectionMismatch = errors.New("lesson does not belong to the given section")

// ----------------------------
// Sections & Lessons
// ----------------------------

func (s *LmsService) CreateSection(courseID uuid.UUID, title string, orderIndex int) (*models.LmsSection, error) {
	section := &models.LmsSection{
		CourseID:   courseID,
		Title:      title,
		OrderIndex: orderIndex,
	}
	if err := s.repo.CreateSection(section); err != nil {
		return nil, err
	}
	return section, nil
}

func (s *LmsService) GetSection(id uuid.UUID) (*models.LmsSection, error) {
	return s.repo.GetSectionByID(id)
}

// UpdateSection persists changes to an existing section. Callers must load
// the current row first (GetSection) and mutate only the fields that
// changed, since this does a full Save.
func (s *LmsService) UpdateSection(section *models.LmsSection) (*models.LmsSection, error) {
	if err := s.repo.UpdateSection(section); err != nil {
		return nil, err
	}
	return section, nil
}

func (s *LmsService) DeleteSection(id uuid.UUID) error {
	return s.repo.DeleteSection(id)
}

func (s *LmsService) ListSections(courseID uuid.UUID) ([]models.LmsSection, error) {
	return s.repo.ListSectionsByCourseID(courseID)
}

func (s *LmsService) ReorderSections(courseID uuid.UUID, sectionIDs []uuid.UUID) error {
	return s.repo.ReorderSections(courseID, sectionIDs)
}

func (s *LmsService) CreateLesson(sectionID uuid.UUID, title string, lessonType models.LessonType, orderIndex int) (*models.LmsLesson, error) {
	lesson := &models.LmsLesson{
		SectionID:  sectionID,
		Title:      title,
		Type:       lessonType,
		OrderIndex: orderIndex,
	}
	if err := s.repo.CreateLesson(lesson); err != nil {
		return nil, err
	}
	return lesson, nil
}

func (s *LmsService) GetLesson(id uuid.UUID) (*models.LmsLesson, error) {
	return s.repo.GetLessonByID(id)
}

// UpdateLesson persists changes to an existing lesson. Callers must load the
// current row first (GetLesson) and mutate only the fields that changed,
// since this does a full Save.
func (s *LmsService) UpdateLesson(lesson *models.LmsLesson) (*models.LmsLesson, error) {
	if err := s.repo.UpdateLesson(lesson); err != nil {
		return nil, err
	}
	return lesson, nil
}

func (s *LmsService) DeleteLesson(id uuid.UUID) error {
	return s.repo.DeleteLesson(id)
}

func (s *LmsService) ListLessons(sectionID uuid.UUID) ([]models.LmsLesson, error) {
	return s.repo.ListLessonsBySectionID(sectionID)
}

func (s *LmsService) ReorderLessons(sectionID uuid.UUID, lessonIDs []uuid.UUID) error {
	return s.repo.ReorderLessons(sectionID, lessonIDs)
}

// LinkExam associates a lesson with an exam (one exam per lesson).
// sectionID must be the lesson's actual section — see ErrLessonSectionMismatch.
func (s *LmsService) LinkExam(sectionID, lessonID uuid.UUID, examID string) (*models.LmsLesson, error) {
	lesson, err := s.repo.GetLessonByID(lessonID)
	if err != nil {
		return nil, err
	}
	if lesson.SectionID != sectionID {
		return nil, ErrLessonSectionMismatch
	}
	if err := s.repo.LinkExamToLesson(lessonID, &examID); err != nil {
		return nil, err
	}
	return s.repo.GetLessonByID(lessonID)
}

// UnlinkExam removes a lesson's exam link, if any.
// sectionID must be the lesson's actual section — see ErrLessonSectionMismatch.
func (s *LmsService) UnlinkExam(sectionID, lessonID uuid.UUID) error {
	lesson, err := s.repo.GetLessonByID(lessonID)
	if err != nil {
		return err
	}
	if lesson.SectionID != sectionID {
		return ErrLessonSectionMismatch
	}
	return s.repo.LinkExamToLesson(lessonID, nil)
}
