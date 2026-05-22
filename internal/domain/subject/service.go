package subject

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrSubjectNotFound = errors.New("subject not found")
	ErrSubjectExists   = errors.New("subject already exists")
	ErrInvalidInput    = errors.New("invalid input")
)

type Service struct {
	repo      Repository
	publisher EventPublisher
}

func NewService(repo Repository, publisher EventPublisher) *Service {
	return &Service{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *Service) CreateSubject(ctx context.Context, input CreateSubjectInput) (*Subject, error) {
	if input.Name == "" {
		return nil, ErrInvalidInput
	}

	subject := &Subject{
		Name:           input.Name,
		NameAr:         input.NameAr,
		Description:    input.Description,
		Icon:           input.Icon,
		Color:          input.Color,
		Type:           input.Type,
		Level:          input.Level,
		Slug:           input.Slug,
		ThumbnailUrl:   input.ThumbnailUrl,
		TrailerUrl:     input.TrailerUrl,
		InstructorName: input.InstructorName,
		InstructorId:   input.InstructorId,
		CategoryId:     input.CategoryId,
		Price:          input.Price,
		IsFree:         input.IsFree,
		IsPublished:    input.IsPublished,
		IsActive:       input.IsActive,
		Language:       input.Language,
		Rating:         0,
		EnrolledCount:  0,
	}

	if err := s.repo.Create(ctx, subject); err != nil {
		return nil, fmt.Errorf("create subject: %w", err)
	}

	s.publishEvent(ctx, SubjectEvent{
		Type:      "subject.created",
		SubjectID: subject.ID,
		Data: map[string]interface{}{
			"name": subject.Name,
			"type": subject.Type,
		},
	})

	return subject, nil
}

func (s *Service) GetSubject(ctx context.Context, idOrSlug string) (*Subject, error) {
	subject, err := s.repo.FindByID(ctx, idOrSlug)
	if err != nil {
		subject, err = s.repo.FindBySlug(ctx, idOrSlug)
		if err != nil {
			return nil, ErrSubjectNotFound
		}
	}
	return subject, nil
}

func (s *Service) UpdateSubject(ctx context.Context, input UpdateSubjectInput) (*Subject, error) {
	subject, err := s.repo.FindByID(ctx, input.ID)
	if err != nil {
		return nil, ErrSubjectNotFound
	}

	applySubjectUpdates(subject, input)

	if err := s.repo.Update(ctx, subject); err != nil {
		return nil, fmt.Errorf("update subject: %w", err)
	}

	s.publishEvent(ctx, SubjectEvent{
		Type:      "subject.updated",
		SubjectID: subject.ID,
		Data: map[string]interface{}{
			"fields": []string{"name", "description", "price", "isPublished"},
		},
	})

	return subject, nil
}

func (s *Service) DeleteSubject(ctx context.Context, id string) error {
	_, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return ErrSubjectNotFound
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete subject: %w", err)
	}

	s.publishEvent(ctx, SubjectEvent{
		Type:      "subject.deleted",
		SubjectID: id,
	})

	return nil
}

func (s *Service) ListSubjects(ctx context.Context, filter ListSubjectsFilter) (ListSubjectsResult, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	return s.repo.List(ctx, filter)
}

func (s *Service) UpdateCurriculum(ctx context.Context, subjectID string, curriculum CurriculumInput) error {
	_, err := s.repo.FindByID(ctx, subjectID)
	if err != nil {
		return ErrSubjectNotFound
	}

	if err := s.repo.UpdateCurriculum(ctx, subjectID, curriculum); err != nil {
		return fmt.Errorf("update curriculum: %w", err)
	}

	s.publishEvent(ctx, SubjectEvent{
		Type:      "subject.curriculum_updated",
		SubjectID: subjectID,
		Data: map[string]interface{}{
			"topics_count": len(curriculum.Topics),
		},
	})

	return nil
}

func (s *Service) GetCurriculum(ctx context.Context, subjectID string) ([]Topic, error) {
	_, err := s.repo.FindByID(ctx, subjectID)
	if err != nil {
		return nil, ErrSubjectNotFound
	}

	return s.repo.GetCurriculum(ctx, subjectID)
}

func (s *Service) GetDashboardStats(ctx context.Context) (map[string]interface{}, error) {
	total, err := s.repo.CountTotal(ctx)
	if err != nil {
		return nil, fmt.Errorf("count subjects: %w", err)
	}

	return map[string]interface{}{
		"totalSubjects": total,
	}, nil
}

func (s *Service) publishEvent(ctx context.Context, event SubjectEvent) {
	event.Timestamp = time.Now()
	if s.publisher != nil {
		_ = s.publisher.Publish(ctx, event)
	}
}

func applySubjectUpdates(subject *Subject, input UpdateSubjectInput) {
	assignValueIfPresent(&subject.Name, input.Name)
	assignValueIfPresent(&subject.Type, input.Type)
	assignValueIfPresent(&subject.Price, input.Price)
	assignValueIfPresent(&subject.IsFree, input.IsFree)
	assignValueIfPresent(&subject.IsPublished, input.IsPublished)
	assignValueIfPresent(&subject.IsActive, input.IsActive)
	assignValueIfPresent(&subject.IsFeatured, input.IsFeatured)

	assignIfPresent(&subject.NameAr, input.NameAr)
	assignIfPresent(&subject.Description, input.Description)
	assignIfPresent(&subject.Icon, input.Icon)
	assignIfPresent(&subject.Color, input.Color)
	assignIfPresent(&subject.Level, input.Level)
	assignIfPresent(&subject.Slug, input.Slug)
	assignIfPresent(&subject.ThumbnailUrl, input.ThumbnailUrl)
	assignIfPresent(&subject.TrailerUrl, input.TrailerUrl)
	assignIfPresent(&subject.SeoTitle, input.SeoTitle)
	assignIfPresent(&subject.SeoDescription, input.SeoDescription)
	assignIfPresent(&subject.InstructorName, input.InstructorName)
	assignIfPresent(&subject.InstructorId, input.InstructorId)
	assignIfPresent(&subject.CategoryId, input.CategoryId)
	assignIfPresent(&subject.Language, input.Language)
	assignIfPresent(&subject.DurationHours, input.DurationHours)
	assignIfPresent(&subject.TrailerDurationMinutes, input.TrailerDurationMinutes)

	assignSliceIfPresent(&subject.Requirements, input.Requirements)
	assignSliceIfPresent(&subject.LearningObjectives, input.LearningObjectives)
}

func assignValueIfPresent[T any](target *T, value *T) {
	if value != nil {
		*target = *value
	}
}

func assignIfPresent[T any](target **T, value *T) {
	if value != nil {
		*target = value
	}
}

func assignSliceIfPresent[T any](target *[]T, value []T) {
	if value != nil {
		*target = value
	}
}
