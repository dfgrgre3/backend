package repositories

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/shopspring/decimal"
)

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
