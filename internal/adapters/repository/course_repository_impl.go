package repository

import (
	"context"

	"thanawy-backend/internal/domain/course"
	"thanawy-backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CourseRepositoryImpl implements the course repository interface using GORM
type CourseRepositoryImpl struct {
	db *gorm.DB
}

// NewCourseRepositoryImpl creates a new course repository implementation
func NewCourseRepositoryImpl(db *gorm.DB) *CourseRepositoryImpl {
	return &CourseRepositoryImpl{db: db}
}

// =============================================================
// Course operations
// =============================================================

func (r *CourseRepositoryImpl) CreateCourse(ctx context.Context, courseEntity *course.Course) error {
	lmsCourse := r.domainToModel(courseEntity)
	return r.db.WithContext(ctx).Create(&lmsCourse).Error
}

func (r *CourseRepositoryImpl) GetCourseByID(ctx context.Context, id string) (*course.Course, error) {
	var lmsCourse models.LmsCourse
	err := r.db.WithContext(ctx).
		Preload("Sections.Lessons.Attachments").
		Preload("Sections.Lessons.Subtitles").
		Preload("Pricings").
		Preload("Instructors").
		First(&lmsCourse, "id = ?", id).Error

	if err != nil {
		return nil, err
	}

	return r.modelToDomain(&lmsCourse), nil
}

func (r *CourseRepositoryImpl) GetCourseBySlug(ctx context.Context, slug string) (*course.Course, error) {
	var lmsCourse models.LmsCourse
	err := r.db.WithContext(ctx).
		Preload("Sections.Lessons.Attachments").
		Preload("Sections.Lessons.Subtitles").
		Preload("Pricings").
		Preload("Instructors").
		First(&lmsCourse, "slug = ?", slug).Error

	if err != nil {
		return nil, err
	}

	return r.modelToDomain(&lmsCourse), nil
}

func (r *CourseRepositoryImpl) UpdateCourse(ctx context.Context, courseEntity *course.Course) error {
	lmsCourse := r.domainToModel(courseEntity)
	return r.db.WithContext(ctx).Save(&lmsCourse).Error
}

func (r *CourseRepositoryImpl) DeleteCourse(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.LmsCourse{}, "id = ?", id).Error
}

func (r *CourseRepositoryImpl) ListCourses(ctx context.Context, filter course.CourseFilter) ([]*course.Course, int, error) {
	query := r.db.WithContext(ctx).Model(&models.LmsCourse{})

	// Apply filters
	if filter.Status != nil {
		query = query.Where("status = ?", string(*filter.Status))
	}
	if filter.Level != nil {
		query = query.Where("level = ?", string(*filter.Level))
	}
	if filter.Language != nil {
		query = query.Where("language = ?", *filter.Language)
	}
	if filter.IsFeatured != nil {
		query = query.Where("is_featured = ?", *filter.IsFeatured)
	}
	if filter.IsTrending != nil {
		query = query.Where("is_trending = ?", *filter.IsTrending)
	}
	if filter.IsNew != nil {
		query = query.Where("is_new = ?", *filter.IsNew)
	}
	if filter.SearchQuery != nil {
		query = query.Where("title ILIKE ? OR short_description ILIKE ?", "%"+*filter.SearchQuery+"%", "%"+*filter.SearchQuery+"%")
	}

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if filter.Page > 0 && filter.Limit > 0 {
		offset := (filter.Page - 1) * filter.Limit
		query = query.Offset(offset).Limit(filter.Limit)
	}

	var lmsCourses []models.LmsCourse
	if err := query.
		Preload("Sections.Lessons").
		Preload("Pricings").
		Order("created_at DESC").
		Find(&lmsCourses).Error; err != nil {
		return nil, 0, err
	}

	courses := make([]*course.Course, len(lmsCourses))
	for i, c := range lmsCourses {
		courses[i] = r.modelToDomain(&c)
	}

	return courses, int(total), nil
}

// =============================================================
// Section operations
// =============================================================

func (r *CourseRepositoryImpl) CreateSection(ctx context.Context, section *course.Section) error {
	lmsSection := r.domainSectionToModel(section)
	return r.db.WithContext(ctx).Create(&lmsSection).Error
}

func (r *CourseRepositoryImpl) GetSectionByID(ctx context.Context, id string) (*course.Section, error) {
	var lmsSection models.LmsSection
	err := r.db.WithContext(ctx).
		Preload("Lessons.Attachments").
		First(&lmsSection, "id = ?", id).Error

	if err != nil {
		return nil, err
	}

	return r.modelSectionToDomain(&lmsSection), nil
}

func (r *CourseRepositoryImpl) UpdateSection(ctx context.Context, section *course.Section) error {
	lmsSection := r.domainSectionToModel(section)
	return r.db.WithContext(ctx).Save(&lmsSection).Error
}

func (r *CourseRepositoryImpl) DeleteSection(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.LmsSection{}, "id = ?", id).Error
}

func (r *CourseRepositoryImpl) ListSections(ctx context.Context, courseID string) ([]*course.Section, error) {
	var lmsSections []models.LmsSection
	err := r.db.WithContext(ctx).
		Preload("Lessons").
		Where("course_id = ?", courseID).
		Order("order_index ASC").
		Find(&lmsSections).Error

	if err != nil {
		return nil, err
	}

	sections := make([]*course.Section, len(lmsSections))
	for i, s := range lmsSections {
		sections[i] = r.modelSectionToDomain(&s)
	}

	return sections, nil
}

func (r *CourseRepositoryImpl) ReorderSections(ctx context.Context, courseID string, sectionIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, sectionID := range sectionIDs {
			if err := tx.Model(&models.LmsSection{}).
				Where("id = ?", sectionID).
				Update("order_index", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// =============================================================
// Lesson operations
// =============================================================

func (r *CourseRepositoryImpl) CreateLesson(ctx context.Context, lesson *course.Lesson) error {
	lmsLesson := r.domainLessonToModel(lesson)
	return r.db.WithContext(ctx).Create(&lmsLesson).Error
}

func (r *CourseRepositoryImpl) GetLessonByID(ctx context.Context, id string) (*course.Lesson, error) {
	var lmsLesson models.LmsLesson
	err := r.db.WithContext(ctx).
		Preload("Attachments").
		Preload("Subtitles").
		First(&lmsLesson, "id = ?", id).Error

	if err != nil {
		return nil, err
	}

	return r.modelLessonToDomain(&lmsLesson), nil
}

func (r *CourseRepositoryImpl) UpdateLesson(ctx context.Context, lesson *course.Lesson) error {
	lmsLesson := r.domainLessonToModel(lesson)
	return r.db.WithContext(ctx).Save(&lmsLesson).Error
}

func (r *CourseRepositoryImpl) DeleteLesson(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&models.LmsLesson{}, "id = ?", id).Error
}

func (r *CourseRepositoryImpl) ListLessons(ctx context.Context, sectionID string) ([]*course.Lesson, error) {
	var lmsLessons []models.LmsLesson
	err := r.db.WithContext(ctx).
		Preload("Attachments").
		Where("section_id = ?", sectionID).
		Order("order_index ASC").
		Find(&lmsLessons).Error

	if err != nil {
		return nil, err
	}

	lessons := make([]*course.Lesson, len(lmsLessons))
	for i, l := range lmsLessons {
		lessons[i] = r.modelLessonToDomain(&l)
	}

	return lessons, nil
}

func (r *CourseRepositoryImpl) ReorderLessons(ctx context.Context, sectionID string, lessonIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, lessonID := range lessonIDs {
			if err := tx.Model(&models.LmsLesson{}).
				Where("id = ?", lessonID).
				Update("order_index", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// =============================================================
// Enrollment operations
// =============================================================

func (r *CourseRepositoryImpl) CreateEnrollment(ctx context.Context, enrollment *course.Enrollment) error {
	lmsEnrollment := r.domainEnrollmentToModel(enrollment)
	return r.db.WithContext(ctx).Create(&lmsEnrollment).Error
}

func (r *CourseRepositoryImpl) GetEnrollment(ctx context.Context, courseID, userID string) (*course.Enrollment, error) {
	var lmsEnrollment models.LmsEnrollment
	err := r.db.WithContext(ctx).
		Where("course_id = ? AND user_id = ?", courseID, userID).
		First(&lmsEnrollment).Error

	if err != nil {
		return nil, err
	}

	return r.modelEnrollmentToDomain(&lmsEnrollment), nil
}

func (r *CourseRepositoryImpl) UpdateEnrollmentProgress(ctx context.Context, enrollment *course.Enrollment) error {
	lmsEnrollment := r.domainEnrollmentToModel(enrollment)
	return r.db.WithContext(ctx).Save(&lmsEnrollment).Error
}

func (r *CourseRepositoryImpl) ListEnrollments(ctx context.Context, filter course.EnrollmentFilter) ([]*course.Enrollment, int, error) {
	query := r.db.WithContext(ctx).Model(&models.LmsEnrollment{})

	if filter.CourseID != nil {
		query = query.Where("course_id = ?", *filter.CourseID)
	}
	if filter.UserID != nil {
		query = query.Where("user_id = ?", *filter.UserID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if filter.Page > 0 && filter.Limit > 0 {
		offset := (filter.Page - 1) * filter.Limit
		query = query.Offset(offset).Limit(filter.Limit)
	}

	var lmsEnrollments []models.LmsEnrollment
	if err := query.Order("enrolled_at DESC").Find(&lmsEnrollments).Error; err != nil {
		return nil, 0, err
	}

	enrollments := make([]*course.Enrollment, len(lmsEnrollments))
	for i, e := range lmsEnrollments {
		enrollments[i] = r.modelEnrollmentToDomain(&e)
	}

	return enrollments, int(total), nil
}

// =============================================================
// Pricing operations
// =============================================================

func (r *CourseRepositoryImpl) CreatePricing(ctx context.Context, pricing *course.Pricing) error {
	lmsPricing := r.domainPricingToModel(pricing)
	return r.db.WithContext(ctx).Create(&lmsPricing).Error
}

func (r *CourseRepositoryImpl) GetPricing(ctx context.Context, courseID string) (*course.Pricing, error) {
	var lmsPricing models.LmsPricing
	err := r.db.WithContext(ctx).
		Where("course_id = ?", courseID).
		First(&lmsPricing).Error

	if err != nil {
		return nil, err
	}

	return r.modelPricingToDomain(&lmsPricing), nil
}

func (r *CourseRepositoryImpl) UpdatePricing(ctx context.Context, pricing *course.Pricing) error {
	lmsPricing := r.domainPricingToModel(pricing)
	return r.db.WithContext(ctx).Save(&lmsPricing).Error
}

func (r *CourseRepositoryImpl) DeletePricing(ctx context.Context, courseID string) error {
	return r.db.WithContext(ctx).Delete(&models.LmsPricing{}, "course_id = ?", courseID).Error
}

// =============================================================
// Category operations
// =============================================================

func (r *CourseRepositoryImpl) AddCourseCategory(ctx context.Context, courseID, categoryID string) error {
	join := models.LmsCourseCategory{
		CourseID:   uuid.MustParse(courseID),
		CategoryID: uuid.MustParse(categoryID),
	}
	return r.db.WithContext(ctx).Create(&join).Error
}

func (r *CourseRepositoryImpl) RemoveCourseCategory(ctx context.Context, courseID, categoryID string) error {
	return r.db.WithContext(ctx).
		Where("course_id = ? AND category_id = ?", courseID, categoryID).
		Delete(&models.LmsCourseCategory{}).Error
}

func (r *CourseRepositoryImpl) ListCourseCategories(ctx context.Context, courseID string) ([]*course.Category, error) {
	var categories []models.LmsCategory
	err := r.db.WithContext(ctx).
		Table("lms_categories").
		Joins("INNER JOIN lms_course_categories ON lms_categories.id = lms_course_categories.category_id").
		Where("lms_course_categories.course_id = ?", courseID).
		Find(&categories).Error

	if err != nil {
		return nil, err
	}

	result := make([]*course.Category, len(categories))
	for i, c := range categories {
		result[i] = &course.Category{
			ID:        c.ID,
			Name:      c.Name,
			Slug:      c.Slug,
			ParentID:  c.ParentID,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		}
	}

	return result, nil
}

// =============================================================
// Instructor operations
// =============================================================

func (r *CourseRepositoryImpl) AddCourseInstructor(ctx context.Context, courseID, instructorID string, role string) error {
	instructor := models.LmsInstructor{
		CourseID:     uuid.MustParse(courseID),
		InstructorID: uuid.MustParse(instructorID),
		Role:         role,
	}
	return r.db.WithContext(ctx).Create(&instructor).Error
}

func (r *CourseRepositoryImpl) RemoveCourseInstructor(ctx context.Context, courseID, instructorID string) error {
	return r.db.WithContext(ctx).
		Where("course_id = ? AND instructor_id = ?", courseID, instructorID).
		Delete(&models.LmsInstructor{}).Error
}

func (r *CourseRepositoryImpl) ListCourseInstructors(ctx context.Context, courseID string) ([]*course.Instructor, error) {
	var instructors []models.LmsInstructor
	err := r.db.WithContext(ctx).
		Where("course_id = ?", courseID).
		Find(&instructors).Error

	if err != nil {
		return nil, err
	}

	result := make([]*course.Instructor, len(instructors))
	for i, inst := range instructors {
		result[i] = &course.Instructor{
			CourseID:     inst.CourseID,
			InstructorID: inst.InstructorID,
			Role:         inst.Role,
			Permissions:  inst.Permissions,
			CreatedAt:    inst.CreatedAt,
		}
	}

	return result, nil
}

// =============================================================
// Review operations
// =============================================================

func (r *CourseRepositoryImpl) CreateReview(ctx context.Context, review *course.Review) error {
	lmsReview := r.domainReviewToModel(review)
	return r.db.WithContext(ctx).Create(&lmsReview).Error
}

func (r *CourseRepositoryImpl) GetReview(ctx context.Context, courseID, userID string) (*course.Review, error) {
	var lmsReview models.LmsReview
	err := r.db.WithContext(ctx).
		Where("course_id = ? AND user_id = ?", courseID, userID).
		First(&lmsReview).Error

	if err != nil {
		return nil, err
	}

	return r.modelReviewToDomain(&lmsReview), nil
}

func (r *CourseRepositoryImpl) UpdateReview(ctx context.Context, review *course.Review) error {
	lmsReview := r.domainReviewToModel(review)
	return r.db.WithContext(ctx).Save(&lmsReview).Error
}

func (r *CourseRepositoryImpl) ListReviews(ctx context.Context, courseID string) ([]*course.Review, error) {
	var lmsReviews []models.LmsReview
	err := r.db.WithContext(ctx).
		Where("course_id = ?", courseID).
		Order("created_at DESC").
		Find(&lmsReviews).Error

	if err != nil {
		return nil, err
	}

	reviews := make([]*course.Review, len(lmsReviews))
	for i, lmsReview := range lmsReviews {
		reviews[i] = r.modelReviewToDomain(&lmsReview)
	}

	return reviews, nil
}

// =============================================================
// Certificate operations
// =============================================================

func (r *CourseRepositoryImpl) CreateCertificate(ctx context.Context, certificate *course.Certificate) error {
	lmsCertificate := r.domainCertificateToModel(certificate)
	return r.db.WithContext(ctx).Create(&lmsCertificate).Error
}

func (r *CourseRepositoryImpl) GetCertificate(ctx context.Context, courseID, userID string) (*course.Certificate, error) {
	var lmsCertificate models.LmsCertificate
	err := r.db.WithContext(ctx).
		Where("course_id = ? AND user_id = ?", courseID, userID).
		First(&lmsCertificate).Error

	if err != nil {
		return nil, err
	}

	return r.modelCertificateToDomain(&lmsCertificate), nil
}

func (r *CourseRepositoryImpl) ListCertificates(ctx context.Context, userID string) ([]*course.Certificate, error) {
	var lmsCertificates []models.LmsCertificate
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("issued_at DESC").
		Find(&lmsCertificates).Error

	if err != nil {
		return nil, err
	}

	certificates := make([]*course.Certificate, len(lmsCertificates))
	for i, c := range lmsCertificates {
		certificates[i] = r.modelCertificateToDomain(&c)
	}

	return certificates, nil
}

// =============================================================
// Domain <-> Model conversion helpers
// =============================================================

func (r *CourseRepositoryImpl) domainToModel(courseEntity *course.Course) *models.LmsCourse {
	return &models.LmsCourse{
		ID:                    courseEntity.ID,
		Title:                 courseEntity.Title,
		Slug:                  courseEntity.Slug,
		ShortDescription:      courseEntity.ShortDescription,
		LongDescription:       courseEntity.LongDescription,
		CoverImageURL:         courseEntity.CoverImageURL,
		PromoVideoURL:         courseEntity.PromoVideoURL,
		Status:                models.CourseStatus(courseEntity.Status),
		Level:                 models.CourseLevel(courseEntity.Level),
		Language:              courseEntity.Language,
		EstimatedDurationMins: courseEntity.EstimatedDurationMins,
		HasCertificate:        courseEntity.HasCertificate,
		CertificateTemplate:   courseEntity.CertificateTemplate,
		MaxStudents:           courseEntity.MaxStudents,
		Version:               courseEntity.Version,
		IsFeatured:            courseEntity.IsFeatured,
		IsTrending:            courseEntity.IsTrending,
		IsNew:                 courseEntity.IsNew,
		NewFrom:               courseEntity.NewFrom,
		NewUntil:              courseEntity.NewUntil,
		SEOTitle:              courseEntity.SEOTitle,
		SEODescription:        courseEntity.SEODescription,
		SEOKeywords:           models.PGStringArray(courseEntity.SEOKeywords),
		PrerequisitesText:     courseEntity.PrerequisitesText,
		TargetAudience:        courseEntity.TargetAudience,
		LearningOutcomes:      models.PGStringArray(courseEntity.LearningOutcomes),
		PrimaryInstructorID:   courseEntity.PrimaryInstructorID,
		AvailableFrom:         courseEntity.AvailableFrom,
		AvailableUntil:        courseEntity.AvailableUntil,
		CreatedAt:             courseEntity.CreatedAt,
		UpdatedAt:             courseEntity.UpdatedAt,
	}
}

func (r *CourseRepositoryImpl) modelToDomain(lmsCourse *models.LmsCourse) *course.Course {
	sections := make([]course.Section, len(lmsCourse.Sections))
	for i, s := range lmsCourse.Sections {
		sections[i] = *r.modelSectionToDomain(&s)
	}

	pricings := make([]course.Pricing, len(lmsCourse.Pricings))
	for i, p := range lmsCourse.Pricings {
		pricings[i] = *r.modelPricingToDomain(&p)
	}

	return &course.Course{
		ID:                    lmsCourse.ID,
		Title:                 lmsCourse.Title,
		Slug:                  lmsCourse.Slug,
		ShortDescription:      lmsCourse.ShortDescription,
		LongDescription:       lmsCourse.LongDescription,
		CoverImageURL:         lmsCourse.CoverImageURL,
		PromoVideoURL:         lmsCourse.PromoVideoURL,
		Status:                course.CourseStatus(lmsCourse.Status),
		Level:                 course.CourseLevel(lmsCourse.Level),
		Language:              lmsCourse.Language,
		EstimatedDurationMins: lmsCourse.EstimatedDurationMins,
		HasCertificate:        lmsCourse.HasCertificate,
		CertificateTemplate:   lmsCourse.CertificateTemplate,
		MaxStudents:           lmsCourse.MaxStudents,
		Version:               lmsCourse.Version,
		IsFeatured:            lmsCourse.IsFeatured,
		IsTrending:            lmsCourse.IsTrending,
		IsNew:                 lmsCourse.IsNew,
		NewFrom:               lmsCourse.NewFrom,
		NewUntil:              lmsCourse.NewUntil,
		SEOTitle:              lmsCourse.SEOTitle,
		SEODescription:        lmsCourse.SEODescription,
		SEOKeywords:           []string(lmsCourse.SEOKeywords),
		PrerequisitesText:     lmsCourse.PrerequisitesText,
		TargetAudience:        lmsCourse.TargetAudience,
		LearningOutcomes:      []string(lmsCourse.LearningOutcomes),
		PrimaryInstructorID:   lmsCourse.PrimaryInstructorID,
		AvailableFrom:         lmsCourse.AvailableFrom,
		AvailableUntil:        lmsCourse.AvailableUntil,
		CreatedAt:             lmsCourse.CreatedAt,
		UpdatedAt:             lmsCourse.UpdatedAt,
		Sections:              sections,
		Pricings:              pricings,
	}
}

func (r *CourseRepositoryImpl) domainSectionToModel(section *course.Section) *models.LmsSection {
	return &models.LmsSection{
		ID:            section.ID,
		CourseID:      section.CourseID,
		Title:         section.Title,
		OrderIndex:    section.OrderIndex,
		AvailableFrom: section.AvailableFrom,
		DripDelayDays: section.DripDelayDays,
		CreatedAt:     section.CreatedAt,
		UpdatedAt:     section.UpdatedAt,
	}
}

func (r *CourseRepositoryImpl) modelSectionToDomain(lmsSection *models.LmsSection) *course.Section {
	lessons := make([]course.Lesson, len(lmsSection.Lessons))
	for i, l := range lmsSection.Lessons {
		lessons[i] = *r.modelLessonToDomain(&l)
	}

	return &course.Section{
		ID:            lmsSection.ID,
		CourseID:      lmsSection.CourseID,
		Title:         lmsSection.Title,
		OrderIndex:    lmsSection.OrderIndex,
		AvailableFrom: lmsSection.AvailableFrom,
		DripDelayDays: lmsSection.DripDelayDays,
		CreatedAt:     lmsSection.CreatedAt,
		UpdatedAt:     lmsSection.UpdatedAt,
		Lessons:       lessons,
	}
}

func (r *CourseRepositoryImpl) domainLessonToModel(lesson *course.Lesson) *models.LmsLesson {
	return &models.LmsLesson{
		ID:               lesson.ID,
		SectionID:        lesson.SectionID,
		Title:            lesson.Title,
		Type:             models.LessonType(lesson.Type),
		Content:          lesson.Content,
		MediaURL:         lesson.MediaURL,
		DurationSeconds:  lesson.DurationSeconds,
		IsFreePreview:    lesson.IsFreePreview,
		OrderIndex:       lesson.OrderIndex,
		AvailabilityType: models.AvailabilityType(lesson.AvailabilityType),
		AvailableFrom:    lesson.AvailableFrom,
		DripDelayDays:    lesson.DripDelayDays,
		CreatedAt:        lesson.CreatedAt,
		UpdatedAt:        lesson.UpdatedAt,
	}
}

func (r *CourseRepositoryImpl) modelLessonToDomain(lmsLesson *models.LmsLesson) *course.Lesson {
	attachments := make([]course.Attachment, len(lmsLesson.Attachments))
	for i, a := range lmsLesson.Attachments {
		attachments[i] = *r.modelAttachmentToDomain(&a)
	}

	return &course.Lesson{
		ID:               lmsLesson.ID,
		SectionID:        lmsLesson.SectionID,
		Title:            lmsLesson.Title,
		Type:             course.LessonType(lmsLesson.Type),
		Content:          lmsLesson.Content,
		MediaURL:         lmsLesson.MediaURL,
		DurationSeconds:  lmsLesson.DurationSeconds,
		IsFreePreview:    lmsLesson.IsFreePreview,
		OrderIndex:       lmsLesson.OrderIndex,
		AvailabilityType: course.AvailabilityType(lmsLesson.AvailabilityType),
		AvailableFrom:    lmsLesson.AvailableFrom,
		DripDelayDays:    lmsLesson.DripDelayDays,
		CreatedAt:        lmsLesson.CreatedAt,
		UpdatedAt:        lmsLesson.UpdatedAt,
		Attachments:      attachments,
	}
}

func (r *CourseRepositoryImpl) modelAttachmentToDomain(lmsAttachment *models.LmsAttachment) *course.Attachment {
	return &course.Attachment{
		ID:        lmsAttachment.ID,
		LessonID:  lmsAttachment.LessonID,
		Title:     lmsAttachment.Title,
		FileURL:   lmsAttachment.FileURL,
		FileType:  lmsAttachment.FileType,
		FileSize:  lmsAttachment.FileSize,
		CreatedAt: lmsAttachment.CreatedAt,
	}
}

func (r *CourseRepositoryImpl) domainEnrollmentToModel(enrollment *course.Enrollment) *models.LmsEnrollment {
	return &models.LmsEnrollment{
		ID:          enrollment.ID,
		CourseID:    enrollment.CourseID,
		UserID:      enrollment.UserID,
		Progress:    enrollment.Progress,
		EnrolledAt:  enrollment.EnrolledAt,
		CompletedAt: enrollment.CompletedAt,
		BundleID:    enrollment.BundleID,
		CreatedAt:   enrollment.CreatedAt,
		UpdatedAt:   enrollment.UpdatedAt,
	}
}

func (r *CourseRepositoryImpl) modelEnrollmentToDomain(lmsEnrollment *models.LmsEnrollment) *course.Enrollment {
	return &course.Enrollment{
		ID:          lmsEnrollment.ID,
		CourseID:    lmsEnrollment.CourseID,
		UserID:      lmsEnrollment.UserID,
		Progress:    lmsEnrollment.Progress,
		EnrolledAt:  lmsEnrollment.EnrolledAt,
		CompletedAt: lmsEnrollment.CompletedAt,
		BundleID:    lmsEnrollment.BundleID,
		CreatedAt:   lmsEnrollment.CreatedAt,
		UpdatedAt:   lmsEnrollment.UpdatedAt,
	}
}

func (r *CourseRepositoryImpl) domainPricingToModel(pricing *course.Pricing) *models.LmsPricing {
	return &models.LmsPricing{
		ID:                       pricing.ID,
		CourseID:                 pricing.CourseID,
		Type:                     models.PriceType(pricing.Type),
		Amount:                   pricing.Amount,
		CurrencyCode:             pricing.CurrencyCode,
		SubscriptionDurationDays: pricing.SubscriptionDurationDays,
		IsActive:                 pricing.IsActive,
		CreatedAt:                pricing.CreatedAt,
		UpdatedAt:                pricing.UpdatedAt,
	}
}

func (r *CourseRepositoryImpl) modelPricingToDomain(lmsPricing *models.LmsPricing) *course.Pricing {
	return &course.Pricing{
		ID:                       lmsPricing.ID,
		CourseID:                 lmsPricing.CourseID,
		Type:                     course.PricingType(lmsPricing.Type),
		Amount:                   lmsPricing.Amount,
		CurrencyCode:             lmsPricing.CurrencyCode,
		SubscriptionDurationDays: lmsPricing.SubscriptionDurationDays,
		IsActive:                 lmsPricing.IsActive,
		CreatedAt:                lmsPricing.CreatedAt,
		UpdatedAt:                lmsPricing.UpdatedAt,
	}
}

func (r *CourseRepositoryImpl) domainReviewToModel(review *course.Review) *models.LmsReview {
	return &models.LmsReview{
		ID:        review.ID,
		CourseID:  review.CourseID,
		UserID:    review.UserID,
		Rating:    review.Rating,
		Comment:   review.Comment,
		Status:    review.Status,
		Reply:     review.Reply,
		CreatedAt: review.CreatedAt,
		UpdatedAt: review.UpdatedAt,
	}
}

func (r *CourseRepositoryImpl) modelReviewToDomain(lmsReview *models.LmsReview) *course.Review {
	return &course.Review{
		ID:        lmsReview.ID,
		CourseID:  lmsReview.CourseID,
		UserID:    lmsReview.UserID,
		Rating:    lmsReview.Rating,
		Comment:   lmsReview.Comment,
		Status:    lmsReview.Status,
		Reply:     lmsReview.Reply,
		CreatedAt: lmsReview.CreatedAt,
		UpdatedAt: lmsReview.UpdatedAt,
	}
}

func (r *CourseRepositoryImpl) domainCertificateToModel(certificate *course.Certificate) *models.LmsCertificate {
	return &models.LmsCertificate{
		ID:            certificate.ID,
		CourseID:      certificate.CourseID,
		UserID:        certificate.UserID,
		CertificateNo: certificate.CertificateNo,
		QRCodeURL:     certificate.QRCodeURL,
		PDFURL:        certificate.PDFURL,
		IssuedAt:      certificate.IssuedAt,
		CreatedAt:     certificate.CreatedAt,
	}
}

func (r *CourseRepositoryImpl) modelCertificateToDomain(lmsCertificate *models.LmsCertificate) *course.Certificate {
	return &course.Certificate{
		ID:            lmsCertificate.ID,
		CourseID:      lmsCertificate.CourseID,
		UserID:        lmsCertificate.UserID,
		CertificateNo: lmsCertificate.CertificateNo,
		QRCodeURL:     lmsCertificate.QRCodeURL,
		PDFURL:        lmsCertificate.PDFURL,
		IssuedAt:      lmsCertificate.IssuedAt,
		CreatedAt:     lmsCertificate.CreatedAt,
	}
}
