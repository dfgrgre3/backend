package repositories

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
)

// ----------------------------
// Certificate Templates
// ----------------------------

func (r *LmsRepository) CreateCertificateTemplate(t *models.LmsCertificateTemplate) error {
	return r.db.Create(t).Error
}

func (r *LmsRepository) ListCertificateTemplates() ([]models.LmsCertificateTemplate, error) {
	var templates []models.LmsCertificateTemplate
	err := r.db.Order("is_default DESC, created_at DESC").Find(&templates).Error
	return templates, err
}

func (r *LmsRepository) DeleteCertificateTemplate(id uuid.UUID) error {
	return r.db.Delete(&models.LmsCertificateTemplate{}, "id = ?", id).Error
}

// ----------------------------
// Certificates
// ----------------------------

func (r *LmsRepository) CreateCertificate(c *models.LmsCertificate) error {
	return r.db.Create(c).Error
}

func (r *LmsRepository) GetCertificate(courseID, userID uuid.UUID) (*models.LmsCertificate, error) {
	var c models.LmsCertificate
	err := r.db.Where("course_id = ? AND user_id = ?", courseID, userID).First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *LmsRepository) ListCertificatesByUser(userID uuid.UUID) ([]models.LmsCertificate, error) {
	var certs []models.LmsCertificate
	err := r.db.Where("user_id = ?", userID).Order("issued_at DESC").Find(&certs).Error
	return certs, err
}
