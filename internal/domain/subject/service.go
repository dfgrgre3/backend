package subject

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrSubjectNotFound    = errors.New("subject not found")
	ErrSubjectExists      = errors.New("subject already exists")
	ErrInvalidInput       = errors.New("invalid input")
	ErrNotEnrolled        = errors.New("user is not enrolled in this course")
	ErrAlreadyEnrolled    = errors.New("user is already enrolled")
	ErrPaymentRequired    = errors.New("payment required for this course")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrLessonNotFound     = errors.New("lesson not found")
	ErrReviewExists       = errors.New("user already reviewed this course")
	ErrNotAuthorized      = errors.New("not authorized")
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

// ============================================================================
// Subject CRUD
// ============================================================================

func (s *Service) CreateSubject(ctx context.Context, input CreateSubjectInput) (*Subject, error) {
	if input.Name == "" {
		return nil, ErrInvalidInput
	}

	now := time.Now()
	subject := &Subject{
		Name:                   input.Name,
		NameAr:                 input.NameAr,
		Code:                   input.Code,
		Description:            input.Description,
		Icon:                   input.Icon,
		Color:                  input.Color,
		Type:                   input.Type,
		Level:                  input.Level,
		Slug:                   input.Slug,
		ThumbnailUrl:           input.ThumbnailUrl,
		TrailerUrl:             input.TrailerUrl,
		SeoTitle:               input.SeoTitle,
		SeoDescription:         input.SeoDescription,
		InstructorName:         input.InstructorName,
		InstructorId:           input.InstructorId,
		CategoryId:             input.CategoryId,
		Price:                  input.Price,
		IsFree:                 input.IsFree,
		IsPublished:            input.IsPublished,
		IsActive:               input.IsActive,
		IsFeatured:             input.IsFeatured,
		Language:               input.Language,
		Rating:                 0,
		EnrolledCount:          0,
		DurationHours:          input.DurationHours,
		TrailerDurationMinutes: input.TrailerDurationMinutes,
		Requirements:           input.Requirements,
		LearningObjectives:     input.LearningObjectives,
		CoursePrerequisites:    input.CoursePrerequisites,
		TargetAudience:         input.TargetAudience,
		WhatYouLearn:           input.WhatYouLearn,
		VideoCount:             0,
		CompletionRate:         0,
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	if err := s.repo.Create(ctx, subject); err != nil {
		return nil, fmt.Errorf("create subject: %w", err)
	}

	s.publishEvent(ctx, SubjectEvent{
		Type:      EventSubjectCreated,
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

func (s *Service) GetSubjectWithDetails(ctx context.Context, idOrSlug string, userID *string) (*SubjectDetail, error) {
	subject, err := s.GetSubject(ctx, idOrSlug)
	if err != nil {
		return nil, err
	}

	detail := &SubjectDetail{
		Subject: *subject,
	}

	if userID != nil && *userID != "" {
		// Check enrollment status
		enrollment, err := s.repo.GetEnrollment(ctx, *userID, subject.ID)
		if err == nil && enrollment != nil {
			detail.IsEnrolled = true
			detail.Progress = enrollment.Progress
			detail.EnrolledAt = &enrollment.EnrolledAt
		}

		// Calculate stats
		totalLessons := 0
		for _, topic := range subject.Topics {
			for range topic.SubTopics {
				totalLessons++
			}
		}
		detail.TotalLessons = totalLessons
		if detail.IsEnrolled && totalLessons > 0 {
			completed, err := s.repo.CountCompletedLessons(ctx, *userID, subject.ID)
			if err == nil {
				detail.CompletedLessons = completed
			}
		}
	}

	return detail, nil
}

func (s *Service) UpdateSubject(ctx context.Context, input UpdateSubjectInput) (*Subject, error) {
	subject, err := s.repo.FindByID(ctx, input.ID)
	if err != nil {
		return nil, ErrSubjectNotFound
	}

	applySubjectUpdates(subject, input)
	subject.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, subject); err != nil {
		return nil, fmt.Errorf("update subject: %w", err)
	}

	s.publishEvent(ctx, SubjectEvent{
		Type:      EventSubjectUpdated,
		SubjectID: subject.ID,
		Data: map[string]interface{}{
			"updatedFields": getUpdatedFields(input),
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
		Type:      EventSubjectDeleted,
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

	// Default: show only published and active for public
	if !filter.IncludeUnpublished {
		published := true
		filter.IsPublished = &published
	}

	return s.repo.List(ctx, filter)
}

// ============================================================================
// Curriculum Management
// ============================================================================

func (s *Service) UpdateCurriculum(ctx context.Context, subjectID string, curriculum CurriculumInput) error {
	_, err := s.repo.FindByID(ctx, subjectID)
	if err != nil {
		return ErrSubjectNotFound
	}

	if err := s.repo.UpdateCurriculum(ctx, subjectID, curriculum); err != nil {
		return fmt.Errorf("update curriculum: %w", err)
	}

	// Recalculate video count
	videoCount := 0
	for _, t := range curriculum.Topics {
		for _, st := range t.SubTopics {
			if st.Type == "VIDEO" {
				videoCount++
			}
		}
	}

	if err := s.repo.UpdateVideoCount(ctx, subjectID, videoCount); err != nil {
		return fmt.Errorf("update video count: %w", err)
	}

	s.publishEvent(ctx, SubjectEvent{
		Type:      EventCurriculumUpdated,
		SubjectID: subjectID,
		Data: map[string]interface{}{
			"chapters": len(curriculum.Topics),
		},
	})

	return nil
}

func (s *Service) GetCurriculum(ctx context.Context, subjectID string) (*Curriculum, error) {
	_, err := s.repo.FindByID(ctx, subjectID)
	if err != nil {
		return nil, ErrSubjectNotFound
	}

	topics, err := s.repo.GetCurriculum(ctx, subjectID)
	if err != nil {
		return nil, fmt.Errorf("get curriculum: %w", err)
	}

	chaptersCount := len(topics)
	lessonsCount := 0
	freeLessonsCount := 0
	totalDuration := 0

	for _, topic := range topics {
		lessonsCount += len(topic.SubTopics)
		for _, subtopic := range topic.SubTopics {
			if subtopic.IsFree {
				freeLessonsCount++
			}
			dur := subtopic.Duration
			if dur == 0 {
				dur = subtopic.DurationMin
			}
			totalDuration += dur
		}
	}

	return &Curriculum{
		Topics:             topics,
		ChaptersCount:      chaptersCount,
		LessonsCount:       lessonsCount,
		FreeLessonsCount:   freeLessonsCount,
		TotalDuration:      totalDuration,
	}, nil
}

// ============================================================================
// Enrollment
// ============================================================================

func (s *Service) EnrollUser(ctx context.Context, userID string, subjectID string) error {
	subject, err := s.repo.FindByID(ctx, subjectID)
	if err != nil {
		return ErrSubjectNotFound
	}

	// Check if already enrolled
	existing, err := s.repo.GetEnrollment(ctx, userID, subjectID)
	if err == nil && existing != nil {
		return ErrAlreadyEnrolled
	}

	// Check payment
	if subject.Price > 0 && !subject.IsFree {
		hasPaid, err := s.repo.HasUserPaid(ctx, userID, subjectID)
		if err != nil || !hasPaid {
			return ErrPaymentRequired
		}
	}

	if err := s.repo.CreateEnrollment(ctx, userID, subjectID); err != nil {
		return fmt.Errorf("create enrollment: %w", err)
	}

	s.publishEvent(ctx, SubjectEvent{
		Type:      EventUserEnrolled,
		SubjectID: subjectID,
		Data: map[string]interface{}{
			"userId": userID,
		},
	})

	return nil
}

func (s *Service) UnenrollUser(ctx context.Context, userID string, subjectID string) error {
	enrollment, err := s.repo.GetEnrollment(ctx, userID, subjectID)
	if err != nil || enrollment == nil {
		return ErrNotEnrolled
	}

	if err := s.repo.DeleteEnrollment(ctx, userID, subjectID); err != nil {
		return fmt.Errorf("delete enrollment: %w", err)
	}

	s.publishEvent(ctx, SubjectEvent{
		Type:      EventUserUnenrolled,
		SubjectID: subjectID,
		Data: map[string]interface{}{
			"userId": userID,
		},
	})

	return nil
}

func (s *Service) GetEnrollmentStatus(ctx context.Context, userID string, subjectID string) (*EnrollmentStatus, error) {
	subject, err := s.repo.FindByID(ctx, subjectID)
	if err != nil {
		return nil, ErrSubjectNotFound
	}

	status := &EnrollmentStatus{
		IsEnrolled:     false,
		PaymentRequired: subject.Price > 0 && !subject.IsFree,
		Price:          subject.Price,
	}

	enrollment, err := s.repo.GetEnrollment(ctx, userID, subjectID)
	if err == nil && enrollment != nil {
		status.IsEnrolled = true
		status.Progress = enrollment.Progress
		status.EnrolledAt = &enrollment.EnrolledAt

		// Get lesson counts
		total, completed, err := s.repo.GetLessonProgressCounts(ctx, userID, subjectID)
		if err == nil {
			status.TotalLessons = total
			status.CompletedLessons = completed
			if total > 0 && completed >= total {
				status.Status = "completed"
			} else if completed > 0 {
				status.Status = "in_progress"
			} else {
				status.Status = "enrolled"
			}
		}
	}

	return status, nil
}

func (s *Service) CompleteCourse(ctx context.Context, userID string, subjectID string) error {
	enrollment, err := s.repo.GetEnrollment(ctx, userID, subjectID)
	if err != nil || enrollment == nil {
		return ErrNotEnrolled
	}

	// Verify all lessons are completed
	total, completed, err := s.repo.GetLessonProgressCounts(ctx, userID, subjectID)
	if err != nil {
		return fmt.Errorf("check progress: %w", err)
	}

	if total > 0 && completed < total {
		return fmt.Errorf("must complete all lessons first: %d/%d", completed, total)
	}

	if err := s.repo.UpdateEnrollmentProgress(ctx, userID, subjectID, 100); err != nil {
		return fmt.Errorf("update progress: %w", err)
	}

	s.publishEvent(ctx, SubjectEvent{
		Type:      EventCourseCompleted,
		SubjectID: subjectID,
		Data: map[string]interface{}{
			"userId": userID,
		},
	})

	return nil
}

func (s *Service) GetUserCourses(ctx context.Context, userID string) ([]UserCourse, error) {
	enrollments, err := s.repo.GetUserEnrollments(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get enrollments: %w", err)
	}

	courses := make([]UserCourse, 0, len(enrollments))
	for _, enrollment := range enrollments {
		courses = append(courses, UserCourse{
			ID:           enrollment.ID,
			UserID:       enrollment.UserID,
			SubjectID:    enrollment.SubjectID,
			Progress:     enrollment.Progress,
			EnrolledAt:   enrollment.EnrolledAt,
			CreatedAt:    enrollment.CreatedAt,
			UpdatedAt:    enrollment.UpdatedAt,
			SubjectName:   enrollment.SubjectName,
			SubjectNameAr: enrollment.SubjectNameAr,
			ThumbnailUrl:  enrollment.ThumbnailUrl,
			Rating:        enrollment.Rating,
			Level:         enrollment.Level,
		})
	}

	return courses, nil
}

func (s *Service) GetCourseStudents(ctx context.Context, subjectID string, filter StudentsFilter) (StudentsResult, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	_, err := s.repo.FindByID(ctx, subjectID)
	if err != nil {
		return StudentsResult{}, ErrSubjectNotFound
	}

	return s.repo.ListStudents(ctx, subjectID, filter)
}

// ============================================================================
// Lesson Progress
// ============================================================================

func (s *Service) UpdateLessonProgress(ctx context.Context, userID string, lessonID string, input ProgressInput) error {
	// Verify enrollment (check if lesson belongs to an enrolled course)
	enrolled, err := s.repo.IsUserEnrolledInLessonCourse(ctx, userID, lessonID)
	if err != nil || !enrolled {
		return ErrNotEnrolled
	}

	progressStatus := input.Status
	if progressStatus == "" {
		if input.Completed {
			progressStatus = "COMPLETED"
		} else {
			progressStatus = "IN_PROGRESS"
		}
	}

	if err := s.repo.UpsertLessonProgress(ctx, userID, lessonID, LessonProgressData{
		Completed:           input.Completed,
		Status:              progressStatus,
		TimeSpentSeconds:    input.TimeSpentSeconds,
		LastWatchedPosition: int(input.LastWatchedPosition),
	}); err != nil {
		return fmt.Errorf("update progress: %w", err)
	}

	// Trigger progress recalculation
	if input.Completed {
		go func() {
			_ = s.recalculateProgress(context.Background(), userID, lessonID)
		}()
	}

	return nil
}

func (s *Service) recalculateProgress(ctx context.Context, userID string, lessonID string) error {
	subjectID, err := s.repo.GetSubjectIDByLesson(ctx, lessonID)
	if err != nil {
		return err
	}

	total, completed, err := s.repo.GetLessonProgressCounts(ctx, userID, subjectID)
	if err != nil {
		return err
	}

	var progress float64
	if total > 0 {
		progress = float64(completed) / float64(total) * 100
	}

	return s.repo.UpdateEnrollmentProgress(ctx, userID, subjectID, progress)
}

func (s *Service) GetLessonProgress(ctx context.Context, userID string, subjectID string) ([]LessonProgressInfo, error) {
	return s.repo.GetLessonProgressForSubject(ctx, userID, subjectID)
}

// ============================================================================
// Reviews
// ============================================================================

func (s *Service) CreateReview(ctx context.Context, userID string, subjectID string, input CreateReviewInput) (*Review, error) {
	// Verify enrollment
	enrolled, err := s.repo.GetEnrollment(ctx, userID, subjectID)
	if err != nil || enrolled == nil {
		return nil, ErrNotEnrolled
	}

	// Check if already reviewed
	existing, _ := s.repo.GetUserReview(ctx, userID, subjectID)
	if existing != nil {
		return nil, ErrReviewExists
	}

	review := &Review{
		SubjectID: subjectID,
		UserID:    userID,
		Rating:    input.Rating,
		Comment:   input.Comment,
		IsVisible: true,
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateReview(ctx, review); err != nil {
		return nil, fmt.Errorf("create review: %w", err)
	}

	// Update average rating
	if err := s.repo.UpdateSubjectRating(ctx, subjectID); err != nil {
		return nil, fmt.Errorf("update rating: %w", err)
	}

	s.publishEvent(ctx, SubjectEvent{
		Type:      EventReviewCreated,
		SubjectID: subjectID,
		Data: map[string]interface{}{
			"userId": userID,
			"rating": input.Rating,
		},
	})

	return review, nil
}

func (s *Service) GetReviews(ctx context.Context, subjectID string) ([]Review, error) {
	_, err := s.repo.FindByID(ctx, subjectID)
	if err != nil {
		return nil, ErrSubjectNotFound
	}

	return s.repo.GetReviews(ctx, subjectID)
}

func (s *Service) GetReviewStats(ctx context.Context, subjectID string) (*ReviewStats, error) {
	return s.repo.GetReviewStats(ctx, subjectID)
}

// ============================================================================
// Dashboard / Stats
// ============================================================================

func (s *Service) GetDashboardStats(ctx context.Context) (map[string]interface{}, error) {
	total, err := s.repo.CountTotal(ctx)
	if err != nil {
		return nil, fmt.Errorf("count subjects: %w", err)
	}

	var published int64
	var active int64
	publishedTrue := true
	activeTrue := true
	publishedFilter := ListSubjectsFilter{IsPublished: &publishedTrue}
	activeFilter := ListSubjectsFilter{IsActive: &activeTrue}

	pubResult, _ := s.repo.List(ctx, publishedFilter)
	actResult, _ := s.repo.List(ctx, activeFilter)

	if pubResult.Total > 0 {
		published = pubResult.Total
	}
	if actResult.Total > 0 {
		active = actResult.Total
	}

	// Get top enrolled
	topFilter := ListSubjectsFilter{
		SortBy:    "enrolled_count",
		SortOrder: "DESC",
		Limit:     5,
	}
	topResult, _ := s.repo.List(ctx, topFilter)

	return map[string]interface{}{
		"totalSubjects":  total,
		"publishedCount": published,
		"activeCount":    active,
		"topEnrolled":    topResult.Subjects,
	}, nil
}

// ============================================================================
// Admin Operations
// ============================================================================

func (s *Service) DuplicateSubject(ctx context.Context, subjectID string) (*Subject, error) {
	original, err := s.repo.FindByID(ctx, subjectID)
	if err != nil {
		return nil, ErrSubjectNotFound
	}

	// Create copy with unique identifiers
	suffix := fmt.Sprintf("-%d", time.Now().Unix()%10000)
	nameCopy := original.Name + " (Copy)"

	var nameArCopy *string
	if original.NameAr != nil {
		val := *original.NameAr + " (نسخة)"
		nameArCopy = &val
	}

	var codeCopy *string
	if original.Code != nil {
		val := *original.Code + suffix
		codeCopy = &val
	}

	var slugCopy *string
	if original.Slug != nil {
		val := *original.Slug + suffix
		slugCopy = &val
	}

	duplicate := &Subject{
		Name:                   nameCopy,
		NameAr:                 nameArCopy,
		Code:                   codeCopy,
		Slug:                   slugCopy,
		Description:            original.Description,
		Icon:                   original.Icon,
		Color:                  original.Color,
		Type:                   original.Type,
		Level:                  original.Level,
		ThumbnailUrl:           original.ThumbnailUrl,
		TrailerUrl:             original.TrailerUrl,
		SeoTitle:               original.SeoTitle,
		SeoDescription:         original.SeoDescription,
		InstructorName:         original.InstructorName,
		InstructorId:           original.InstructorId,
		CategoryId:             original.CategoryId,
		Price:                  original.Price,
		IsFree:                 original.IsFree,
		IsPublished:            false,
		IsActive:               true,
		IsFeatured:             false,
		Language:               original.Language,
		DurationHours:          original.DurationHours,
		TrailerDurationMinutes: original.TrailerDurationMinutes,
		Requirements:           original.Requirements,
		LearningObjectives:     original.LearningObjectives,
		CoursePrerequisites:    original.CoursePrerequisites,
		TargetAudience:         original.TargetAudience,
		WhatYouLearn:           original.WhatYouLearn,
		Rating:                 0,
		EnrolledCount:          0,
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}

	if err := s.repo.Create(ctx, duplicate); err != nil {
		return nil, fmt.Errorf("duplicate subject: %w", err)
	}

	// Duplicate curriculum
	topics, err := s.repo.GetCurriculum(ctx, subjectID)
	if err == nil && len(topics) > 0 {
		curriculumInput := CurriculumInput{
			Topics: make([]TopicInput, len(topics)),
		}
		for i, topic := range topics {
			curriculumInput.Topics[i] = TopicInput{
				Title:     topic.Title,
				Order:     topic.Order,
				SubTopics: make([]SubTopicInput, len(topic.SubTopics)),
			}
			for j, st := range topic.SubTopics {
				curriculumInput.Topics[i].SubTopics[j] = SubTopicInput{
					Title:       st.Title,
					Type:        st.Type,
					Order:       st.Order,
					IsFree:      st.IsFree,
					VideoUrl:    st.VideoUrl,
					Duration:    st.Duration,
					DurationMin: st.DurationMin,
					Description: st.Description,
				}
			}
		}
		_ = s.repo.UpdateCurriculum(ctx, duplicate.ID, curriculumInput)
	}

	s.publishEvent(ctx, SubjectEvent{
		Type:      EventSubjectDuplicated,
		SubjectID: duplicate.ID,
		Data: map[string]interface{}{
			"originalId": subjectID,
		},
	})

	return duplicate, nil
}

func (s *Service) BatchAction(ctx context.Context, ids []string, action string) error {
	if len(ids) == 0 {
		return nil
	}

	switch action {
	case "publish":
		return s.repo.BatchUpdate(ctx, ids, map[string]interface{}{"isPublished": true})
	case "unpublish":
		return s.repo.BatchUpdate(ctx, ids, map[string]interface{}{"isPublished": false})
	case "activate":
		return s.repo.BatchUpdate(ctx, ids, map[string]interface{}{"isActive": true})
	case "deactivate":
		return s.repo.BatchUpdate(ctx, ids, map[string]interface{}{"isActive": false})
	case "delete":
		return s.repo.BatchDelete(ctx, ids)
	default:
		return ErrInvalidInput
	}
}

// ============================================================================
// Event Publishing
// ============================================================================

func (s *Service) publishEvent(ctx context.Context, event SubjectEvent) {
	event.Timestamp = time.Now()
	if s.publisher != nil {
		_ = s.publisher.Publish(ctx, event)
	}
}

// ============================================================================
// Helpers
// ============================================================================

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
	assignIfPresent(&subject.Code, input.Code)

	if input.DurationHours != nil {
		subject.DurationHours = input.DurationHours
	}
	if input.TrailerDurationMinutes != nil {
		subject.TrailerDurationMinutes = input.TrailerDurationMinutes
	}
	if input.VideoCount != nil {
		subject.VideoCount = *input.VideoCount
	}

	assignSliceIfPresent(&subject.Requirements, input.Requirements)
	assignSliceIfPresent(&subject.LearningObjectives, input.LearningObjectives)
	assignSliceIfPresent(&subject.CoursePrerequisites, input.CoursePrerequisites)
	assignSliceIfPresent(&subject.TargetAudience, input.TargetAudience)
	assignSliceIfPresent(&subject.WhatYouLearn, input.WhatYouLearn)
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

func getUpdatedFields(input UpdateSubjectInput) []string {
	var fields []string
	if input.Name != nil {
		fields = append(fields, "name")
	}
	if input.NameAr != nil {
		fields = append(fields, "nameAr")
	}
	if input.Description != nil {
		fields = append(fields, "description")
	}
	if input.Price != nil {
		fields = append(fields, "price")
	}
	if input.IsPublished != nil {
		fields = append(fields, "isPublished")
	}
	if input.IsActive != nil {
		fields = append(fields, "isActive")
	}
	if input.Level != nil {
		fields = append(fields, "level")
	}
	return fields
}