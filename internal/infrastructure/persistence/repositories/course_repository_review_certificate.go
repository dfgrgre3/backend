package repositories

import (
	"context"
	"fmt"
	models "thanawy-backend/internal/domain/common"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Review operations
func (r *GormRepository) CreateReview(ctx context.Context, review *Review) error {
	if review == nil {
		return fmt.Errorf("review is required")
	}
	if review.CourseID == uuid.Nil || review.UserID == uuid.Nil {
		return fmt.Errorf("course and user are required")
	}
	if review.Rating < 1 || review.Rating > 5 {
		return fmt.Errorf("rating must be between 1 and 5")
	}
	model := &models.LmsReview{
		ID: review.ID, CourseID: review.CourseID, UserID: review.UserID,
		Rating: review.Rating, Comment: review.Comment, Status: review.Status,
		Reply: review.Reply, CreatedAt: review.CreatedAt, UpdatedAt: review.UpdatedAt,
	}
	if model.Status == "" {
		model.Status = "PENDING"
	}
	if model.CreatedAt.IsZero() {
		model.CreatedAt = time.Now()
	}
	if model.UpdatedAt.IsZero() {
		model.UpdatedAt = model.CreatedAt
	}
	if err := r.repo.db.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}
	review.ID, review.Status, review.CreatedAt, review.UpdatedAt = model.ID, model.Status, model.CreatedAt, model.UpdatedAt
	return nil
}

func (r *GormRepository) GetReview(ctx context.Context, courseID, userID string) (*Review, error) {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	userUUID, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	var model models.LmsReview
	if err := r.repo.db.WithContext(ctx).Where("course_id = ? AND user_id = ?", courseUUID, userUUID).First(&model).Error; err != nil {
		return nil, err
	}
	return reviewFromModel(&model), nil
}

func (r *GormRepository) UpdateReview(ctx context.Context, review *Review) error {
	if review == nil || review.ID == uuid.Nil {
		return fmt.Errorf("review with id is required")
	}
	if review.Rating < 1 || review.Rating > 5 {
		return fmt.Errorf("rating must be between 1 and 5")
	}
	updates := map[string]interface{}{"rating": review.Rating, "comment": review.Comment, "updated_at": time.Now()}
	if review.Status != "" {
		updates["status"] = review.Status
	}
	if review.Reply != nil {
		updates["reply"] = review.Reply
	}
	result := r.repo.db.WithContext(ctx).Model(&models.LmsReview{}).Where("id = ? AND deleted_at IS NULL", review.ID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	review.UpdatedAt = updates["updated_at"].(time.Time)
	return nil
}

func (r *GormRepository) ListReviews(ctx context.Context, courseID string) ([]*Review, error) {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	var modelsList []models.LmsReview
	if err := r.repo.db.WithContext(ctx).Where("course_id = ? AND deleted_at IS NULL", courseUUID).
		Order("created_at DESC").Find(&modelsList).Error; err != nil {
		return nil, err
	}
	result := make([]*Review, len(modelsList))
	for i := range modelsList {
		result[i] = reviewFromModel(&modelsList[i])
	}
	return result, nil
}

// Certificate operations
func (r *GormRepository) CreateCertificate(ctx context.Context, certificate *Certificate) error {
	if certificate == nil || certificate.CourseID == uuid.Nil || certificate.UserID == uuid.Nil {
		return fmt.Errorf("certificate, course and user are required")
	}
	model := &models.LmsCertificate{
		ID: certificate.ID, CourseID: certificate.CourseID, UserID: certificate.UserID,
		CertificateNo: certificate.CertificateNo, QRCodeURL: certificate.QRCodeURL,
		PDFURL: certificate.PDFURL, IssuedAt: certificate.IssuedAt, CreatedAt: certificate.CreatedAt,
	}
	if model.CertificateNo == "" {
		model.CertificateNo = "CERT-" + uuid.NewString()
	}
	if model.IssuedAt.IsZero() {
		model.IssuedAt = time.Now()
	}
	if model.CreatedAt.IsZero() {
		model.CreatedAt = model.IssuedAt
	}
	if err := r.repo.db.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}
	certificate.ID, certificate.CertificateNo, certificate.IssuedAt, certificate.CreatedAt = model.ID, model.CertificateNo, model.IssuedAt, model.CreatedAt
	return nil
}

func (r *GormRepository) GetCertificate(ctx context.Context, courseID, userID string) (*Certificate, error) {
	courseUUID, err := parseUUID(courseID)
	if err != nil {
		return nil, err
	}
	userUUID, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	var model models.LmsCertificate
	if err := r.repo.db.WithContext(ctx).Where("course_id = ? AND user_id = ?", courseUUID, userUUID).First(&model).Error; err != nil {
		return nil, err
	}
	return certificateFromModel(&model), nil
}

func (r *GormRepository) ListCertificates(ctx context.Context, userID string) ([]*Certificate, error) {
	userUUID, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	var modelsList []models.LmsCertificate
	if err := r.repo.db.WithContext(ctx).Where("user_id = ?", userUUID).
		Order("issued_at DESC").Find(&modelsList).Error; err != nil {
		return nil, err
	}
	result := make([]*Certificate, len(modelsList))
	for i := range modelsList {
		result[i] = certificateFromModel(&modelsList[i])
	}
	return result, nil
}

func reviewFromModel(model *models.LmsReview) *Review {
	return &Review{ID: model.ID, CourseID: model.CourseID, UserID: model.UserID, Rating: model.Rating, Comment: model.Comment, Status: model.Status, Reply: model.Reply, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt}
}

func certificateFromModel(model *models.LmsCertificate) *Certificate {
	return &Certificate{ID: model.ID, CourseID: model.CourseID, UserID: model.UserID, CertificateNo: model.CertificateNo, QRCodeURL: model.QRCodeURL, PDFURL: model.PDFURL, IssuedAt: model.IssuedAt, CreatedAt: model.CreatedAt}
}
