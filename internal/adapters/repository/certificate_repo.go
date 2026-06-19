package repository

import (
	"context"
	"time"

	"thanawy-backend/internal/domain/certificate"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type certificateRepository struct {
	db *gorm.DB
}

func NewCertificateRepository(database *gorm.DB) certificate.Repository {
	return &certificateRepository{db: database}
}

// ============================================================================
// Certificate CRUD
// ============================================================================

func (r *certificateRepository) Create(ctx context.Context, cert *certificate.Certificate) error {
	if cert.ID == "" {
		cert.ID = uuid.New().String()
	}

	record := &certificateRecord{
		ID:        cert.ID,
		UserID:    cert.UserID,
		SubjectID: cert.SubjectID,
		IssuedAt:  cert.IssuedAt,
		CreatedAt: cert.CreatedAt,
	}

	return r.db.WithContext(ctx).Create(record).Error
}

func (r *certificateRepository) FindByID(ctx context.Context, id string) (*certificate.Certificate, error) {
	var record certificateRecord
	if err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&record).Error; err != nil {
		return nil, err
	}
	return record.toDomain(), nil
}

func (r *certificateRepository) List(ctx context.Context, filter certificate.ListCertificatesFilter) (certificate.ListCertificatesResult, error) {
	query := r.db.WithContext(ctx).Model(&certificateRecord{})

	if filter.UserID != nil {
		query = query.Where("user_id = ?", *filter.UserID)
	}
	if filter.SubjectID != nil {
		query = query.Where("subject_id = ?", *filter.SubjectID)
	}

	var total int64
	query.Count(&total)

	var records []certificateRecord
	if err := query.Order("issued_at DESC").
		Limit(filter.Limit).
		Offset((filter.Page - 1) * filter.Limit).
		Find(&records).Error; err != nil {
		return certificate.ListCertificatesResult{}, err
	}

	certs := make([]certificate.Certificate, len(records))
	for i, rec := range records {
		certs[i] = *rec.toDomain()
	}

	totalPages := int64(0)
	if total > 0 {
		totalPages = (total + int64(filter.Limit) - 1) / int64(filter.Limit)
	}

	return certificate.ListCertificatesResult{
		Certificates: certs,
		Total:        total,
		Page:         filter.Page,
		Limit:        filter.Limit,
		TotalPages:   totalPages,
	}, nil
}

func (r *certificateRepository) HasCertificate(ctx context.Context, userID string, subjectID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&certificateRecord{}).
		Where("user_id = ? AND subject_id = ?", userID, subjectID).
		Count(&count).Error
	return count > 0, err
}

func (r *certificateRepository) GetWithDetails(ctx context.Context, id string) (*certificate.CertificateWithDetails, error) {
	var record certificateRecord
	if err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Subject").
		Where("id = ?", id).
		First(&record).Error; err != nil {
		return nil, err
	}

	details := &certificate.CertificateWithDetails{
		Certificate: *record.toDomain(),
		User: certificate.UserInfo{
			ID:    record.User.ID,
			Name:  record.User.Name,
			Email: record.User.Email,
			Avatar: record.User.Avatar,
		},
		Subject: certificate.SubjectInfo{
			ID:             record.Subject.ID,
			Name:           record.Subject.Name,
			NameAr:         record.Subject.NameAr,
			InstructorName: record.Subject.InstructorName,
			InstructorId:   record.Subject.InstructorId,
			ThumbnailUrl:   record.Subject.ThumbnailUrl,
		},
	}

	// Set user-friendly fields on the certificate
	details.Certificate.UserName = record.User.Name
	details.Certificate.SubjectName = record.Subject.Name
	details.Certificate.SubjectNameAr = record.Subject.NameAr
	details.Certificate.InstructorName = record.Subject.InstructorName

	return details, nil
}

func (r *certificateRepository) CountByUser(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&certificateRecord{}).
		Where("user_id = ?", userID).
		Count(&count).Error
	return count, err
}

// ============================================================================
// Database Records
// ============================================================================

type certificateRecord struct {
	ID        string               `gorm:"column:id;primaryKey;type:uuid"`
	UserID    string               `gorm:"column:user_id"`
	SubjectID string               `gorm:"column:subject_id"`
	IssuedAt  time.Time            `gorm:"column:issued_at"`
	CreatedAt time.Time            `gorm:"column:created_at"`
	User      userProfileRecord    `gorm:"foreignKey:UserID"`
	Subject   subjectRecord        `gorm:"foreignKey:SubjectID"`
}

func (certificateRecord) TableName() string {
	return "Certificate"
}

// ============================================================================
// Mappers
// ============================================================================

func (r *certificateRecord) toDomain() *certificate.Certificate {
	cert := &certificate.Certificate{
		ID:        r.ID,
		UserID:    r.UserID,
		SubjectID: r.SubjectID,
		IssuedAt:  r.IssuedAt,
		CreatedAt: r.CreatedAt,
		Metadata: map[string]interface{}{
			"issuedAt": r.IssuedAt.Format(time.RFC3339),
		},
	}

	// Map user info if loaded
	if r.User.ID != "" {
		cert.UserName = r.User.Name
	}

	// Map subject info if loaded
	if r.Subject.ID != "" {
		cert.SubjectName = r.Subject.Name
		cert.SubjectNameAr = r.Subject.NameAr
		cert.InstructorName = r.Subject.InstructorName
	}

	return cert
}