package courseservice

import (
	"fmt"
	models "thanawy-backend/internal/domain/common"
	"time"

	"github.com/google/uuid"
)

// ----------------------------
// Certificate Templates
// ----------------------------

func (s *LmsService) CreateCertificateTemplate(name, templateHTML string, isDefault bool) (*models.LmsCertificateTemplate, error) {
	t := &models.LmsCertificateTemplate{
		Name:         name,
		TemplateHTML: templateHTML,
		IsDefault:    isDefault,
	}
	if err := s.repo.CreateCertificateTemplate(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *LmsService) ListCertificateTemplates() ([]models.LmsCertificateTemplate, error) {
	return s.repo.ListCertificateTemplates()
}

func (s *LmsService) DeleteCertificateTemplate(id uuid.UUID) error {
	return s.repo.DeleteCertificateTemplate(id)
}

// ----------------------------
// Certificates
// ----------------------------

// generateCertificate creates a certificate for a completed course.
func (s *LmsService) generateCertificate(courseID, userID uuid.UUID) error {
	certNo := fmt.Sprintf("CERT-%s-%d", courseID.String()[:8], time.Now().Unix())
	cert := &models.LmsCertificate{
		CourseID:      courseID,
		UserID:        userID,
		CertificateNo: certNo,
		PDFURL:        fmt.Sprintf("/certificates/%s.pdf", certNo),
	}
	return s.repo.CreateCertificate(cert)
}

func (s *LmsService) GetCertificate(courseID, userID uuid.UUID) (*models.LmsCertificate, error) {
	return s.repo.GetCertificate(courseID, userID)
}

func (s *LmsService) ListUserCertificates(userID uuid.UUID) ([]models.LmsCertificate, error) {
	return s.repo.ListCertificatesByUser(userID)
}
