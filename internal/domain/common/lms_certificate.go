package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LmsCertificate stores completion certificates
type LmsCertificate struct {
	ID            uuid.UUID `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	CourseID      uuid.UUID `gorm:"not null;type:uuid;index;column:course_id;constraint:OnDelete:CASCADE" json:"courseId"`
	UserID        uuid.UUID `gorm:"not null;type:uuid;index:idx_lms_cert_user_course,unique;column:user_id;constraint:OnDelete:CASCADE" json:"userId"`
	CertificateNo string    `gorm:"uniqueIndex;not null;column:certificate_no" json:"certificateNo"`
	QRCodeURL     *string   `gorm:"column:qr_code_url" json:"qrCodeUrl,omitempty"`
	PDFURL        string    `gorm:"not null;column:pdf_url" json:"pdfUrl"`
	IssuedAt      time.Time `gorm:"index;column:issued_at" json:"issuedAt"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (LmsCertificate) TableName() string {
	return "LmsCertificate"
}

func (cert *LmsCertificate) BeforeCreate(tx *gorm.DB) (err error) {
	if cert.ID == uuid.Nil {
		cert.ID = uuid.New()
	}
	if cert.IssuedAt.IsZero() {
		cert.IssuedAt = time.Now()
	}
	return
}
