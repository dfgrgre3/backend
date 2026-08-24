package repositories

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
)

// ----------------------------
// Attachments & Subtitles
// ----------------------------

func (r *LmsRepository) CreateAttachment(a *models.LmsAttachment) error {
	return r.db.Create(a).Error
}

func (r *LmsRepository) ListAttachments(lessonID uuid.UUID) ([]models.LmsAttachment, error) {
	var attachments []models.LmsAttachment
	err := r.db.Where("lesson_id = ?", lessonID).Find(&attachments).Error
	return attachments, err
}

func (r *LmsRepository) DeleteAttachment(id uuid.UUID) error {
	return r.db.Delete(&models.LmsAttachment{}, "id = ?", id).Error
}

func (r *LmsRepository) CreateSubtitle(s *models.LmsSubtitle) error {
	return r.db.Create(s).Error
}

func (r *LmsRepository) ListSubtitles(lessonID uuid.UUID) ([]models.LmsSubtitle, error) {
	var subtitles []models.LmsSubtitle
	err := r.db.Where("lesson_id = ?", lessonID).Find(&subtitles).Error
	return subtitles, err
}

func (r *LmsRepository) DeleteSubtitle(id uuid.UUID) error {
	return r.db.Delete(&models.LmsSubtitle{}, "id = ?", id).Error
}
