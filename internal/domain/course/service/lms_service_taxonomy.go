package courseservice

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
)

// ----------------------------
// Categories & Tags
// ----------------------------

func (s *LmsService) CreateCategory(name, slug string, parentID *uuid.UUID) (*models.LmsCategory, error) {
	c := &models.LmsCategory{
		Name:     name,
		Slug:     slug,
		ParentID: parentID,
	}
	if err := s.repo.CreateCategory(c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *LmsService) ListCategories() ([]models.LmsCategory, error) {
	return s.repo.ListCategories()
}

func (s *LmsService) CreateTag(name string) (*models.LmsTag, error) {
	t := &models.LmsTag{Name: name}
	if err := s.repo.CreateTag(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *LmsService) ListTags() ([]models.LmsTag, error) {
	return s.repo.ListTags()
}

func (s *LmsService) SetCourseCategories(courseID uuid.UUID, categoryIDs []uuid.UUID) error {
	return s.repo.SetCourseCategories(courseID, categoryIDs)
}

func (s *LmsService) SetCourseTags(courseID uuid.UUID, tagIDs []uuid.UUID) error {
	return s.repo.SetCourseTags(courseID, tagIDs)
}
