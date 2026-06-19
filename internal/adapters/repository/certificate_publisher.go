package repository

import (
	"context"
	"thanawy-backend/internal/domain/certificate"
)

// certificateNoOpPublisher is a no-op publisher for certificate events
type certificateNoOpPublisher struct{}

func NewNoOpCertificatePublisher() certificate.EventPublisher {
	return &certificateNoOpPublisher{}
}

func (p *certificateNoOpPublisher) Publish(_ context.Context, _ certificate.CertificateEvent) error {
	return nil
}