package courseservice

import (
	"encoding/json"
	"errors"
	models "thanawy-backend/internal/domain/common"
	"time"

	db "thanawy-backend/internal/infrastructure/database"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LessonService handles business logic for course lessons
type LessonService struct{}

// NewLessonService creates a new LessonService
func NewLessonService() *LessonService {
	return &LessonService{}
}

// LessonEligibility determines if a user can access a lesson
type LessonEligibility struct {
	Eligible     bool      `json:"eligible"`
	Locked       bool      `json:"locked"`
	Reason       string    `json:"reason,omitempty"`
	ReleasesAt   time.Time `json:"releasesAt,omitempty"`
	CourseStatus string    `json:"courseStatus,omitempty"`
}

// CheckLessonEligibility checks if a user can access a specific lesson
func (s *LessonService) CheckLessonEligibility(userID, subTopicID string) (*LessonEligibility, error) {
	result := &LessonEligibility{Eligible: true}

	// Get subtopic and its parent topic/course
	var subTopic models.SubTopic
	if err := db.DB.Preload("Topic").First(&subTopic, "id = ?", subTopicID).Error; err != nil {
		return nil, errors.New("lesson not found")
	}

	var subject models.Subject
	if err := db.DB.Select("id, status, is_active, is_published, available_from, available_until, max_students").
		First(&subject, "id = ?", subTopic.Topic.SubjectID).Error; err != nil {
		return nil, errors.New("course not found")
	}

	result.CourseStatus = string(subject.Status)

	// Check course status
	if subject.Status != models.CourseStatusPublished {
		result.Eligible = false
		result.Locked = true
		result.Reason = "Course is not published"
		return result, nil
	}

	if !subject.IsActive || !subject.IsPublished {
		result.Eligible = false
		result.Locked = true
		result.Reason = "Course is not available"
		return result, nil
	}

	// Check availability windows
	now := time.Now().UTC()
	if subject.AvailableFrom != nil && now.Before(*subject.AvailableFrom) {
		result.Eligible = false
		result.Locked = true
		result.Reason = "Course not yet available"
		result.ReleasesAt = *subject.AvailableFrom
		return result, nil
	}
	if subject.AvailableUntil != nil && now.After(*subject.AvailableUntil) {
		result.Eligible = false
		result.Locked = true
		result.Reason = "Course access period has ended"
		return result, nil
	}

	// Check drip schedule for this specific lesson
	if subTopic.IsDripEnabled {
		var schedule models.LessonDripSchedule
		if err := db.DB.Where("sub_topic_id = ? AND is_active = ?", subTopicID, true).First(&schedule).Error; err == nil {
			// For drip check, we need enrollment date
			var enrollment models.Enrollment
			if err := db.DB.Where("user_id = ? AND subject_id = ?", userID, subject.ID).First(&enrollment).Error; err == nil {
				if !schedule.IsReleased(enrollment.CreatedAt) {
					result.Eligible = false
					result.Locked = true
					result.Reason = "Lesson not yet available"

					switch schedule.DripType {
					case models.DripAbsolute:
						if schedule.ReleaseDate != nil {
							result.ReleasesAt = *schedule.ReleaseDate
						}
					case models.DripRelative:
						if schedule.DaysAfterEnrollment != nil {
							result.ReleasesAt = enrollment.CreatedAt.AddDate(0, 0, *schedule.DaysAfterEnrollment)
						}
					}
					return result, nil
				}
			}
		}
	}

	// Free lessons are always accessible
	if subTopic.IsFree {
		return result, nil
	}

	// Check enrollment
	var enrollment models.Enrollment
	if err := db.DB.Where("user_id = ? AND subject_id = ?", userID, subject.ID).First(&enrollment).Error; err != nil {
		result.Eligible = false
		result.Locked = true
		result.Reason = "Not enrolled in this course"
		return result, nil
	}

	// Check bundle enrollment
	var bundleEnrollment models.BundleEnrollment
	if db.DB.Where("user_id = ? AND bundle_id IN (SELECT bundle_id FROM bundle_courses WHERE course_id = ?)",
		userID, subject.ID).First(&bundleEnrollment).Error == nil {
		// User has bundle access - treat as enrolled
		return result, nil
	}

	return result, nil
}

// GetAvailableLessons returns all lessons a user can access in a course
func (s *LessonService) GetAvailableLessons(userID, subjectID string) ([]models.SubTopic, error) {
	var subject models.Subject
	if err := db.DB.Select("id, status, is_active, is_published").First(&subject, "id = ?", subjectID).Error; err != nil {
		return nil, errors.New("course not found")
	}

	if subject.Status != models.CourseStatusPublished || !subject.IsActive || !subject.IsPublished {
		return nil, errors.New("course not available")
	}

	var subTopics []models.SubTopic
	if err := db.DB.
		Joins("JOIN Topic ON Topic.id = SubTopic.topic_id").
		Where("Topic.subject_id = ? AND SubTopic.deleted_at IS NULL", subjectID).
		Order("Topic.sort_order ASC, SubTopic.order ASC").
		Find(&subTopics).Error; err != nil {
		return nil, err
	}

	// Check enrollment
	isEnrolled := db.DB.Where("user_id = ? AND subject_id = ?", userID, subjectID).First(&models.Enrollment{}).Error == nil

	enrolledAt := time.Now().UTC()
	if isEnrolled {
		var enrollment models.Enrollment
		db.DB.Select("created_at").Where("user_id = ? AND subject_id = ?", userID, subjectID).First(&enrollment)
		enrolledAt = enrollment.CreatedAt
	}

	// Filter by drip eligibility
	var available []models.SubTopic
	for _, st := range subTopics {
		if st.IsFree || isEnrolled {
			// Check drip
			if st.IsDripEnabled {
				var schedule models.LessonDripSchedule
				if db.DB.Where("sub_topic_id = ? AND is_active = ?", st.ID, true).First(&schedule).Error == nil {
					if !schedule.IsReleased(enrolledAt) {
						continue
					}
				}
			}
			available = append(available, st)
		}
	}

	return available, nil
}

// UpdateLessonViewStats updates the view statistics for a lesson
func (s *LessonService) UpdateLessonViewStats(userID, subTopicID string, watchTime, lastPosition int, completed bool) error {
	now := time.Now().UTC()

	var stat models.LessonViewStat
	exists := db.DB.Where("sub_topic_id = ? AND user_id = ?", subTopicID, userID).First(&stat).Error == nil

	if exists {
		updates := map[string]interface{}{
			"watch_time_seconds":    watchTime,
			"last_position_seconds": lastPosition,
			"updated_at":            now,
			"attempts":              stat.Attempts + 1,
		}
		if lastPosition > stat.MaxPositionSeconds {
			updates["max_position_seconds"] = lastPosition
		}
		if completed && !stat.Completed {
			updates["completed"] = true
			updates["completed_at"] = now
		}
		return db.DB.Model(&stat).Updates(updates).Error
	}

	stat = models.LessonViewStat{
		ID:                  uuid.New().String(),
		SubTopicID:          subTopicID,
		UserID:              userID,
		WatchTimeSeconds:    watchTime,
		LastPositionSeconds: lastPosition,
		MaxPositionSeconds:  lastPosition,
		Completed:           completed,
		Attempts:            1,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if completed {
		stat.CompletedAt = &now
	}

	if err := db.DB.Create(&stat).Error; err != nil {
		return err
	}

	// Update denormalized SubTopic stats
	completedDelta := 0
	if completed {
		completedDelta = 1
	}
	return db.DB.Model(&models.SubTopic{}).
		Where("id = ?", subTopicID).
		Updates(map[string]interface{}{
			"view_count":             gorm.Expr("view_count + 1"),
			"completion_count":       gorm.Expr("completion_count + ?", completedDelta),
			"avg_watch_time_seconds": watchTime,
		}).Error
}

// GetLessonAnalytics returns analytics for a specific lesson
func (s *LessonService) GetLessonAnalytics(subTopicID string) (map[string]interface{}, error) {
	var subTopic models.SubTopic
	if err := db.DB.First(&subTopic, "id = ?", subTopicID).Error; err != nil {
		return nil, errors.New("lesson not found")
	}

	var stats struct {
		TotalViews       int64
		TotalCompletions int64
		AvgWatchTime     float64
	}
	db.DB.Model(&models.LessonViewStat{}).
		Where("sub_topic_id = ?", subTopicID).
		Select(`COUNT(*) as total_views,
			COALESCE(SUM(CASE WHEN completed THEN 1 ELSE 0 END), 0) as total_completions,
			COALESCE(AVG(watch_time_seconds), 0) as avg_watch_time`).
		Scan(&stats)

	var total int64
	db.DB.Model(&models.LessonViewStat{}).
		Where("sub_topic_id = ?", subTopicID).
		Count(&total)

	var completionRate float64
	if stats.TotalViews > 0 {
		completionRate = float64(stats.TotalCompletions) / float64(stats.TotalViews) * 100
	}

	return map[string]interface{}{
		"subTopic": map[string]interface{}{
			"id":              subTopic.ID,
			"title":           subTopic.Title,
			"viewCount":       subTopic.ViewCount,
			"completionCount": subTopic.CompletionCount,
			"avgWatchTime":    subTopic.AvgWatchTimeSeconds,
			"type":            subTopic.Type,
		},
		"totalViews":       stats.TotalViews,
		"totalCompletions": stats.TotalCompletions,
		"avgWatchTime":     stats.AvgWatchTime,
		"completionRate":   completionRate,
		"uniqueViewers":    total,
	}, nil
}

// GetVideoChapters returns parsed video chapters for a lesson
func (s *LessonService) GetVideoChapters(subTopicID string) ([]VideoChapter, error) {
	var subTopic models.SubTopic
	if err := db.DB.Select("video_chapters").First(&subTopic, "id = ?", subTopicID).Error; err != nil {
		return nil, errors.New("lesson not found")
	}

	if len(subTopic.VideoChaptersData) == 0 {
		return []VideoChapter{}, nil
	}

	var chapters []VideoChapter
	if err := json.Unmarshal(subTopic.VideoChaptersData, &chapters); err != nil {
		return nil, err
	}

	return chapters, nil
}

// VideoChapter represents a chapter in a video lesson
type VideoChapter struct {
	TimeSeconds int    `json:"timeSeconds"`
	Title       string `json:"title"`
	TitleAr     string `json:"titleAr,omitempty"`
}
