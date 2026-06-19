package certificate

import (
	"context"
	"time"
)

type Repository interface {
	// Certificate CRUD
	Create(ctx context.Context, cert *Certificate) error
	FindByID(ctx context.Context, id string) (*Certificate, error)
	List(ctx context.Context, filter ListCertificatesFilter) (ListCertificatesResult, error)
	
	// Check if certificate already exists
	HasCertificate(ctx context.Context, userID string, subjectID string) (bool, error)
	
	// Get certificate with details
	GetWithDetails(ctx context.Context, id string) (*CertificateWithDetails, error)
	
	// Count
	CountByUser(ctx context.Context, userID string) (int64, error)
}

type EventPublisher interface {
	Publish(ctx context.Context, event CertificateEvent) error
}

type CertificateEvent struct {
	Type      string
	CertificateID string
	UserID    string
	SubjectID string
	Timestamp time.Time
	Data      map[string]interface{}
}

// Event constants
const (
	EventCertificateIssued = "certificate.issued"
)