package repositories

import (
	"context"
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Type aliases for domain types
type Course = models.Course
type CourseFilter = models.CourseFilter
type CourseStatus = models.CourseStatus
type CourseLevel = models.CourseLevel
type Section = models.Section
type Lesson = models.Lesson
type LessonType = models.LessonType
type AvailabilityType = models.AvailabilityType
type Enrollment = models.DomainEnrollment
type EnrollmentFilter = models.EnrollmentFilter
type Pricing = models.Pricing
type PricingType = models.DomainPricingType
type Category = models.DomainCategory
type Instructor = models.Instructor
type Review = models.Review
type Certificate = models.Certificate
type CourseVersion = models.DomainCourseVersion
type CourseChangelog = models.CourseChangelog
type PGStringArray = models.PGStringArray

// Helper function to parse UUID
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// GormRepository implements the Repository interface using GORM
type GormRepository struct {
	repo *LmsRepository
}

// NewGormRepository creates a new GormRepository
func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{
		repo: NewLmsRepository(db),
	}
}

// Course operations
func (r *GormRepository) CreateCourse(ctx context.Context, course *Course) error {
	return r.repo.CreateCourse(r.toModelCourse(course))
}

func (r *GormRepository) GetCourseByID(ctx context.Context, id string) (*Course, error) {
	courseUUID, err := parseUUID(id)
	if err != nil {
		return nil, err
	}
	modelCourse, err := r.repo.GetCourseByID(courseUUID)
	if err != nil {
		return nil, err
	}
	return r.toDomainCourse(modelCourse), nil
}

func (r *GormRepository) GetCourseBySlug(ctx context.Context, slug string) (*Course, error) {
	modelCourse, err := r.repo.GetCourseBySlug(slug)
	if err != nil {
		return nil, err
	}
	return r.toDomainCourse(modelCourse), nil
}

func (r *GormRepository) UpdateCourse(ctx context.Context, course *Course) error {
	return r.repo.UpdateCourse(r.toModelCourse(course))
}

func (r *GormRepository) DeleteCourse(ctx context.Context, id string) error {
	courseUUID, err := parseUUID(id)
	if err != nil {
		return err
	}
	return r.repo.DeleteCourse(courseUUID)
}

func (r *GormRepository) ListCourses(ctx context.Context, filter CourseFilter) ([]*Course, int, error) {
	var status, level, language *string
	if filter.Status != nil {
		s := string(*filter.Status)
		status = &s
	}
	if filter.Level != nil {
		l := string(*filter.Level)
		level = &l
	}
	if filter.Language != nil {
		language = filter.Language
	}

	modelCourses, total, err := r.repo.ListCourses(filter.Page, filter.Limit,
		derefString(status), derefString(level), derefString(language))
	if err != nil {
		return nil, 0, err
	}

	courses := make([]*Course, len(modelCourses))
	for i, mc := range modelCourses {
		courses[i] = r.toDomainCourse(&mc)
	}
	return courses, int(total), nil
}

// Section operations
func (r *GormRepository) CreateSection(ctx context.Context, section *Section) error {
	return r.repo.CreateSection(r.toModelSection(section))
}

func (r *GormRepository) GetSectionByID(ctx context.Context, id string) (*Section, error) {
	sectionUUID, err := parseUUID(id)
	if err != nil {
		return nil, err
	}
	modelSection, err := r.repo.GetSectionByID(sectionUUID)
	if err != nil {
		return nil, err
	}
	return r.toDomainSection(modelSection), nil
}

func (r *GormRepository) UpdateSection(ctx context.Context, section *Section) error {
	return r.repo.UpdateSection(r.toModelSection(section))
}

func (r *GormRepository) DeleteSection(ctx context.Context, id string) error {
	sectionUUID, err := parseUUID(id)
	if err != nil {
		return err
	}
	return r.repo.DeleteSection(sectionUUID)
}

func (r *GormRepository) ListSections(ctx context.Context, courseID string) ([]*Section, error) {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	modelSections, err := r.repo.ListSectionsByCourseID(courseUUID)
	if err != nil {
		return nil, err
	}

	sections := make([]*Section, len(modelSections))
	for i, ms := range modelSections {
		sections[i] = r.toDomainSection(&ms)
	}
	return sections, nil
}

func (r *GormRepository) ReorderSections(ctx context.Context, courseID string, sectionIDs []string) error {
	// Implementation would update order_index for each section
	return nil
}

// Lesson operations
func (r *GormRepository) CreateLesson(ctx context.Context, lesson *Lesson) error {
	return r.repo.CreateLesson(r.toModelLesson(lesson))
}

func (r *GormRepository) GetLessonByID(ctx context.Context, id string) (*Lesson, error) {
	lessonUUID, err := parseUUID(id)
	if err != nil {
		return nil, err
	}
	modelLesson, err := r.repo.GetLessonByID(lessonUUID)
	if err != nil {
		return nil, err
	}
	return r.toDomainLesson(modelLesson), nil
}

func (r *GormRepository) UpdateLesson(ctx context.Context, lesson *Lesson) error {
	return r.repo.UpdateLesson(r.toModelLesson(lesson))
}

func (r *GormRepository) DeleteLesson(ctx context.Context, id string) error {
	lessonUUID, err := parseUUID(id)
	if err != nil {
		return err
	}
	return r.repo.DeleteLesson(lessonUUID)
}

func (r *GormRepository) ListLessons(ctx context.Context, sectionID string) ([]*Lesson, error) {
	sectionUUID, err := parseUUID(sectionID)
	if err != nil {
		return nil, err
	}
	modelLessons, err := r.repo.ListLessonsBySectionID(sectionUUID)
	if err != nil {
		return nil, err
	}

	lessons := make([]*Lesson, len(modelLessons))
	for i, ml := range modelLessons {
		lessons[i] = r.toDomainLesson(&ml)
	}
	return lessons, nil
}

func (r *GormRepository) ReorderLessons(ctx context.Context, sectionID string, lessonIDs []string) error {
	// Implementation would update order_index for each lesson
	return nil
}

// Enrollment operations
func (r *GormRepository) CreateEnrollment(ctx context.Context, enrollment *Enrollment) error {
	return r.repo.CreateEnrollment(r.toModelEnrollment(enrollment))
}

func (r *GormRepository) GetEnrollment(ctx context.Context, courseID, userID string) (*Enrollment, error) {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	userUUID, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	modelEnrollment, err := r.repo.GetEnrollment(courseUUID, userUUID)
	if err != nil {
		return nil, err
	}
	return r.toDomainEnrollment(modelEnrollment), nil
}

func (r *GormRepository) UpdateEnrollmentProgress(ctx context.Context, enrollment *Enrollment) error {
	courseUUID, err := parseUUID(enrollment.CourseID.String())
	if err != nil {
		return err
	}
	userUUID, err := parseUUID(enrollment.UserID.String())
	if err != nil {
		return err
	}
	return r.repo.UpdateEnrollmentProgress(courseUUID, userUUID, float64(enrollment.Progress))
}

func (r *GormRepository) ListEnrollments(ctx context.Context, filter EnrollmentFilter) ([]*Enrollment, int, error) {
	return nil, 0, nil
}

// Pricing operations
func (r *GormRepository) CreatePricing(ctx context.Context, pricing *Pricing) error {
	return r.repo.CreatePricing(r.toModelPricing(pricing))
}

func (r *GormRepository) GetPricing(ctx context.Context, courseID string) (*Pricing, error) {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	modelPricings, err := r.repo.ListPricingsByCourseID(courseUUID)
	if err != nil || len(modelPricings) == 0 {
		return nil, err
	}
	return r.toDomainPricing(&modelPricings[0]), nil
}

func (r *GormRepository) UpdatePricing(ctx context.Context, pricing *Pricing) error {
	return r.repo.UpdatePricing(r.toModelPricing(pricing))
}

func (r *GormRepository) DeletePricing(ctx context.Context, courseID string) error {
	return nil
}

// Category operations
func (r *GormRepository) AddCourseCategory(ctx context.Context, courseID, categoryID string) error {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return err
	}
	catUUID, err := parseUUID(categoryID)
	if err != nil {
		return err
	}
	return r.repo.SetCourseCategories(courseUUID, []uuid.UUID{catUUID})
}

func (r *GormRepository) RemoveCourseCategory(ctx context.Context, courseID, categoryID string) error {
	return nil
}

func (r *GormRepository) ListCourseCategories(ctx context.Context, courseID string) ([]*Category, error) {
	return nil, nil
}

// Instructor operations
func (r *GormRepository) AddCourseInstructor(ctx context.Context, courseID, instructorID string, role string) error {
	return nil
}

func (r *GormRepository) RemoveCourseInstructor(ctx context.Context, courseID, instructorID string) error {
	return nil
}

func (r *GormRepository) ListCourseInstructors(ctx context.Context, courseID string) ([]*Instructor, error) {
	return nil, nil
}

// Review operations
func (r *GormRepository) CreateReview(ctx context.Context, review *Review) error {
	return nil
}

func (r *GormRepository) GetReview(ctx context.Context, courseID, userID string) (*Review, error) {
	return nil, nil
}

func (r *GormRepository) UpdateReview(ctx context.Context, review *Review) error {
	return nil
}

func (r *GormRepository) ListReviews(ctx context.Context, courseID string) ([]*Review, error) {
	return nil, nil
}

// Certificate operations
func (r *GormRepository) CreateCertificate(ctx context.Context, certificate *Certificate) error {
	return nil
}

func (r *GormRepository) GetCertificate(ctx context.Context, courseID, userID string) (*Certificate, error) {
	return nil, nil
}

func (r *GormRepository) ListCertificates(ctx context.Context, userID string) ([]*Certificate, error) {
	return nil, nil
}

// Course versioning & cloning
func (r *GormRepository) CloneCourse(ctx context.Context, courseID string, newTitle string) (*Course, error) {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	modelCourse, err := r.repo.CloneCourse(courseUUID, newTitle)
	if err != nil {
		return nil, err
	}
	return r.toDomainCourse(modelCourse), nil
}

func (r *GormRepository) CreateVersion(ctx context.Context, courseID string, userID string) (*CourseVersion, error) {
	return nil, nil
}

func (r *GormRepository) ListVersions(ctx context.Context, courseID string) ([]*CourseVersion, error) {
	return nil, nil
}

func (r *GormRepository) RestoreVersion(ctx context.Context, courseID string, versionNumber int, userID string) (*Course, error) {
	return nil, nil
}

func (r *GormRepository) GetChangelog(ctx context.Context, courseID string) ([]*CourseChangelog, error) {
	return nil, nil
}

// Helper functions for conversion between domain and model
func (r *GormRepository) toModelCourse(c *Course) *models.LmsCourse {
	return &models.LmsCourse{
		ID:                    c.ID,
		Title:                 c.Title,
		Slug:                  c.Slug,
		ShortDescription:      c.ShortDescription,
		LongDescription:       c.LongDescription,
		CoverImageURL:         c.CoverImageURL,
		PromoVideoURL:         c.PromoVideoURL,
		Status:                models.CourseStatus(c.Status),
		Level:                 models.CourseLevel(c.Level),
		Language:              c.Language,
		EstimatedDurationMins: c.EstimatedDurationMins,
		HasCertificate:        c.HasCertificate,
		CertificateTemplate:   c.CertificateTemplate,
		MaxStudents:           c.MaxStudents,
		Version:               c.Version,
		IsFeatured:            c.IsFeatured,
		IsTrending:            c.IsTrending,
		IsNew:                 c.IsNew,
		NewFrom:               c.NewFrom,
		NewUntil:              c.NewUntil,
		SEOTitle:              c.SEOTitle,
		SEODescription:        c.SEODescription,
		SEOKeywords:           c.SEOKeywords,
		PrerequisitesText:     c.PrerequisitesText,
		TargetAudience:        c.TargetAudience,
		LearningOutcomes:      c.LearningOutcomes,
		PrimaryInstructorID:   c.PrimaryInstructorID,
		AvailableFrom:         c.AvailableFrom,
		AvailableUntil:        c.AvailableUntil,
		CreatedAt:             c.CreatedAt,
		UpdatedAt:             c.UpdatedAt,
	}
}

func (r *GormRepository) toDomainCourse(m *models.LmsCourse) *Course {
	seoKeywords := []string(m.SEOKeywords)
	learningOutcomes := []string(m.LearningOutcomes)

	return &Course{
		ID:                    m.ID,
		Title:                 m.Title,
		Slug:                  m.Slug,
		ShortDescription:      m.ShortDescription,
		LongDescription:       m.LongDescription,
		CoverImageURL:         m.CoverImageURL,
		PromoVideoURL:         m.PromoVideoURL,
		Status:                CourseStatus(m.Status),
		Level:                 CourseLevel(m.Level),
		Language:              m.Language,
		EstimatedDurationMins: m.EstimatedDurationMins,
		HasCertificate:        m.HasCertificate,
		CertificateTemplate:   m.CertificateTemplate,
		MaxStudents:           m.MaxStudents,
		Version:               m.Version,
		IsFeatured:            m.IsFeatured,
		IsTrending:            m.IsTrending,
		IsNew:                 m.IsNew,
		NewFrom:               m.NewFrom,
		NewUntil:              m.NewUntil,
		SEOTitle:              m.SEOTitle,
		SEODescription:        m.SEODescription,
		SEOKeywords:           seoKeywords,
		PrerequisitesText:     m.PrerequisitesText,
		TargetAudience:        m.TargetAudience,
		LearningOutcomes:      learningOutcomes,
		PrimaryInstructorID:   m.PrimaryInstructorID,
		AvailableFrom:         m.AvailableFrom,
		AvailableUntil:        m.AvailableUntil,
		CreatedAt:             m.CreatedAt,
		UpdatedAt:             m.UpdatedAt,
	}
}

func (r *GormRepository) toModelSection(s *Section) *models.LmsSection {
	return &models.LmsSection{
		ID:            s.ID,
		CourseID:      s.CourseID,
		Title:         s.Title,
		OrderIndex:    s.OrderIndex,
		AvailableFrom: s.AvailableFrom,
		DripDelayDays: s.DripDelayDays,
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
	}
}

func (r *GormRepository) toDomainSection(m *models.LmsSection) *Section {
	return &Section{
		ID:            m.ID,
		CourseID:      m.CourseID,
		Title:         m.Title,
		OrderIndex:    m.OrderIndex,
		AvailableFrom: m.AvailableFrom,
		DripDelayDays: m.DripDelayDays,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

func (r *GormRepository) toModelLesson(l *Lesson) *models.LmsLesson {
	return &models.LmsLesson{
		ID:               l.ID,
		SectionID:        l.SectionID,
		Title:            l.Title,
		Type:             models.LessonType(l.Type),
		Content:          l.Content,
		MediaURL:         l.MediaURL,
		DurationSeconds:  l.DurationSeconds,
		IsFreePreview:    l.IsFreePreview,
		OrderIndex:       l.OrderIndex,
		AvailabilityType: models.AvailabilityType(l.AvailabilityType),
		AvailableFrom:    l.AvailableFrom,
		DripDelayDays:    l.DripDelayDays,
		CreatedAt:        l.CreatedAt,
		UpdatedAt:        l.UpdatedAt,
	}
}

func (r *GormRepository) toDomainLesson(m *models.LmsLesson) *Lesson {
	return &Lesson{
		ID:               m.ID,
		SectionID:        m.SectionID,
		Title:            m.Title,
		Type:             LessonType(m.Type),
		Content:          m.Content,
		MediaURL:         m.MediaURL,
		DurationSeconds:  m.DurationSeconds,
		IsFreePreview:    m.IsFreePreview,
		OrderIndex:       m.OrderIndex,
		AvailabilityType: AvailabilityType(m.AvailabilityType),
		AvailableFrom:    m.AvailableFrom,
		DripDelayDays:    m.DripDelayDays,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
}

func (r *GormRepository) toModelEnrollment(e *Enrollment) *models.LmsEnrollment {
	progress := decimal.NewFromFloat(e.Progress)
	return &models.LmsEnrollment{
		ID:          e.ID,
		CourseID:    e.CourseID,
		UserID:      e.UserID,
		Progress:    progress,
		EnrolledAt:  e.EnrolledAt,
		CompletedAt: e.CompletedAt,
		BundleID:    e.BundleID,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

func (r *GormRepository) toDomainEnrollment(m *models.LmsEnrollment) *Enrollment {
	progress, _ := m.Progress.Float64()
	return &Enrollment{
		ID:          m.ID,
		CourseID:    m.CourseID,
		UserID:      m.UserID,
		Progress:    progress,
		EnrolledAt:  m.EnrolledAt,
		CompletedAt: m.CompletedAt,
		BundleID:    m.BundleID,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func (r *GormRepository) toModelPricing(p *Pricing) *models.LmsPricing {
	var discountPrice *decimal.Decimal
	if p.DiscountPrice != nil {
		d := decimal.NewFromFloat(*p.DiscountPrice)
		discountPrice = &d
	}
	return &models.LmsPricing{
		ID:                       p.ID,
		CourseID:                 p.CourseID,
		Type:                     models.PriceType(p.Type),
		Amount:                   decimal.NewFromFloat(p.Amount),
		CurrencyCode:             p.CurrencyCode,
		SubscriptionDurationDays: p.SubscriptionDurationDays,
		DiscountPrice:            discountPrice,
		DiscountStartAt:          p.DiscountStartAt,
		DiscountEndAt:            p.DiscountEndAt,
		SubscriptionPlanID:       p.SubscriptionPlanID,
		IsActive:                 p.IsActive,
		CreatedAt:                p.CreatedAt,
		UpdatedAt:                p.UpdatedAt,
	}
}

func (r *GormRepository) toDomainPricing(m *models.LmsPricing) *Pricing {
	amount, _ := m.Amount.Float64()
	var discountPrice *float64
	if m.DiscountPrice != nil {
		d, _ := m.DiscountPrice.Float64()
		discountPrice = &d
	}
	return &Pricing{
		ID:                       m.ID,
		CourseID:                 m.CourseID,
		Type:                     PricingType(m.Type),
		Amount:                   amount,
		CurrencyCode:             m.CurrencyCode,
		SubscriptionDurationDays: m.SubscriptionDurationDays,
		DiscountPrice:            discountPrice,
		DiscountStartAt:          m.DiscountStartAt,
		DiscountEndAt:            m.DiscountEndAt,
		SubscriptionPlanID:       m.SubscriptionPlanID,
		IsActive:                 m.IsActive,
		CreatedAt:                m.CreatedAt,
		UpdatedAt:                m.UpdatedAt,
	}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
