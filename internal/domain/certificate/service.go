package certificate

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrCertificateNotFound = errors.New("certificate not found")
	ErrAlreadyExists       = errors.New("certificate already issued for this course")
	ErrNotEligible         = errors.New("course not completed yet")
)

type Service struct {
	repo      Repository
	publisher EventPublisher
}

func NewService(repo Repository, publisher EventPublisher) *Service {
	return &Service{
		repo:      repo,
		publisher: publisher,
	}
}

// IssueCertificate creates a new certificate for a user who completed a course
func (s *Service) IssueCertificate(ctx context.Context, input IssueCertificateInput) (*Certificate, error) {
	if input.UserID == "" || input.SubjectID == "" {
		return nil, ErrCertificateNotFound
	}

	// Check if already exists
	exists, err := s.repo.HasCertificate(ctx, input.UserID, input.SubjectID)
	if err != nil {
		return nil, fmt.Errorf("check certificate: %w", err)
	}
	if exists {
		return nil, ErrAlreadyExists
	}

	now := time.Now()
	cert := &Certificate{
		UserID:    input.UserID,
		SubjectID: input.SubjectID,
		IssuedAt:  now,
		CreatedAt: now,
		Metadata: map[string]interface{}{
			"issuedAt": now.Format(time.RFC3339),
		},
	}

	if err := s.repo.Create(ctx, cert); err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}

	// Publish event
	s.publishEvent(ctx, CertificateEvent{
		Type:      EventCertificateIssued,
		CertificateID: cert.ID,
		UserID:    input.UserID,
		SubjectID: input.SubjectID,
		Data: map[string]interface{}{
			"issuedAt": now,
		},
	})

	return cert, nil
}

// GetCertificate returns a certificate by ID with full details
func (s *Service) GetCertificate(ctx context.Context, id string) (*CertificateWithDetails, error) {
	details, err := s.repo.GetWithDetails(ctx, id)
	if err != nil {
		return nil, ErrCertificateNotFound
	}
	return details, nil
}

// GetUserCertificate returns a certificate for a user in a specific course
func (s *Service) GetUserCertificate(ctx context.Context, userID string, subjectID string) (*Certificate, error) {
	result, err := s.repo.List(ctx, ListCertificatesFilter{
		UserID:    &userID,
		SubjectID: &subjectID,
		Page:      1,
		Limit:     1,
	})
	if err != nil {
		return nil, err
	}
	if len(result.Certificates) == 0 {
		return nil, ErrCertificateNotFound
	}
	return &result.Certificates[0], nil
}

// ListUserCertificates returns all certificates for a user
func (s *Service) ListUserCertificates(ctx context.Context, userID string, page, limit int) (ListCertificatesResult, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	return s.repo.List(ctx, ListCertificatesFilter{
		UserID: &userID,
		Page:   page,
		Limit:  limit,
	})
}

// CountUserCertificates returns total certificates count for a user
func (s *Service) CountUserCertificates(ctx context.Context, userID string) (int64, error) {
	return s.repo.CountByUser(ctx, userID)
}

// ============================================================================
// Event Publishing
// ============================================================================

func (s *Service) publishEvent(ctx context.Context, event CertificateEvent) {
	event.Timestamp = time.Now()
	if s.publisher != nil {
		_ = s.publisher.Publish(ctx, event)
	}
}