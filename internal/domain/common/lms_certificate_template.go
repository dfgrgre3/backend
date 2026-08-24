package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LmsCertificateTemplate is a shared, global certificate design a course can
// pick from. Course.CertificateTemplate stores this table's id (a loose
// reference, no FK constraint — same style as Lesson.ExamID).
type LmsCertificateTemplate struct {
	ID           uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name         string         `gorm:"not null;column:name" json:"name"`
	TemplateHTML string         `gorm:"type:text;not null;column:template_html" json:"templateHtml"`
	IsDefault    bool           `gorm:"default:false;column:is_default" json:"isDefault"`
	CreatedAt    time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`
}

func (LmsCertificateTemplate) TableName() string {
	return "LmsCertificateTemplate"
}

func (t *LmsCertificateTemplate) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return
}
