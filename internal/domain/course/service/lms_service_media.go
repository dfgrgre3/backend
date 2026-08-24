package courseservice

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
)

// ----------------------------
// Attachments & Subtitles
// ----------------------------

func (s *LmsService) AddAttachment(lessonID uuid.UUID, title, fileURL, fileType string, fileSize *int64) (*models.LmsAttachment, error) {
	a := &models.LmsAttachment{
		LessonID: lessonID,
		Title:    title,
		FileURL:  fileURL,
		FileType: fileType,
		FileSize: fileSize,
	}
	if err := s.repo.CreateAttachment(a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *LmsService) ListAttachments(lessonID uuid.UUID) ([]models.LmsAttachment, error) {
	return s.repo.ListAttachments(lessonID)
}

func (s *LmsService) DeleteAttachment(id uuid.UUID) error {
	return s.repo.DeleteAttachment(id)
}

func (s *LmsService) AddSubtitle(lessonID uuid.UUID, language, vttURL string) (*models.LmsSubtitle, error) {
	sub := &models.LmsSubtitle{
		LessonID: lessonID,
		Language: language,
		VTTURL:   vttURL,
	}
	if err := s.repo.CreateSubtitle(sub); err != nil {
		return nil, err
	}
	return sub, nil
}

func (s *LmsService) ListSubtitles(lessonID uuid.UUID) ([]models.LmsSubtitle, error) {
	return s.repo.ListSubtitles(lessonID)
}
