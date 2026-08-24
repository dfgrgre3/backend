package repositories

import (
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
)

// ----------------------------
// Reviews
// ----------------------------

func (r *LmsRepository) CreateReview(rev *models.LmsReview) error {
	return r.db.Create(rev).Error
}

func (r *LmsRepository) ListReviews(courseID uuid.UUID, status string) ([]models.LmsReview, error) {
	var reviews []models.LmsReview
	q := r.db.Where("course_id = ?", courseID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Order("created_at DESC").Find(&reviews).Error
	return reviews, err
}

func (r *LmsRepository) UpdateReviewStatus(id uuid.UUID, status string) error {
	return r.db.Model(&models.LmsReview{}).Where("id = ?", id).Update("status", status).Error
}

func (r *LmsRepository) ReplyToReview(id uuid.UUID, reply string) error {
	return r.db.Model(&models.LmsReview{}).Where("id = ?", id).Update("reply", reply).Error
}

// ----------------------------
// Review Comments (Workflow)
// ----------------------------

func (r *LmsRepository) CreateReviewComment(rc *models.LmsReviewComment) error {
	return r.db.Create(rc).Error
}

func (r *LmsRepository) ListReviewComments(courseID uuid.UUID) ([]models.LmsReviewComment, error) {
	var comments []models.LmsReviewComment
	err := r.db.Where("course_id = ?", courseID).Order("created_at DESC").Find(&comments).Error
	return comments, err
}
