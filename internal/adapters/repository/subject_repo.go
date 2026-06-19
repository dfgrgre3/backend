package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"thanawy-backend/internal/domain/subject"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type subjectRepository struct {
	db *gorm.DB
}

func NewSubjectRepository(database *gorm.DB) subject.Repository {
	return &subjectRepository{db: database}
}

// ============================================================================
// Subject CRUD
// ============================================================================

func (r *subjectRepository) Create(ctx context.Context, s *subject.Subject) error {
	now := time.Now()
	s.CreatedAt = now
	s.UpdatedAt = now

	record := toSubjectRecord(s)
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return err
	}
	s.ID = record.ID

	// Update search vector
	_ = r.updateSearchVector(ctx, record.ID)

	return nil
}

func (r *subjectRepository) FindByID(ctx context.Context, id string) (*subject.Subject, error) {
	var record subjectRecord
	if err := r.db.WithContext(ctx).
		Preload("Topics", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" ASC")
		}).
		Preload("Topics.SubTopics", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" ASC")
		}).
		Where("id = ?", id).First(&record).Error; err != nil {
		return nil, err
	}
	return record.toDomain(), nil
}

func (r *subjectRepository) FindBySlug(ctx context.Context, slug string) (*subject.Subject, error) {
	var record subjectRecord
	if err := r.db.WithContext(ctx).
		Preload("Topics", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" ASC")
		}).
		Preload("Topics.SubTopics", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" ASC")
		}).
		Where("slug = ?", slug).First(&record).Error; err != nil {
		return nil, err
	}
	return record.toDomain(), nil
}

func (r *subjectRepository) Update(ctx context.Context, s *subject.Subject) error {
	s.UpdatedAt = time.Now()
	record := toSubjectRecord(s)
	if err := r.db.WithContext(ctx).Save(record).Error; err != nil {
		return err
	}

	// Update search vector
	_ = r.updateSearchVector(ctx, s.ID)

	return nil
}

// updateSearchVector updates the PostgreSQL tsvector column for full-text search.
func (r *subjectRepository) updateSearchVector(ctx context.Context, id string) error {
	searchSQL := `
		UPDATE "Subject"
		SET search_vector = to_tsvector('simple',
			COALESCE(name, '') || ' ' ||
			COALESCE(name_ar, '') || ' ' ||
			COALESCE(description, '') || ' ' ||
			COALESCE(code, '')
		)
		WHERE id = ?
	`
	return r.db.WithContext(ctx).Exec(searchSQL, id).Error
}

func (r *subjectRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete related records
		var topics []topicRecord
		tx.Where("subject_id = ?", id).Find(&topics)
		for _, topic := range topics {
			tx.Where("topic_id = ?", topic.ID).Delete(&subTopicRecord{})
		}
		tx.Where("subject_id = ?", id).Delete(&topicRecord{})
		tx.Where("subject_id = ?", id).Delete(&enrollmentRecord{})
		return tx.Where("id = ?", id).Delete(&subjectRecord{}).Error
	})
}

func (r *subjectRepository) List(ctx context.Context, filter subject.ListSubjectsFilter) (subject.ListSubjectsResult, error) {
	db := r.db.WithContext(ctx)

	// Build base query
	baseQuery := db.Model(&subjectRecord{})
	if filter.CategoryID != nil {
		baseQuery = baseQuery.Where("category_id = ?", *filter.CategoryID)
	}
	if filter.Level != nil {
		baseQuery = baseQuery.Where("level = ?", *filter.Level)
	}
	if filter.IsPublished != nil {
		baseQuery = baseQuery.Where("is_published = ?", *filter.IsPublished)
	}
	if filter.IsActive != nil {
		baseQuery = baseQuery.Where("is_active = ?", *filter.IsActive)
	}
	if filter.IsFeatured != nil {
		baseQuery = baseQuery.Where("is_featured = ?", *filter.IsFeatured)
	}

	// Full-Text Search with tsvector (faster than ILIKE)
	if filter.Search != nil && *filter.Search != "" {
		searchTerm := *filter.Search
		// Clean the search term for tsquery
		cleanTerm := strings.ReplaceAll(searchTerm, "'", "''")
		cleanTerm = strings.ReplaceAll(cleanTerm, " ", " & ")
		tsQuery := fmt.Sprintf("search_vector @@ to_tsquery('simple', '%s')", cleanTerm)

		// Also add relevance ranking
		baseQuery = baseQuery.Where(tsQuery + ` OR name ILIKE ? OR name_ar ILIKE ?`, "%"+searchTerm+"%", "%"+searchTerm+"%")
	}

	// Get total count using a separate optimized query
	var total int64
	countQuery := db.Model(&subjectRecord{})
	// Apply same filters to count query
	if filter.CategoryID != nil {
		countQuery = countQuery.Where("category_id = ?", *filter.CategoryID)
	}
	if filter.Level != nil {
		countQuery = countQuery.Where("level = ?", *filter.Level)
	}
	if filter.IsPublished != nil {
		countQuery = countQuery.Where("is_published = ?", *filter.IsPublished)
	}
	if filter.IsActive != nil {
		countQuery = countQuery.Where("is_active = ?", *filter.IsActive)
	}
	if filter.IsFeatured != nil {
		countQuery = countQuery.Where("is_featured = ?", *filter.IsFeatured)
	}
	if filter.Search != nil && *filter.Search != "" {
		searchTerm := *filter.Search
		cleanTerm := strings.ReplaceAll(searchTerm, "'", "''")
		cleanTerm = strings.ReplaceAll(cleanTerm, " ", " & ")
		tsQuery := fmt.Sprintf("search_vector @@ to_tsquery('simple', '%s')", cleanTerm)
		countQuery = countQuery.Where(tsQuery + ` OR name ILIKE ? OR name_ar ILIKE ?`, "%"+searchTerm+"%", "%"+searchTerm+"%")
	}
	countQuery.Count(&total)

	// Sorting
	orderClause := "created_at DESC"
	if filter.Search != nil && *filter.Search != "" {
		// Use relevance as primary sort when searching
		orderClause = "ts_rank(search_vector, to_tsquery('simple', '" + strings.ReplaceAll(*filter.Search, " ", " & ") + "')) DESC, " + orderClause
	} else if filter.SortBy != "" {
		direction := "ASC"
		if filter.SortOrder == "DESC" {
			direction = "DESC"
		}
		orderClause = fmt.Sprintf("%s %s", filter.SortBy, direction)
	}

	// Fetch paginated results (without Preload for list performance)
	offset := (filter.Page - 1) * filter.Limit
	var records []subjectRecord
	if err := baseQuery.Select("id, name, name_ar, code, description, icon, color, type, level, slug, thumbnail_url, trailer_url, seo_title, seo_description, instructor_name, instructor_id, category_id, price, is_free, is_published, is_active, is_featured, rating, enrolled_count, duration_hours, trailer_duration_minutes, language, video_count, completion_rate, created_at, updated_at").
		Order(orderClause).
		Limit(filter.Limit).
		Offset(offset).
		Find(&records).Error; err != nil {
		return subject.ListSubjectsResult{}, err
	}

	subjects := make([]subject.Subject, len(records))
	for i, r := range records {
		subjects[i] = *r.toDomain()
	}

	totalPages := int64(0)
	if total > 0 {
		totalPages = (total + int64(filter.Limit) - 1) / int64(filter.Limit)
	}

	return subject.ListSubjectsResult{
		Subjects:   subjects,
		Total:      total,
		Page:       filter.Page,
		Limit:      filter.Limit,
		TotalPages: totalPages,
	}, nil
}

// ============================================================================
// Curriculum
// ============================================================================

func (r *subjectRepository) UpdateCurriculum(ctx context.Context, subjectID string, curriculum subject.CurriculumInput) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete existing curriculum
		if err := tx.Where("subject_id = ?", subjectID).Delete(&topicRecord{}).Error; err != nil {
			return err
		}

		for _, topicInput := range curriculum.Topics {
			topic := topicRecord{
				ID:        uuid.New().String(),
				SubjectID: subjectID,
				Title:     topicInput.Title,
				Order:     topicInput.Order,
				CreatedAt: time.Now(),
			}
			if topicInput.ID != "" {
				topic.ID = topicInput.ID
			}
			if err := tx.Create(&topic).Error; err != nil {
				return err
			}

			for _, subTopicInput := range topicInput.SubTopics {
				subTopic := subTopicRecord{
					ID:          uuid.New().String(),
					TopicID:     topic.ID,
					Title:       subTopicInput.Title,
					Type:        subTopicInput.Type,
					Order:       subTopicInput.Order,
					IsFree:      subTopicInput.IsFree,
					VideoUrl:    subTopicInput.VideoUrl,
					Duration:    subTopicInput.Duration,
					DurationMin: subTopicInput.DurationMin,
					Description: subTopicInput.Description,
					CreatedAt:   time.Now(),
				}
				if subTopicInput.ID != "" {
					subTopic.ID = subTopicInput.ID
				}
				if err := tx.Create(&subTopic).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})
}

func (r *subjectRepository) GetCurriculum(ctx context.Context, subjectID string) ([]subject.Topic, error) {
	var topics []topicRecord
	if err := r.db.WithContext(ctx).
		Preload("SubTopics", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" ASC")
		}).
		Where("subject_id = ?", subjectID).
		Order("\"order\" ASC").
		Find(&topics).Error; err != nil {
		return nil, err
	}

	result := make([]subject.Topic, len(topics))
	for i, topic := range topics {
		subTopicDomain := make([]subject.SubTopic, len(topic.SubTopics))
		for j, st := range topic.SubTopics {
			subTopicDomain[j] = *st.toDomain()
		}
		result[i] = *topic.toDomain(subTopicDomain)
	}

	return result, nil
}

func (r *subjectRepository) UpdateVideoCount(ctx context.Context, subjectID string, count int) error {
	return r.db.WithContext(ctx).Model(&subjectRecord{}).Where("id = ?", subjectID).Update("video_count", count).Error
}

// ============================================================================
// Stats / Counts
// ============================================================================

func (r *subjectRepository) CountByCategory(ctx context.Context, categoryID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&subjectRecord{}).Where("category_id = ?", categoryID).Count(&count).Error
	return count, err
}

func (r *subjectRepository) CountTotal(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&subjectRecord{}).Count(&count).Error
	return count, err
}

func (r *subjectRepository) CountCompletedLessons(ctx context.Context, userID string, subjectID string) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&topicProgressRecord{}).
		Joins("JOIN SubTopic ON SubTopic.id = TopicProgress.sub_topic_id").
		Joins("JOIN Topic ON Topic.id = SubTopic.topic_id").
		Where("TopicProgress.user_id = ? AND Topic.subject_id = ? AND TopicProgress.completed = ?", userID, subjectID, true).
		Count(&count).Error
	return int(count), err
}

// ============================================================================
// Enrollment
// ============================================================================

func (r *subjectRepository) GetEnrollment(ctx context.Context, userID string, subjectID string) (*subject.Enrollment, error) {
	var record enrollmentRecord
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND subject_id = ?", userID, subjectID).
		First(&record).Error; err != nil {
		return nil, err
	}
	return record.toDomain(), nil
}

func (r *subjectRepository) CreateEnrollment(ctx context.Context, userID string, subjectID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		enrollment := enrollmentRecord{
			ID:         uuid.New().String(),
			UserID:     userID,
			SubjectID:  subjectID,
			Progress:   0,
			EnrolledAt: time.Now(),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		// Use OnConflict to handle duplicate
		if err := tx.Where("user_id = ? AND subject_id = ?", userID, subjectID).
			FirstOrCreate(&enrollment).Error; err != nil {
			return err
		}

		// Increment enrolled count
		return tx.Model(&subjectRecord{}).
			Where("id = ?", subjectID).
			Update("enrolled_count", gorm.Expr("enrolled_count + 1")).Error
	})
}

func (r *subjectRepository) DeleteEnrollment(ctx context.Context, userID string, subjectID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete lesson progress
		if err := tx.Where("user_id = ? AND sub_topic_id IN (?)",
			userID,
			tx.Table("SubTopic").
				Select("SubTopic.id").
				Joins("JOIN Topic ON Topic.id = SubTopic.topic_id").
				Where("Topic.subject_id = ?", subjectID),
		).Delete(&topicProgressRecord{}).Error; err != nil {
			return err
		}

		// Delete enrollment
		if err := tx.Where("user_id = ? AND subject_id = ?", userID, subjectID).
			Delete(&enrollmentRecord{}).Error; err != nil {
			return err
		}

		// Decrement enrolled count
		return tx.Model(&subjectRecord{}).
			Where("id = ?", subjectID).
			Update("enrolled_count", gorm.Expr("GREATEST(enrolled_count - 1, 0)")).Error
	})
}

func (r *subjectRepository) UpdateEnrollmentProgress(ctx context.Context, userID string, subjectID string, progress float64) error {
	return r.db.WithContext(ctx).Model(&enrollmentRecord{}).
		Where("user_id = ? AND subject_id = ?", userID, subjectID).
		Updates(map[string]interface{}{
			"progress":   progress,
			"updated_at": time.Now(),
		}).Error
}

func (r *subjectRepository) GetUserEnrollments(ctx context.Context, userID string) ([]subject.EnrollmentWithSubject, error) {
	var records []enrollmentRecord
	if err := r.db.WithContext(ctx).
		Preload("Subject").
		Where("user_id = ?", userID).
		Order("updated_at DESC").
		Find(&records).Error; err != nil {
		return nil, err
	}

	result := make([]subject.EnrollmentWithSubject, 0, len(records))
	for _, rec := range records {
		ews := rec.toEnrollmentWithSubject()
		result = append(result, *ews)
	}
	return result, nil
}

func (r *subjectRepository) ListStudents(ctx context.Context, subjectID string, filter subject.StudentsFilter) (subject.StudentsResult, error) {
	query := r.db.WithContext(ctx).Model(&enrollmentRecord{}).
		Preload("User").
		Where("subject_id = ?", subjectID)

	switch filter.Progress {
	case "completed":
		query = query.Where("progress >= ?", 100.0)
	case "in_progress":
		query = query.Where("progress > ? AND progress < ?", 0, 100.0)
	case "not_started":
		query = query.Where("progress = ?", 0)
	}

	var total int64
	query.Count(&total)

	var records []enrollmentRecord
	if err := query.Order("enrolled_at DESC").
		Limit(filter.Limit).
		Offset((filter.Page - 1) * filter.Limit).
		Find(&records).Error; err != nil {
		return subject.StudentsResult{}, err
	}

	students := make([]subject.StudentInfo, len(records))
	for i, rec := range records {
		students[i] = subject.StudentInfo{
			ID:         rec.ID,
			UserID:     rec.UserID,
			Progress:   rec.Progress,
			EnrolledAt: rec.EnrolledAt,
			Completed:  rec.Progress >= 100.0,
		}
		// Load user info if available
		if rec.User.ID != "" {
			students[i].Name = rec.User.Name
			students[i].Email = rec.User.Email
			students[i].Avatar = rec.User.Avatar
		}
	}

	totalPages := int64(0)
	if total > 0 {
		totalPages = (total + int64(filter.Limit) - 1) / int64(filter.Limit)
	}

	return subject.StudentsResult{
		Students:   students,
		Total:      total,
		Page:       filter.Page,
		Limit:      filter.Limit,
		TotalPages: totalPages,
	}, nil
}

// ============================================================================
// Lesson Progress
// ============================================================================

func (r *subjectRepository) IsUserEnrolledInLessonCourse(ctx context.Context, userID string, lessonID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&enrollmentRecord{}).
		Joins("JOIN Topic ON Topic.subject_id = SubjectEnrollment.subject_id").
		Joins("JOIN SubTopic ON SubTopic.topic_id = Topic.id").
		Where("SubjectEnrollment.user_id = ? AND SubTopic.id = ?", userID, lessonID).
		Count(&count).Error
	return count > 0, err
}

func (r *subjectRepository) GetSubjectIDByLesson(ctx context.Context, lessonID string) (string, error) {
	var subjectID string
	err := r.db.WithContext(ctx).Model(&subTopicRecord{}).
		Select("Topic.subject_id").
		Joins("JOIN Topic ON Topic.id = SubTopic.topic_id").
		Where("SubTopic.id = ?", lessonID).
		Scan(&subjectID).Error
	return subjectID, err
}

func (r *subjectRepository) UpsertLessonProgress(ctx context.Context, userID string, lessonID string, data subject.LessonProgressData) error {
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing topicProgressRecord
		result := tx.Where("user_id = ? AND sub_topic_id = ?", userID, lessonID).First(&existing)

		if result.Error != nil {
			// Create new
			record := topicProgressRecord{
				ID:                  uuid.New().String(),
				UserID:              userID,
				SubTopicID:          lessonID,
				Completed:           data.Completed,
				Status:              data.Status,
				TimeSpentSeconds:    data.TimeSpentSeconds,
				LastWatchedPosition: data.LastWatchedPosition,
				CreatedAt:           now,
				UpdatedAt:           now,
			}
			return tx.Create(&record).Error
		}

		// Update existing
		updates := map[string]interface{}{
			"completed":              data.Completed,
			"status":                 data.Status,
			"last_watched_position":  data.LastWatchedPosition,
			"time_spent_seconds":     gorm.Expr("time_spent_seconds + ?", data.TimeSpentSeconds),
			"updated_at":             now,
		}
		return tx.Model(&existing).Updates(updates).Error
	})
}

func (r *subjectRepository) GetLessonProgressForSubject(ctx context.Context, userID string, subjectID string) ([]subject.LessonProgressInfo, error) {
	var records []topicProgressRecord
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND sub_topic_id IN (?)",
			userID,
			r.db.Table("SubTopic").
				Select("SubTopic.id").
				Joins("JOIN Topic ON Topic.id = SubTopic.topic_id").
				Where("Topic.subject_id = ?", subjectID),
		).
		Find(&records).Error
	if err != nil {
		return nil, err
	}

	info := make([]subject.LessonProgressInfo, len(records))
	for i, rec := range records {
		info[i] = subject.LessonProgressInfo{
			LessonID:            rec.SubTopicID,
			Completed:           rec.Completed,
			Status:              rec.Status,
			TimeSpentSeconds:    rec.TimeSpentSeconds,
			LastWatchedPosition: rec.LastWatchedPosition,
			UpdatedAt:           rec.UpdatedAt,
		}
	}
	return info, nil
}

func (r *subjectRepository) GetLessonProgressCounts(ctx context.Context, userID string, subjectID string) (total int64, completed int64, err error) {
	// Total lessons
	err = r.db.WithContext(ctx).Model(&subTopicRecord{}).
		Joins("JOIN Topic ON Topic.id = SubTopic.topic_id").
		Where("Topic.subject_id = ?", subjectID).
		Count(&total).Error
	if err != nil {
		return 0, 0, err
	}

	// Completed lessons
	err = r.db.WithContext(ctx).Model(&topicProgressRecord{}).
		Where("user_id = ? AND completed = ? AND sub_topic_id IN (?)",
			userID, true,
			r.db.Table("SubTopic").
				Select("SubTopic.id").
				Joins("JOIN Topic ON Topic.id = SubTopic.topic_id").
				Where("Topic.subject_id = ?", subjectID),
		).
		Count(&completed).Error
	return total, completed, err
}

// ============================================================================
// Payment
// ============================================================================

func (r *subjectRepository) HasUserPaid(ctx context.Context, userID string, subjectID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&paymentRecord{}).
		Where("user_id = ? AND subject_id = ? AND status = ?", userID, subjectID, "COMPLETED").
		Count(&count).Error
	return count > 0, err
}

// ============================================================================
// Reviews
// ============================================================================

func (r *subjectRepository) CreateReview(ctx context.Context, review *subject.Review) error {
	if review.ID == "" {
		review.ID = uuid.New().String()
	}
	record := &reviewRecord{
		ID:        review.ID,
		SubjectID: review.SubjectID,
		UserID:    review.UserID,
		Rating:    review.Rating,
		Comment:   review.Comment,
		IsVisible: review.IsVisible,
		CreatedAt: time.Now(),
	}
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *subjectRepository) GetReviews(ctx context.Context, subjectID string) ([]subject.Review, error) {
	var records []reviewRecord
	if err := r.db.WithContext(ctx).
		Preload("User").
		Where("subject_id = ? AND is_visible = ?", subjectID, true).
		Order("created_at DESC").
		Find(&records).Error; err != nil {
		return nil, err
	}

	reviews := make([]subject.Review, len(records))
	for i, rec := range records {
		reviews[i] = *rec.toDomain()
	}
	return reviews, nil
}

func (r *subjectRepository) GetReviewStats(ctx context.Context, subjectID string) (*subject.ReviewStats, error) {
	type distributionRow struct {
		Rating int
		Count  int64
	}
	var rows []distributionRow
	if err := r.db.WithContext(ctx).Model(&reviewRecord{}).
		Select("rating, COUNT(*) as count").
		Where("subject_id = ? AND is_visible = ?", subjectID, true).
		Group("rating").
		Order("rating ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	distribution := make(map[int]int64)
	var total int64
	var sum int64
	for _, row := range rows {
		distribution[row.Rating] = row.Count
		total += row.Count
		sum += int64(row.Rating) * row.Count
	}

	var avg float64
	if total > 0 {
		avg = float64(sum) / float64(total)
	}

	return &subject.ReviewStats{
		TotalReviews: total,
		AvgRating:    avg,
		Distribution: distribution,
	}, nil
}

func (r *subjectRepository) GetUserReview(ctx context.Context, userID string, subjectID string) (*subject.Review, error) {
	var record reviewRecord
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND subject_id = ?", userID, subjectID).
		First(&record).Error; err != nil {
		return nil, err
	}
	return record.toDomain(), nil
}

func (r *subjectRepository) UpdateSubjectRating(ctx context.Context, subjectID string) error {
	var avg float64
	if err := r.db.WithContext(ctx).Model(&reviewRecord{}).
		Where("subject_id = ? AND is_visible = ?", subjectID, true).
		Select("COALESCE(AVG(rating), 0)").Scan(&avg).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&subjectRecord{}).
		Where("id = ?", subjectID).
		Update("rating", avg).Error
}

// ============================================================================
// Batch Operations
// ============================================================================

func (r *subjectRepository) BatchUpdate(ctx context.Context, ids []string, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&subjectRecord{}).
		Where("id IN ?", ids).
		Updates(updates).Error
}

func (r *subjectRepository) BatchDelete(ctx context.Context, ids []string) error {
	// Check for enrollments
	var count int64
	r.db.WithContext(ctx).Model(&enrollmentRecord{}).
		Where("subject_id IN ?", ids).Count(&count)
	if count > 0 {
		return fmt.Errorf("cannot delete courses with %d enrolled students", count)
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete topics and subtopics
		var topics []topicRecord
		tx.Where("subject_id IN ?", ids).Find(&topics)
		for _, topic := range topics {
			tx.Where("topic_id = ?", topic.ID).Delete(&subTopicRecord{})
		}
		tx.Where("subject_id IN ?", ids).Delete(&topicRecord{})
		return tx.Where("id IN ?", ids).Delete(&subjectRecord{}).Error
	})
}

// ============================================================================
// Database Records
// ============================================================================

type subjectRecord struct {
	ID                     string    `gorm:"column:id;primaryKey;type:uuid"`
	Name                   string    `gorm:"column:name"`
	NameAr                 *string   `gorm:"column:name_ar"`
	Code                   *string   `gorm:"column:code"`
	Description            *string   `gorm:"column:description"`
	Icon                   *string   `gorm:"column:icon"`
	Color                  *string   `gorm:"column:color"`
	Type                   string    `gorm:"column:type"`
	Level                  *string   `gorm:"column:level"`
	Slug                   *string   `gorm:"column:slug"`
	ThumbnailUrl           *string   `gorm:"column:thumbnail_url"`
	TrailerUrl             *string   `gorm:"column:trailer_url"`
	SeoTitle               *string   `gorm:"column:seo_title"`
	SeoDescription         *string   `gorm:"column:seo_description"`
	InstructorName         *string   `gorm:"column:instructor_name"`
	InstructorId           *string   `gorm:"column:instructor_id"`
	CategoryId             *string   `gorm:"column:category_id"`
	Price                  float64   `gorm:"column:price"`
	IsFree                 bool      `gorm:"column:is_free"`
	IsPublished            bool      `gorm:"column:is_published"`
	IsActive               bool      `gorm:"column:is_active"`
	IsFeatured             bool      `gorm:"column:is_featured"`
	Rating                 float64   `gorm:"column:rating"`
	EnrolledCount          int       `gorm:"column:enrolled_count"`
	DurationHours          *float64  `gorm:"column:duration_hours"`
	TrailerDurationMinutes *int      `gorm:"column:trailer_duration_minutes"`
	Language               *string   `gorm:"column:language"`
	VideoCount             int       `gorm:"column:video_count"`
	CompletionRate         float64   `gorm:"column:completion_rate"`
	CreatedAt              time.Time `gorm:"column:created_at"`
	UpdatedAt              time.Time `gorm:"column:updated_at"`
	Topics                 []topicRecord `gorm:"foreignKey:SubjectID;constraint:OnDelete:CASCADE"`
}

func (subjectRecord) TableName() string {
	return "Subject"
}

type topicRecord struct {
	ID        string          `gorm:"column:id;primaryKey;type:uuid"`
	SubjectID string          `gorm:"column:subject_id"`
	Title     string          `gorm:"column:title"`
	Order     int             `gorm:"column:order"`
	CreatedAt time.Time       `gorm:"column:created_at"`
	SubTopics []subTopicRecord `gorm:"foreignKey:TopicID;constraint:OnDelete:CASCADE"`
}

func (topicRecord) TableName() string {
	return "Topic"
}

type subTopicRecord struct {
	ID          string    `gorm:"column:id;primaryKey;type:uuid"`
	TopicID     string    `gorm:"column:topic_id"`
	Title       string    `gorm:"column:title"`
	Type        string    `gorm:"column:type"`
	Order       int       `gorm:"column:order"`
	IsFree      bool      `gorm:"column:is_free"`
	VideoUrl    *string   `gorm:"column:video_url"`
	Duration    int       `gorm:"column:duration"`
	DurationMin int       `gorm:"column:duration_minutes"`
	Description *string   `gorm:"column:description"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (subTopicRecord) TableName() string {
	return "SubTopic"
}

type enrollmentRecord struct {
	ID         string           `gorm:"column:id;primaryKey;type:uuid"`
	UserID     string           `gorm:"column:user_id"`
	SubjectID  string           `gorm:"column:subject_id"`
	Progress   float64          `gorm:"column:progress"`
	EnrolledAt time.Time        `gorm:"column:enrolled_at"`
	CreatedAt  time.Time        `gorm:"column:created_at"`
	UpdatedAt  time.Time        `gorm:"column:updated_at"`
	Subject    subjectRecord    `gorm:"foreignKey:SubjectID"`
	User       userProfileRecord `gorm:"foreignKey:UserID"`
}

func (enrollmentRecord) TableName() string {
	return "SubjectEnrollment"
}

type topicProgressRecord struct {
	ID                  string    `gorm:"column:id;primaryKey;type:uuid"`
	UserID              string    `gorm:"column:user_id"`
	SubTopicID          string    `gorm:"column:sub_topic_id"`
	Status              string    `gorm:"column:status"`
	Completed           bool      `gorm:"column:completed"`
	TimeSpentSeconds    int       `gorm:"column:time_spent_seconds"`
	LastWatchedPosition int       `gorm:"column:last_watched_position"`
	CreatedAt           time.Time `gorm:"column:created_at"`
	UpdatedAt           time.Time `gorm:"column:updated_at"`
}

func (topicProgressRecord) TableName() string {
	return "TopicProgress"
}

type reviewRecord struct {
	ID        string             `gorm:"column:id;primaryKey;type:uuid"`
	SubjectID string             `gorm:"column:subject_id"`
	UserID    string             `gorm:"column:user_id"`
	Rating    int                `gorm:"column:rating"`
	Comment   string             `gorm:"column:comment"`
	IsVisible bool               `gorm:"column:is_visible"`
	CreatedAt time.Time          `gorm:"column:created_at"`
	User      userProfileRecord  `gorm:"foreignKey:UserID"`
}

func (reviewRecord) TableName() string {
	return "CourseReview"
}

type paymentRecord struct {
	ID        string `gorm:"column:id;primaryKey;type:uuid"`
	UserID    string `gorm:"column:user_id"`
	SubjectID string `gorm:"column:subject_id"`
	Status    string `gorm:"column:status"`
}

func (paymentRecord) TableName() string {
	return "Payment"
}

type userProfileRecord struct {
	ID     string  `gorm:"column:id;type:uuid"`
	Name   string  `gorm:"column:name"`
	Email  string  `gorm:"column:email"`
	Avatar *string `gorm:"column:avatar"`
}

func (userProfileRecord) TableName() string {
	return "User"
}

// ============================================================================
// Mappers
// ============================================================================

func toSubjectRecord(s *subject.Subject) *subjectRecord {
	return &subjectRecord{
		ID:                     s.ID,
		Name:                   s.Name,
		NameAr:                 s.NameAr,
		Code:                   s.Code,
		Description:            s.Description,
		Icon:                   s.Icon,
		Color:                  s.Color,
		Type:                   s.Type,
		Level:                  s.Level,
		Slug:                   s.Slug,
		ThumbnailUrl:           s.ThumbnailUrl,
		TrailerUrl:             s.TrailerUrl,
		SeoTitle:               s.SeoTitle,
		SeoDescription:         s.SeoDescription,
		InstructorName:         s.InstructorName,
		InstructorId:           s.InstructorId,
		CategoryId:             s.CategoryId,
		Price:                  s.Price,
		IsFree:                 s.IsFree,
		IsPublished:            s.IsPublished,
		IsActive:               s.IsActive,
		IsFeatured:             s.IsFeatured,
		Rating:                 s.Rating,
		EnrolledCount:          s.EnrolledCount,
		DurationHours:          s.DurationHours,
		TrailerDurationMinutes: s.TrailerDurationMinutes,
		Language:               s.Language,
		VideoCount:             s.VideoCount,
		CompletionRate:         s.CompletionRate,
		CreatedAt:              s.CreatedAt,
		UpdatedAt:              s.UpdatedAt,
	}
}

func (r *subjectRecord) toDomain() *subject.Subject {
	s := &subject.Subject{
		ID:                     r.ID,
		Name:                   r.Name,
		NameAr:                 r.NameAr,
		Code:                   r.Code,
		Description:            r.Description,
		Icon:                   r.Icon,
		Color:                  r.Color,
		Type:                   r.Type,
		Level:                  r.Level,
		Slug:                   r.Slug,
		ThumbnailUrl:           r.ThumbnailUrl,
		TrailerUrl:             r.TrailerUrl,
		SeoTitle:               r.SeoTitle,
		SeoDescription:         r.SeoDescription,
		InstructorName:         r.InstructorName,
		InstructorId:           r.InstructorId,
		CategoryId:             r.CategoryId,
		Price:                  r.Price,
		IsFree:                 r.IsFree,
		IsPublished:            r.IsPublished,
		IsActive:               r.IsActive,
		IsFeatured:             r.IsFeatured,
		Rating:                 r.Rating,
		EnrolledCount:          r.EnrolledCount,
		DurationHours:          r.DurationHours,
		TrailerDurationMinutes: r.TrailerDurationMinutes,
		Language:               r.Language,
		VideoCount:             r.VideoCount,
		CompletionRate:         r.CompletionRate,
		CreatedAt:              r.CreatedAt,
		UpdatedAt:              r.UpdatedAt,
	}

	// Map topics if loaded
	if len(r.Topics) > 0 {
		topics := make([]subject.Topic, len(r.Topics))
		for i, t := range r.Topics {
			topics[i] = subject.Topic{
				ID:        t.ID,
				SubjectID: t.SubjectID,
				Title:     t.Title,
				Order:     t.Order,
				CreatedAt: t.CreatedAt,
			}
			// Map subtopics if loaded
			if len(t.SubTopics) > 0 {
				subTopics := make([]subject.SubTopic, len(t.SubTopics))
				for j, st := range t.SubTopics {
					subTopics[j] = subject.SubTopic{
						ID:          st.ID,
						TopicID:     st.TopicID,
						Title:       st.Title,
						Type:        st.Type,
						Order:       st.Order,
						IsFree:      st.IsFree,
						VideoUrl:    st.VideoUrl,
						Duration:    st.Duration,
						DurationMin: st.DurationMin,
						Description: st.Description,
						CreatedAt:   st.CreatedAt,
					}
				}
				topics[i].SubTopics = subTopics
			}
		}
		s.Topics = topics
	}

	return s
}

func (r *topicRecord) toDomain(subTopics []subject.SubTopic) *subject.Topic {
	return &subject.Topic{
		ID:        r.ID,
		SubjectID: r.SubjectID,
		Title:     r.Title,
		Order:     r.Order,
		SubTopics: subTopics,
		CreatedAt: r.CreatedAt,
	}
}

func (r *subTopicRecord) toDomain() *subject.SubTopic {
	return &subject.SubTopic{
		ID:          r.ID,
		TopicID:     r.TopicID,
		Title:       r.Title,
		Type:        r.Type,
		Order:       r.Order,
		IsFree:      r.IsFree,
		VideoUrl:    r.VideoUrl,
		Duration:    r.Duration,
		DurationMin: r.DurationMin,
		Description: r.Description,
		CreatedAt:   r.CreatedAt,
	}
}

func (r *enrollmentRecord) toDomain() *subject.Enrollment {
	return &subject.Enrollment{
		ID:         r.ID,
		UserID:     r.UserID,
		SubjectID:  r.SubjectID,
		Progress:   r.Progress,
		EnrolledAt: r.EnrolledAt,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}

func (r *enrollmentRecord) toEnrollmentWithSubject() *subject.EnrollmentWithSubject {
	ews := &subject.EnrollmentWithSubject{
		ID:         r.ID,
		UserID:     r.UserID,
		SubjectID:  r.SubjectID,
		Progress:   r.Progress,
		EnrolledAt: r.EnrolledAt,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
	if r.Subject.ID != "" {
		ews.SubjectName = r.Subject.Name
		ews.SubjectNameAr = r.Subject.NameAr
		ews.SubjectCode = r.Subject.Code
		ews.ThumbnailUrl = r.Subject.ThumbnailUrl
		ews.Rating = r.Subject.Rating
		ews.Level = r.Subject.Level
	}
	return ews
}

func (r *reviewRecord) toDomain() *subject.Review {
	review := &subject.Review{
		ID:        r.ID,
		SubjectID: r.SubjectID,
		UserID:    r.UserID,
		Rating:    r.Rating,
		Comment:   r.Comment,
		IsVisible: r.IsVisible,
		CreatedAt: r.CreatedAt,
	}
	if r.User.ID != "" {
		review.UserName = r.User.Name
		review.UserAvatar = r.User.Avatar
	}
	return review
}

// ============================================================================
// Event Publisher (No-Op)
// ============================================================================

type noOpSubjectPublisher struct{}

func NewNoOpSubjectPublisher() subject.EventPublisher {
	return &noOpSubjectPublisher{}
}

func (p *noOpSubjectPublisher) Publish(ctx context.Context, event subject.SubjectEvent) error {
	return nil
}