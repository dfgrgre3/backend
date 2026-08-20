package queries

import (
	"time"

	"gorm.io/gorm"

	db "thanawy-backend/internal/infrastructure/database"
)

// CourseProgressReadModel represents an enrolled course with its calculated progress.
type CourseProgressReadModel struct {
	ID             string    `json:"id"`
	EnrollmentID   string    `json:"enrollmentId"`
	Title          string    `json:"title"`
	ThumbnailURL   string    `json:"thumbnailUrl"`
	Progress       int       `json:"progress"`
	TotalLessons   int       `json:"totalLessons"`
	DoneLessons    int       `json:"doneLessons"`
	EnrolledAt     time.Time `json:"enrolledAt"`
	LastAccessedAt time.Time `json:"lastAccessedAt"`
}

// CourseProgressQueryService provides read operations for a learner's course progress.
// It computes completion dynamically from lesson-level progress rather than relying on stored placeholders.
type CourseProgressQueryService struct{}

// NewCourseProgressQueryService creates a new instance of CourseProgressQueryService.
func NewCourseProgressQueryService() *CourseProgressQueryService {
	return &CourseProgressQueryService{}
}

// readDB returns the read-only database connection.
func (s *CourseProgressQueryService) readDB() *gorm.DB {
	return db.ReadDB()
}

// courseProgressRow represents the raw database row structure for the progress query.
// Defined at the package level to avoid re-allocation on every function call.
type courseProgressRow struct {
	ID             string    `gorm:"column:id"`
	EnrollmentID   string    `gorm:"column:enrollment_id"`
	Title          string    `gorm:"column:title"`
	ThumbnailURL   *string   `gorm:"column:thumbnail_url"`
	TotalLessons   int       `gorm:"column:total_lessons"`
	DoneLessons    int       `gorm:"column:done_lessons"`
	EnrolledAt     time.Time `gorm:"column:enrolled_at"`
	LastAccessedAt time.Time `gorm:"column:last_accessed_at"`
}

// GetCourseProgress retrieves all active enrollments for a specific user.
// Lesson counts and completion percentages are derived by joining the course's topic tree with the user's topic progress.
func (s *CourseProgressQueryService) GetCourseProgress(userID string) ([]CourseProgressReadModel, error) {
	rdb := s.readDB()
	if rdb == nil {
		// Return an empty slice if the database connection is unavailable, preserving original behavior.
		return []CourseProgressReadModel{}, nil
	}

	var rows []courseProgressRow

	// Optimized and formatted SQL query for better readability, maintainability, and performance.
	// Using Raw SQL is highly recommended for complex aggregations and multiple JOINs in GORM.
	query := `
		SELECT
			s.id AS id,
			se.id AS enrollment_id,
			COALESCE(NULLIF(s.name_ar, ''), s.name) AS title,
			s.thumbnail_url AS thumbnail_url,
			COUNT(DISTINCT st.id) AS total_lessons,
			COUNT(DISTINCT CASE WHEN tp.completed = true THEN st.id ELSE NULL END) AS done_lessons,
			se.enrolled_at AS enrolled_at,
			se.updated_at AS last_accessed_at
		FROM "SubjectEnrollment" se
		JOIN "Subject" s ON s.id = se.subject_id AND s.deleted_at IS NULL
		LEFT JOIN "Topic" t ON t.subject_id = s.id AND t.deleted_at IS NULL
		LEFT JOIN "SubTopic" st ON st.topic_id = t.id AND st.deleted_at IS NULL
		LEFT JOIN "TopicProgress" tp ON tp.sub_topic_id = st.id AND tp.user_id = se.user_id AND tp.deleted_at IS NULL
		WHERE se.user_id = ? AND se.deleted_at IS NULL
		GROUP BY s.id, s.name, s.name_ar, s.thumbnail_url, se.id, se.enrolled_at, se.updated_at
		ORDER BY se.updated_at DESC
	`

	err := rdb.Raw(query, userID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	// Pre-allocate slice capacity for better memory management
	courses := make([]CourseProgressReadModel, 0, len(rows))

	for _, r := range rows {
		thumbnail := ""
		if r.ThumbnailURL != nil {
			thumbnail = *r.ThumbnailURL
		}

		progress := 0
		if r.TotalLessons > 0 {
			// Calculate percentage with explicit parentheses for clarity
			progress = (r.DoneLessons * 100) / r.TotalLessons
		}

		courses = append(courses, CourseProgressReadModel{
			ID:             r.ID,
			EnrollmentID:   r.EnrollmentID,
			Title:          r.Title,
			ThumbnailURL:   thumbnail,
			Progress:       progress,
			TotalLessons:   r.TotalLessons,
			DoneLessons:    r.DoneLessons,
			EnrolledAt:     r.EnrolledAt,
			LastAccessedAt: r.LastAccessedAt,
		})
	}

	return courses, nil
}

// CourseProgressSummaryReadModel aggregates a learner's course progress across all
// of their active enrollments.
type CourseProgressSummaryReadModel struct {
	Courses        []CourseProgressReadModel `json:"courses"`
	TotalCourses   int                       `json:"totalCourses"`
	Completed      int                       `json:"completed"`
	InProgress     int                       `json:"inProgress"`
	NotStarted     int                       `json:"notStarted"`
	AveragePercent float64                   `json:"averagePercent"`
}

// GetCourseProgressSummary derives completion stats from the enrolled courses.
// Completed = 100%, InProgress = 0 < progress < 100, NotStarted = 0%.
func (s *CourseProgressQueryService) GetCourseProgressSummary(userID string) (*CourseProgressSummaryReadModel, error) {
	courses, err := s.GetCourseProgress(userID)
	if err != nil {
		return nil, err
	}

	summary := &CourseProgressSummaryReadModel{
		Courses:      courses,
		TotalCourses: len(courses),
	}

	if len(courses) == 0 {
		return summary, nil
	}

	totalPercent := 0.0
	for _, c := range courses {
		switch {
		case c.Progress >= 100:
			summary.Completed++
		case c.Progress > 0:
			summary.InProgress++
		default:
			summary.NotStarted++
		}
		totalPercent += float64(c.Progress)
	}

	summary.AveragePercent = totalPercent / float64(len(courses))
	return summary, nil
}
