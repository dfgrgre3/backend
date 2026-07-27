package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"thanawy-backend/internal/cache"
	"thanawy-backend/internal/domain/subject"

	"github.com/redis/go-redis/v9"
)

// SubjectCachedRepository wraps the base subject repository with Redis caching.
type SubjectCachedRepository struct {
	repo     subject.Repository
	redis    *redis.Client
	invalid  *cache.CacheInvalidator
}

// NewSubjectCachedRepository creates a new caching wrapper around SubjectRepository.
func NewSubjectCachedRepository(repo subject.Repository, redisClient *redis.Client, invalidator *cache.CacheInvalidator) *SubjectCachedRepository {
	return &SubjectCachedRepository{
		repo:    repo,
		redis:   redisClient,
		invalid: invalidator,
	}
}

// ============================================================================
// Cache helpers
// ============================================================================

func (r *SubjectCachedRepository) cacheKey(entity string, id string) string {
	return fmt.Sprintf("subject:%s:%s", entity, id)
}

func (r *SubjectCachedRepository) listCacheKey(filter subject.ListSubjectsFilter) string {
	data, _ := json.Marshal(filter)
	return fmt.Sprintf("subject:list:%x", data)
}

func (r *SubjectCachedRepository) getFromCache(ctx context.Context, key string, dest interface{}) bool {
	if r.redis == nil {
		return false
	}
	val, err := r.redis.Get(ctx, key).Bytes()
	if err != nil {
		return false
	}
	if err := json.Unmarshal(val, dest); err != nil {
		log.Printf("[Cache] Unmarshal error for key %s: %v", key, err)
		return false
	}
	return true
}

func (r *SubjectCachedRepository) setCache(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	if r.redis == nil {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		log.Printf("[Cache] Marshal error for key %s: %v", key, err)
		return
	}
	if err := r.redis.Set(ctx, key, data, ttl).Err(); err != nil {
		log.Printf("[Cache] Set error for key %s: %v", key, err)
	}
}

// ============================================================================
// Subject CRUD (with caching)
// ============================================================================

func (r *SubjectCachedRepository) Create(ctx context.Context, s *subject.Subject) error {
	if err := r.repo.Create(ctx, s); err != nil {
		return err
	}
	// Cache the new subject
	r.setCache(ctx, r.cacheKey("id", s.ID), s, cache.TTLSubjectDetail)
	if s.Slug != nil && *s.Slug != "" {
		r.setCache(ctx, r.cacheKey("slug", *s.Slug), s, cache.TTLSubjectDetail)
	}
	r.invalid.InvalidateAllLists(ctx)
	return nil
}

func (r *SubjectCachedRepository) FindByID(ctx context.Context, id string) (*subject.Subject, error) {
	// Try cache first
	var cached subject.Subject
	if r.getFromCache(ctx, r.cacheKey("id", id), &cached) {
		return &cached, nil
	}

	// Fallback to DB
	s, err := r.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Populate cache (async-safe, ignore errors)
	r.setCache(ctx, r.cacheKey("id", id), s, cache.TTLSubjectDetail)
	if s.Slug != nil && *s.Slug != "" {
		r.setCache(ctx, r.cacheKey("slug", *s.Slug), s, cache.TTLSubjectDetail)
	}

	return s, nil
}

func (r *SubjectCachedRepository) FindBySlug(ctx context.Context, slug string) (*subject.Subject, error) {
	// Try cache first
	var cached subject.Subject
	if r.getFromCache(ctx, r.cacheKey("slug", slug), &cached) {
		return &cached, nil
	}

	// Fallback to DB
	s, err := r.repo.FindBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	// Populate cache
	r.setCache(ctx, r.cacheKey("slug", slug), s, cache.TTLSubjectDetail)
	r.setCache(ctx, r.cacheKey("id", s.ID), s, cache.TTLSubjectDetail)

	return s, nil
}

func (r *SubjectCachedRepository) Update(ctx context.Context, s *subject.Subject) error {
	if err := r.repo.Update(ctx, s); err != nil {
		return err
	}

	// Invalidate cache
	r.invalid.InvalidateSubject(ctx, s.ID)
	r.setCache(ctx, r.cacheKey("id", s.ID), s, cache.TTLSubjectDetail)
	if s.Slug != nil && *s.Slug != "" {
		r.setCache(ctx, r.cacheKey("slug", *s.Slug), s, cache.TTLSubjectDetail)
	}
	r.invalid.InvalidateAllLists(ctx)
	return nil
}

func (r *SubjectCachedRepository) Delete(ctx context.Context, id string) error {
	if err := r.repo.Delete(ctx, id); err != nil {
		return err
	}

	r.invalid.InvalidateSubject(ctx, id)
	r.invalid.InvalidateAllLists(ctx)
	return nil
}

func (r *SubjectCachedRepository) List(ctx context.Context, filter subject.ListSubjectsFilter) (subject.ListSubjectsResult, error) {
	// Only cache default/public list requests (page 1, no search, no special filters)
	if filter.Search != nil || filter.Page > 1 || filter.Limit != 20 {
		return r.repo.List(ctx, filter)
	}

	cacheKey := r.listCacheKey(filter)
	var cached subject.ListSubjectsResult
	if r.getFromCache(ctx, cacheKey, &cached) {
		return cached, nil
	}

	result, err := r.repo.List(ctx, filter)
	if err != nil {
		return result, err
	}

	r.setCache(ctx, cacheKey, result, cache.CacheTTLList)
	return result, nil
}

// ============================================================================
// Curriculum (thin caching - always read from DB for consistency)
// ============================================================================

func (r *SubjectCachedRepository) UpdateCurriculum(ctx context.Context, subjectID string, curriculum subject.CurriculumInput) error {
	err := r.repo.UpdateCurriculum(ctx, subjectID, curriculum)
	if err == nil {
		r.invalid.InvalidateSubject(ctx, subjectID)
	}
	return err
}

func (r *SubjectCachedRepository) GetCurriculum(ctx context.Context, subjectID string) ([]subject.Topic, error) {
	// Curriculum changes infrequently, cache for a short time
	cacheKey := r.cacheKey("curriculum", subjectID)
	var cached []subject.Topic
	if r.getFromCache(ctx, cacheKey, &cached) {
		return cached, nil
	}

	topics, err := r.repo.GetCurriculum(ctx, subjectID)
	if err != nil {
		return nil, err
	}

	r.setCache(ctx, cacheKey, topics, 15*time.Minute)
	return topics, nil
}

func (r *SubjectCachedRepository) UpdateVideoCount(ctx context.Context, subjectID string, count int) error {
	return r.repo.UpdateVideoCount(ctx, subjectID, count)
}

// ============================================================================
// Stats / Counts (short cache)
// ============================================================================

func (r *SubjectCachedRepository) CountByCategory(ctx context.Context, categoryID string) (int64, error) {
	cacheKey := r.cacheKey("count_category", categoryID)
	var cached int64
	if r.getFromCache(ctx, cacheKey, &cached) {
		return cached, nil
	}

	count, err := r.repo.CountByCategory(ctx, categoryID)
	if err != nil {
		return 0, err
	}

	r.setCache(ctx, cacheKey, count, cache.CacheTTLCategory)
	return count, nil
}

func (r *SubjectCachedRepository) CountTotal(ctx context.Context) (int64, error) {
	cacheKey := r.cacheKey("count_total", "global")
	var cached int64
	if r.getFromCache(ctx, cacheKey, &cached) {
		return cached, nil
	}

	count, err := r.repo.CountTotal(ctx)
	if err != nil {
		return 0, err
	}

	r.setCache(ctx, cacheKey, count, cache.CacheTTLCategory)
	return count, nil
}

func (r *SubjectCachedRepository) CountCompletedLessons(ctx context.Context, userID string, subjectID string) (int, error) {
	// User-specific data: short cache
	cacheKey := r.cacheKey("completed", fmt.Sprintf("%s:%s", userID, subjectID))
	var cached int
	if r.getFromCache(ctx, cacheKey, &cached) {
		return cached, nil
	}

	count, err := r.repo.CountCompletedLessons(ctx, userID, subjectID)
	if err != nil {
		return 0, err
	}

	r.setCache(ctx, cacheKey, count, 2*time.Minute)
	return count, nil
}

// ============================================================================
// Enrollment (no caching for consistency)
// ============================================================================

func (r *SubjectCachedRepository) GetEnrollment(ctx context.Context, userID string, subjectID string) (*subject.Enrollment, error) {
	return r.repo.GetEnrollment(ctx, userID, subjectID)
}

func (r *SubjectCachedRepository) CreateEnrollment(ctx context.Context, userID string, subjectID string) error {
	err := r.repo.CreateEnrollment(ctx, userID, subjectID)
	if err == nil {
		r.invalid.InvalidateSubject(ctx, subjectID)
	}
	return err
}

func (r *SubjectCachedRepository) DeleteEnrollment(ctx context.Context, userID string, subjectID string) error {
	err := r.repo.DeleteEnrollment(ctx, userID, subjectID)
	if err == nil {
		r.invalid.InvalidateSubject(ctx, subjectID)
	}
	return err
}

func (r *SubjectCachedRepository) UpdateEnrollmentProgress(ctx context.Context, userID string, subjectID string, progress float64) error {
	return r.repo.UpdateEnrollmentProgress(ctx, userID, subjectID, progress)
}

func (r *SubjectCachedRepository) GetUserEnrollments(ctx context.Context, userID string) ([]subject.EnrollmentWithSubject, error) {
	cacheKey := r.cacheKey("enrollments", userID)
	var cached []subject.EnrollmentWithSubject
	if r.getFromCache(ctx, cacheKey, &cached) {
		return cached, nil
	}

	enrollments, err := r.repo.GetUserEnrollments(ctx, userID)
	if err != nil {
		return nil, err
	}

	r.setCache(ctx, cacheKey, enrollments, 2*time.Minute)
	return enrollments, nil
}

func (r *SubjectCachedRepository) ListStudents(ctx context.Context, subjectID string, filter subject.StudentsFilter) (subject.StudentsResult, error) {
	return r.repo.ListStudents(ctx, subjectID, filter)
}

// ============================================================================
// Lesson Progress (user-specific, short cache)
// ============================================================================

func (r *SubjectCachedRepository) IsUserEnrolledInLessonCourse(ctx context.Context, userID string, lessonID string) (bool, error) {
	return r.repo.IsUserEnrolledInLessonCourse(ctx, userID, lessonID)
}

func (r *SubjectCachedRepository) GetSubjectIDByLesson(ctx context.Context, lessonID string) (string, error) {
	return r.repo.GetSubjectIDByLesson(ctx, lessonID)
}

func (r *SubjectCachedRepository) UpsertLessonProgress(ctx context.Context, userID string, lessonID string, data subject.LessonProgressData) error {
	err := r.repo.UpsertLessonProgress(ctx, userID, lessonID, data)
	if err == nil {
		// Invalidate completed-lesson cache if completed
		if data.Completed {
			subjectID, lookupErr := r.repo.GetSubjectIDByLesson(ctx, lessonID)
			if lookupErr == nil && subjectID != "" {
				r.redis.Del(ctx, r.cacheKey("completed", fmt.Sprintf("%s:%s", userID, subjectID)))
			}
		}
	}
	return err
}

func (r *SubjectCachedRepository) GetLessonProgressForSubject(ctx context.Context, userID string, subjectID string) ([]subject.LessonProgressInfo, error) {
	cacheKey := r.cacheKey("progress", fmt.Sprintf("%s:%s", userID, subjectID))
	var cached []subject.LessonProgressInfo
	if r.getFromCache(ctx, cacheKey, &cached) {
		return cached, nil
	}

	progress, err := r.repo.GetLessonProgressForSubject(ctx, userID, subjectID)
	if err != nil {
		return nil, err
	}

	r.setCache(ctx, cacheKey, progress, 1*time.Minute)
	return progress, nil
}

func (r *SubjectCachedRepository) GetLessonProgressCounts(ctx context.Context, userID string, subjectID string) (int64, int64, error) {
	cacheKey := r.cacheKey("progress_counts", fmt.Sprintf("%s:%s", userID, subjectID))
	type counts struct {
		Total     int64
		Completed int64
	}
	var cached counts
	if r.getFromCache(ctx, cacheKey, &cached) {
		return cached.Total, cached.Completed, nil
	}

	total, completed, err := r.repo.GetLessonProgressCounts(ctx, userID, subjectID)
	if err != nil {
		return 0, 0, err
	}

	r.setCache(ctx, cacheKey, counts{Total: total, Completed: completed}, 1*time.Minute)
	return total, completed, nil
}

// ============================================================================
// Payment
// ============================================================================

func (r *SubjectCachedRepository) HasUserPaid(ctx context.Context, userID string, subjectID string) (bool, error) {
	return r.repo.HasUserPaid(ctx, userID, subjectID)
}

// ============================================================================
// Reviews
// ============================================================================

func (r *SubjectCachedRepository) CreateReview(ctx context.Context, review *subject.Review) error {
	err := r.repo.CreateReview(ctx, review)
	if err == nil {
		r.redis.Del(ctx, r.cacheKey("reviews", review.SubjectID))
		r.redis.Del(ctx, r.cacheKey("review_stats", review.SubjectID))
	}
	return err
}

func (r *SubjectCachedRepository) GetReviews(ctx context.Context, subjectID string) ([]subject.Review, error) {
	cacheKey := r.cacheKey("reviews", subjectID)
	var cached []subject.Review
	if r.getFromCache(ctx, cacheKey, &cached) {
		return cached, nil
	}

	reviews, err := r.repo.GetReviews(ctx, subjectID)
	if err != nil {
		return nil, err
	}

	r.setCache(ctx, cacheKey, reviews, 5*time.Minute)
	return reviews, nil
}

func (r *SubjectCachedRepository) GetReviewStats(ctx context.Context, subjectID string) (*subject.ReviewStats, error) {
	cacheKey := r.cacheKey("review_stats", subjectID)
	var cached subject.ReviewStats
	if r.getFromCache(ctx, cacheKey, &cached) {
		return &cached, nil
	}

	stats, err := r.repo.GetReviewStats(ctx, subjectID)
	if err != nil {
		return nil, err
	}

	if stats != nil {
		r.setCache(ctx, cacheKey, stats, 10*time.Minute)
	}
	return stats, nil
}

func (r *SubjectCachedRepository) GetUserReview(ctx context.Context, userID string, subjectID string) (*subject.Review, error) {
	return r.repo.GetUserReview(ctx, userID, subjectID)
}

func (r *SubjectCachedRepository) UpdateSubjectRating(ctx context.Context, subjectID string) error {
	err := r.repo.UpdateSubjectRating(ctx, subjectID)
	if err == nil {
		r.invalid.InvalidateSubject(ctx, subjectID)
		r.redis.Del(ctx, r.cacheKey("review_stats", subjectID))
	}
	return err
}

// ============================================================================
// Batch Operations
// ============================================================================

func (r *SubjectCachedRepository) BatchUpdate(ctx context.Context, ids []string, updates map[string]interface{}) error {
	err := r.repo.BatchUpdate(ctx, ids, updates)
	if err == nil {
		for _, id := range ids {
			r.invalid.InvalidateSubject(ctx, id)
		}
		r.invalid.InvalidateAllLists(ctx)
	}
	return err
}

func (r *SubjectCachedRepository) BatchDelete(ctx context.Context, ids []string) error {
	err := r.repo.BatchDelete(ctx, ids)
	if err == nil {
		for _, id := range ids {
			r.invalid.InvalidateSubject(ctx, id)
		}
		r.invalid.InvalidateAllLists(ctx)
	}
	return err
}