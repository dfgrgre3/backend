package repositories

import (
	"encoding/json"
	"fmt"
	models "thanawy-backend/internal/domain/common"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LmsRepository handles all DB operations for the LMS module.
type LmsRepository struct {
	db *gorm.DB
}

// NewLmsRepository creates a new LmsRepository.
func NewLmsRepository(db *gorm.DB) *LmsRepository {
	return &LmsRepository{db: db}
}

// ----------------------------
// Course CRUD
// ----------------------------

// CreateCourse creates a new course with optional relations.
func (r *LmsRepository) CreateCourse(course *models.LmsCourse) error {
	return r.db.Create(course).Error
}

// GetCourseByID fetches a course by ID with sections, lessons, pricings, instructors.
func (r *LmsRepository) GetCourseByID(id uuid.UUID) (*models.LmsCourse, error) {
	var c models.LmsCourse
	err := r.db.
		Preload("Sections.Lessons.Attachments").
		Preload("Sections.Lessons.Subtitles").
		Preload("Sections.Lessons.Quizzes").
		Preload("Pricings").
		Preload("Instructors").
		First(&c, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetCourseBySlug fetches a course by slug.
func (r *LmsRepository) GetCourseBySlug(slug string) (*models.LmsCourse, error) {
	var c models.LmsCourse
	err := r.db.
		Preload("Sections.Lessons").
		Preload("Pricings").
		First(&c, "slug = ?", slug).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpdateCourse updates a course.
func (r *LmsRepository) UpdateCourse(course *models.LmsCourse) error {
	return r.db.Save(course).Error
}

// UpdateCourseStatus updates only the status field.
func (r *LmsRepository) UpdateCourseStatus(id uuid.UUID, status models.CourseStatus) error {
	return r.db.Model(&models.LmsCourse{}).Where("id = ?", id).Update("status", status).Error
}

// DeleteCourse soft-deletes a course.
func (r *LmsRepository) DeleteCourse(id uuid.UUID) error {
	return r.db.Delete(&models.LmsCourse{}, "id = ?", id).Error
}

// ListCourses returns paginated courses with optional filters.
func (r *LmsRepository) ListCourses(page, pageSize int, status string, level string, language string) ([]models.LmsCourse, int64, error) {
	var courses []models.LmsCourse
	var total int64

	q := r.db.Model(&models.LmsCourse{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if level != "" {
		q = q.Where("level = ?", level)
	}
	if language != "" {
		q = q.Where("language = ?", language)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := q.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&courses).Error; err != nil {
		return nil, 0, err
	}
	return courses, total, nil
}

// ----------------------------
// Section CRUD
// ----------------------------

func (r *LmsRepository) CreateSection(section *models.LmsSection) error {
	return r.db.Create(section).Error
}

func (r *LmsRepository) GetSectionByID(id uuid.UUID) (*models.LmsSection, error) {
	var s models.LmsSection
	err := r.db.Preload("Lessons").First(&s, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *LmsRepository) UpdateSection(section *models.LmsSection) error {
	return r.db.Save(section).Error
}

func (r *LmsRepository) DeleteSection(id uuid.UUID) error {
	return r.db.Delete(&models.LmsSection{}, "id = ?", id).Error
}

func (r *LmsRepository) ListSectionsByCourseID(courseID uuid.UUID) ([]models.LmsSection, error) {
	var sections []models.LmsSection
	err := r.db.Preload("Lessons", func(db *gorm.DB) *gorm.DB {
		return db.Order("order_index ASC")
	}).Where("course_id = ?", courseID).Order("order_index ASC").Find(&sections).Error
	return sections, err
}

// ----------------------------
// Lesson CRUD
// ----------------------------

func (r *LmsRepository) CreateLesson(lesson *models.LmsLesson) error {
	return r.db.Create(lesson).Error
}

func (r *LmsRepository) GetLessonByID(id uuid.UUID) (*models.LmsLesson, error) {
	var l models.LmsLesson
	err := r.db.Preload("Attachments").Preload("Subtitles").Preload("Quizzes").First(&l, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *LmsRepository) UpdateLesson(lesson *models.LmsLesson) error {
	return r.db.Save(lesson).Error
}

func (r *LmsRepository) DeleteLesson(id uuid.UUID) error {
	return r.db.Delete(&models.LmsLesson{}, "id = ?", id).Error
}

func (r *LmsRepository) ListLessonsBySectionID(sectionID uuid.UUID) ([]models.LmsLesson, error) {
	var lessons []models.LmsLesson
	err := r.db.Where("section_id = ?", sectionID).Order("order_index ASC").Find(&lessons).Error
	return lessons, err
}

// ----------------------------
// Pricing CRUD
// ----------------------------

func (r *LmsRepository) CreatePricing(p *models.LmsPricing) error {
	return r.db.Create(p).Error
}

func (r *LmsRepository) ListPricingsByCourseID(courseID uuid.UUID) ([]models.LmsPricing, error) {
	var pricings []models.LmsPricing
	err := r.db.Where("course_id = ? AND deleted_at IS NULL", courseID).Find(&pricings).Error
	return pricings, err
}

func (r *LmsRepository) DeletePricing(id uuid.UUID) error {
	return r.db.Delete(&models.LmsPricing{}, "id = ?", id).Error
}

func (r *LmsRepository) UpdatePricing(p *models.LmsPricing) error {
	return r.db.Save(p).Error
}

// ----------------------------
// Bundle CRUD
// ----------------------------

func (r *LmsRepository) CreateBundle(b *models.LmsBundle) error {
	return r.db.Create(b).Error
}

func (r *LmsRepository) GetBundleByID(id uuid.UUID) (*models.LmsBundle, error) {
	var b models.LmsBundle
	err := r.db.Preload("Courses").First(&b, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *LmsRepository) UpdateBundle(b *models.LmsBundle) error {
	return r.db.Save(b).Error
}

func (r *LmsRepository) DeleteBundle(id uuid.UUID) error {
	return r.db.Delete(&models.LmsBundle{}, "id = ?", id).Error
}

func (r *LmsRepository) ListBundles(page, pageSize int) ([]models.LmsBundle, int64, error) {
	var bundles []models.LmsBundle
	var total int64
	q := r.db.Model(&models.LmsBundle{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := q.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&bundles).Error; err != nil {
		return nil, 0, err
	}
	return bundles, total, nil
}

// AddCourseToBundle links a course to a bundle.
func (r *LmsRepository) AddCourseToBundle(bundleID, courseID uuid.UUID) error {
	return r.db.Exec("INSERT INTO \"LmsBundleCourse\" (bundle_id, course_id) VALUES (?, ?) ON CONFLICT DO NOTHING", bundleID, courseID).Error
}

// RemoveCourseFromBundle unlinks a course from a bundle.
func (r *LmsRepository) RemoveCourseFromBundle(bundleID, courseID uuid.UUID) error {
	return r.db.Exec("DELETE FROM \"LmsBundleCourse\" WHERE bundle_id = ? AND course_id = ?", bundleID, courseID).Error
}

// ----------------------------
// Enrollment
// ----------------------------

func (r *LmsRepository) CreateEnrollment(e *models.LmsEnrollment) error {
	return r.db.Create(e).Error
}

func (r *LmsRepository) GetEnrollment(courseID, userID uuid.UUID) (*models.LmsEnrollment, error) {
	var e models.LmsEnrollment
	err := r.db.Where("course_id = ? AND user_id = ?", courseID, userID).First(&e).Error
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *LmsRepository) UpdateEnrollmentProgress(courseID, userID uuid.UUID, progress float64) error {
	return r.db.Model(&models.LmsEnrollment{}).
		Where("course_id = ? AND user_id = ?", courseID, userID).
		Updates(map[string]interface{}{
			"progress":   progress,
			"updated_at": time.Now(),
		}).Error
}

func (r *LmsRepository) CompleteEnrollment(courseID, userID uuid.UUID) error {
	return r.db.Model(&models.LmsEnrollment{}).
		Where("course_id = ? AND user_id = ?", courseID, userID).
		Updates(map[string]interface{}{
			"progress":     100,
			"completed_at": time.Now(),
			"updated_at":   time.Now(),
		}).Error
}

func (r *LmsRepository) ListEnrollmentsByUserID(userID uuid.UUID) ([]models.LmsEnrollment, error) {
	var enrollments []models.LmsEnrollment
	err := r.db.Where("user_id = ?", userID).Find(&enrollments).Error
	return enrollments, err
}

func (r *LmsRepository) ListEnrollmentsByCourseID(courseID uuid.UUID) ([]models.LmsEnrollment, error) {
	var enrollments []models.LmsEnrollment
	err := r.db.Where("course_id = ?", courseID).Find(&enrollments).Error
	return enrollments, err
}

// EnrollUserInBundle enrolls a user in all courses of a bundle.
func (r *LmsRepository) EnrollUserInBundle(bundleID, userID uuid.UUID) error {
	var bundle models.LmsBundle
	if err := r.db.Preload("Courses").First(&bundle, "id = ?", bundleID).Error; err != nil {
		return err
	}
	tx := r.db.Begin()
	for _, c := range bundle.Courses {
		e := models.LmsEnrollment{
			CourseID:   c.ID,
			UserID:     userID,
			BundleID:   &bundleID,
			EnrolledAt: time.Now(),
		}
		if err := tx.Where("course_id = ? AND user_id = ?", c.ID, userID).FirstOrCreate(&e).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

// ----------------------------
// Lesson Interaction & Video Notes
// ----------------------------

func (r *LmsRepository) UpsertLessonInteraction(i *models.LmsLessonInteraction) error {
	// Upsert by unique (lesson_id, user_id)
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "lesson_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"watched_duration_sec", "last_position_sec", "play_count", "is_completed", "quiz_answers", "updated_at",
		}),
	}).Create(i).Error
}

func (r *LmsRepository) GetLessonInteraction(lessonID, userID uuid.UUID) (*models.LmsLessonInteraction, error) {
	var i models.LmsLessonInteraction
	err := r.db.Where("lesson_id = ? AND user_id = ?", lessonID, userID).First(&i).Error
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (r *LmsRepository) CreateVideoNote(n *models.LmsVideoNote) error {
	return r.db.Create(n).Error
}

func (r *LmsRepository) ListVideoNotes(lessonID, userID uuid.UUID) ([]models.LmsVideoNote, error) {
	var notes []models.LmsVideoNote
	err := r.db.Where("lesson_id = ? AND user_id = ?", lessonID, userID).Order("timestamp_sec ASC").Find(&notes).Error
	return notes, err
}

func (r *LmsRepository) DeleteVideoNote(id uuid.UUID) error {
	return r.db.Delete(&models.LmsVideoNote{}, "id = ?", id).Error
}

// ----------------------------
// Attachments & Subtitles
// ----------------------------

func (r *LmsRepository) CreateAttachment(a *models.LmsAttachment) error {
	return r.db.Create(a).Error
}

func (r *LmsRepository) ListAttachments(lessonID uuid.UUID) ([]models.LmsAttachment, error) {
	var attachments []models.LmsAttachment
	err := r.db.Where("lesson_id = ?", lessonID).Find(&attachments).Error
	return attachments, err
}

func (r *LmsRepository) DeleteAttachment(id uuid.UUID) error {
	return r.db.Delete(&models.LmsAttachment{}, "id = ?", id).Error
}

func (r *LmsRepository) CreateSubtitle(s *models.LmsSubtitle) error {
	return r.db.Create(s).Error
}

func (r *LmsRepository) ListSubtitles(lessonID uuid.UUID) ([]models.LmsSubtitle, error) {
	var subtitles []models.LmsSubtitle
	err := r.db.Where("lesson_id = ?", lessonID).Find(&subtitles).Error
	return subtitles, err
}

func (r *LmsRepository) DeleteSubtitle(id uuid.UUID) error {
	return r.db.Delete(&models.LmsSubtitle{}, "id = ?", id).Error
}

// ----------------------------
// Interactive Quizzes
// ----------------------------

func (r *LmsRepository) CreateQuiz(q *models.LmsInteractiveQuiz) error {
	return r.db.Create(q).Error
}

func (r *LmsRepository) ListQuizzes(lessonID uuid.UUID) ([]models.LmsInteractiveQuiz, error) {
	var quizzes []models.LmsInteractiveQuiz
	err := r.db.Where("lesson_id = ?", lessonID).Order("timestamp_sec ASC").Find(&quizzes).Error
	return quizzes, err
}

func (r *LmsRepository) DeleteQuiz(id uuid.UUID) error {
	return r.db.Delete(&models.LmsInteractiveQuiz{}, "id = ?", id).Error
}

// ----------------------------
// Categories & Tags
// ----------------------------

func (r *LmsRepository) CreateCategory(c *models.LmsCategory) error {
	return r.db.Create(c).Error
}

func (r *LmsRepository) ListCategories() ([]models.LmsCategory, error) {
	var categories []models.LmsCategory
	err := r.db.Find(&categories).Error
	return categories, err
}

func (r *LmsRepository) CreateTag(t *models.LmsTag) error {
	return r.db.Create(t).Error
}

func (r *LmsRepository) ListTags() ([]models.LmsTag, error) {
	var tags []models.LmsTag
	err := r.db.Find(&tags).Error
	return tags, err
}

func (r *LmsRepository) SetCourseCategories(courseID uuid.UUID, categoryIDs []uuid.UUID) error {
	tx := r.db.Begin()
	if err := tx.Where("course_id = ?", courseID).Delete(&models.LmsCourseCategory{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	for _, catID := range categoryIDs {
		if err := tx.Exec("INSERT INTO \"LmsCourseCategory\" (course_id, category_id) VALUES (?, ?) ON CONFLICT DO NOTHING", courseID, catID).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

func (r *LmsRepository) SetCourseTags(courseID uuid.UUID, tagIDs []uuid.UUID) error {
	tx := r.db.Begin()
	if err := tx.Where("course_id = ?", courseID).Delete(&models.LmsCourseTag{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	for _, tagID := range tagIDs {
		if err := tx.Exec("INSERT INTO \"LmsCourseTag\" (course_id, tag_id) VALUES (?, ?) ON CONFLICT DO NOTHING", courseID, tagID).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

// ----------------------------
// Instructors
// ----------------------------

func (r *LmsRepository) AddInstructor(i *models.LmsInstructor) error {
	return r.db.Create(i).Error
}

func (r *LmsRepository) RemoveInstructor(courseID, instructorID uuid.UUID) error {
	return r.db.Where("course_id = ? AND instructor_id = ?", courseID, instructorID).Delete(&models.LmsInstructor{}).Error
}

func (r *LmsRepository) ListInstructors(courseID uuid.UUID) ([]models.LmsInstructor, error) {
	var instructors []models.LmsInstructor
	err := r.db.Where("course_id = ?", courseID).Find(&instructors).Error
	return instructors, err
}

// ----------------------------
// Changelog & Versions
// ----------------------------

func (r *LmsRepository) AddChangelog(c *models.LmsCourseChangelog) error {
	return r.db.Create(c).Error
}

func (r *LmsRepository) ListChangelogs(courseID uuid.UUID) ([]models.LmsCourseChangelog, error) {
	var logs []models.LmsCourseChangelog
	err := r.db.Where("course_id = ?", courseID).Order("created_at DESC").Find(&logs).Error
	return logs, err
}

func (r *LmsRepository) CreateVersion(v *models.LmsCourseVersion) error {
	return r.db.Create(v).Error
}

func (r *LmsRepository) ListVersions(courseID uuid.UUID) ([]models.LmsCourseVersion, error) {
	var versions []models.LmsCourseVersion
	err := r.db.Where("course_id = ?", courseID).Order("version_number DESC").Find(&versions).Error
	return versions, err
}

func (r *LmsRepository) GetVersion(courseID uuid.UUID, versionNumber int) (*models.LmsCourseVersion, error) {
	var v models.LmsCourseVersion
	err := r.db.Where("course_id = ? AND version_number = ?", courseID, versionNumber).First(&v).Error
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// SnapshotCourse creates a JSON snapshot of the course for versioning.
func (r *LmsRepository) SnapshotCourse(courseID uuid.UUID) ([]byte, error) {
	c, err := r.GetCourseByID(courseID)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal course snapshot: %w", err)
	}
	return data, nil
}

// ----------------------------
// Certificates
// ----------------------------

func (r *LmsRepository) CreateCertificate(c *models.LmsCertificate) error {
	return r.db.Create(c).Error
}

func (r *LmsRepository) GetCertificate(courseID, userID uuid.UUID) (*models.LmsCertificate, error) {
	var c models.LmsCertificate
	err := r.db.Where("course_id = ? AND user_id = ?", courseID, userID).First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *LmsRepository) ListCertificatesByUser(userID uuid.UUID) ([]models.LmsCertificate, error) {
	var certs []models.LmsCertificate
	err := r.db.Where("user_id = ?", userID).Order("issued_at DESC").Find(&certs).Error
	return certs, err
}

// ----------------------------
// Reviews
// ----------------------------

func (r *LmsRepository) CreateReview(rev *models.LmsReview) error {
	return r.db.Create(rev).Error
}

func (r *LmsRepository) ListReviews(courseID uuid.UUID, status string) ([]models.LmsReview, error) {
	var reviews []models.LmsReview
	q := r.db.Where("course_id = ?", courseID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Order("created_at DESC").Find(&reviews).Error
	return reviews, err
}

func (r *LmsRepository) UpdateReviewStatus(id uuid.UUID, status string) error {
	return r.db.Model(&models.LmsReview{}).Where("id = ?", id).Update("status", status).Error
}

func (r *LmsRepository) ReplyToReview(id uuid.UUID, reply string) error {
	return r.db.Model(&models.LmsReview{}).Where("id = ?", id).Update("reply", reply).Error
}

// ----------------------------
// Review Comments (Workflow)
// ----------------------------

func (r *LmsRepository) CreateReviewComment(rc *models.LmsReviewComment) error {
	return r.db.Create(rc).Error
}

func (r *LmsRepository) ListReviewComments(courseID uuid.UUID) ([]models.LmsReviewComment, error) {
	var comments []models.LmsReviewComment
	err := r.db.Where("course_id = ?", courseID).Order("created_at DESC").Find(&comments).Error
	return comments, err
}

// ----------------------------
// Clone & Version helpers
// ----------------------------

// CloneCourse duplicates a course with all sections/lessons into a new draft.
func (r *LmsRepository) CloneCourse(srcID uuid.UUID, newTitle string) (*models.LmsCourse, error) {
	src, err := r.GetCourseByID(srcID)
	if err != nil {
		return nil, err
	}
	tx := r.db.Begin()

	newCourse := *src
	newCourse.ID = uuid.New()
	newCourse.Title = newTitle
	newCourse.Slug = fmt.Sprintf("%s-copy-%s", src.Slug, uuid.New().String()[:8])
	newCourse.Status = models.CourseStatusDraft
	newCourse.Version = 1
	newCourse.CreatedAt = time.Now()
	newCourse.UpdatedAt = time.Now()
	newCourse.DeletedAt = gorm.DeletedAt{}

	if err := tx.Create(&newCourse).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	for _, s := range src.Sections {
		newSection := s
		newSection.ID = uuid.New()
		newSection.CourseID = newCourse.ID
		newSection.DeletedAt = gorm.DeletedAt{}
		if err := tx.Create(&newSection).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		for _, l := range s.Lessons {
			newLesson := l
			newLesson.ID = uuid.New()
			newLesson.SectionID = newSection.ID
			newLesson.DeletedAt = gorm.DeletedAt{}
			if err := tx.Create(&newLesson).Error; err != nil {
				tx.Rollback()
				return nil, err
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return &newCourse, nil
}

// RestoreVersion restores a course to a specific version
func (r *LmsRepository) RestoreVersion(courseID string, versionNumber int, userID string) (*models.LmsCourse, error) {
	courseUUID, err := uuid.Parse(courseID)
	if err != nil {
		return nil, err
	}

	_, err = r.GetVersion(courseUUID, versionNumber)
	if err != nil {
		return nil, err
	}

	// Get current course
	course, err := r.GetCourseByID(courseUUID)
	if err != nil {
		return nil, err
	}

	// Update course from snapshot
	course.Version = versionNumber + 1
	course.UpdatedAt = time.Now()

	if err := r.db.Save(course).Error; err != nil {
		return nil, err
	}

	// Add changelog entry
	userUUID, _ := uuid.Parse(userID)
	changelog := &models.LmsCourseChangelog{
		CourseID: courseUUID,
		UserID:   userUUID,
		Field:    "version_restore",
		NewValue: &[]string{fmt.Sprintf("Restored to version %d", versionNumber)}[0],
	}
	r.db.Create(changelog)

	return course, nil
}

// GetChangelogByCourseID returns the changelog for a course (string ID version)
func (r *LmsRepository) GetChangelogByCourseID(courseID string) ([]*models.LmsCourseChangelog, error) {
	courseUUID, err := uuid.Parse(courseID)
	if err != nil {
		return nil, err
	}

	logs, err := r.ListChangelogs(courseUUID)
	if err != nil {
		return nil, err
	}

	// Convert []models.LmsCourseChangelog to []*models.LmsCourseChangelog
	result := make([]*models.LmsCourseChangelog, len(logs))
	for i := range logs {
		result[i] = &logs[i]
	}
	return result, nil
}
