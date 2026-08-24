package models

import (
	"context"
)

// SetPricing sets course pricing
func (s *CourseService) SetPricing(ctx context.Context, courseID string, pricing *Pricing) (*Pricing, error) {
	id, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	pricing.CourseID = id

	// Try to get existing pricing
	existing, err := s.repo.GetPricing(ctx, courseID)
	if err == nil {
		pricing.ID = existing.ID
		return pricing, s.repo.UpdatePricing(ctx, pricing)
	}

	return pricing, s.repo.CreatePricing(ctx, pricing)
}

// GetPricing gets course pricing
func (s *CourseService) GetPricing(ctx context.Context, courseID string) (*Pricing, error) {
	return s.repo.GetPricing(ctx, courseID)
}

// AddCategory adds a category to a course
func (s *CourseService) AddCategory(ctx context.Context, courseID, categoryID string) error {
	return s.repo.AddCourseCategory(ctx, courseID, categoryID)
}

// RemoveCategory removes a category from a course
func (s *CourseService) RemoveCategory(ctx context.Context, courseID, categoryID string) error {
	return s.repo.RemoveCourseCategory(ctx, courseID, categoryID)
}

// AddInstructor adds an instructor to a course
func (s *CourseService) AddInstructor(ctx context.Context, courseID, instructorID string, role string) error {
	return s.repo.AddCourseInstructor(ctx, courseID, instructorID, role)
}

// RemoveInstructor removes an instructor from a course
func (s *CourseService) RemoveInstructor(ctx context.Context, courseID, instructorID string) error {
	return s.repo.RemoveCourseInstructor(ctx, courseID, instructorID)
}
