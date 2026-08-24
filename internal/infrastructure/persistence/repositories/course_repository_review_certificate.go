package repositories

import (
	"context"
)

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
