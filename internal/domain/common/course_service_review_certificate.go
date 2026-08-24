package models

import (
	"context"
	"errors"
	"time"
)

// CreateReview creates a course review
func (s *CourseService) CreateReview(ctx context.Context, review *Review) (*Review, error) {
	if err := s.repo.CreateReview(ctx, review); err != nil {
		return nil, err
	}
	return review, nil
}

// UpdateReview updates a course review
func (s *CourseService) UpdateReview(ctx context.Context, review *Review) (*Review, error) {
	if err := s.repo.UpdateReview(ctx, review); err != nil {
		return nil, err
	}
	return review, nil
}

// GetReviews gets course reviews
func (s *CourseService) GetReviews(ctx context.Context, courseID string) ([]*Review, error) {
	return s.repo.ListReviews(ctx, courseID)
}

// IssueCertificate issues a certificate
func (s *CourseService) IssueCertificate(ctx context.Context, courseID, userID string) (*Certificate, error) {
	// Check if already has certificate
	_, err := s.repo.GetCertificate(ctx, courseID, userID)
	if err == nil {
		return nil, errors.New("certificate already issued")
	}

	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	userUUID, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}

	certificate := &Certificate{
		CourseID: courseUUID,
		UserID:   userUUID,
		IssuedAt: time.Now(),
	}

	if err := s.repo.CreateCertificate(ctx, certificate); err != nil {
		return nil, err
	}
	return certificate, nil
}

// GetCertificate gets a certificate
func (s *CourseService) GetCertificate(ctx context.Context, courseID, userID string) (*Certificate, error) {
	return s.repo.GetCertificate(ctx, courseID, userID)
}

// GetUserCertificates gets all certificates for a user
func (s *CourseService) GetUserCertificates(ctx context.Context, userID string) ([]*Certificate, error) {
	return s.repo.ListCertificates(ctx, userID)
}
