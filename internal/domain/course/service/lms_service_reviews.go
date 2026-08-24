package courseservice

import (
	"errors"
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
)

// ----------------------------
// Reviews
// ----------------------------

func (s *LmsService) CreateReview(courseID, userID uuid.UUID, rating int, comment string) (*models.LmsReview, error) {
	if rating < 1 || rating > 5 {
		return nil, errors.New("rating must be between 1 and 5")
	}
	rev := &models.LmsReview{
		CourseID: courseID,
		UserID:   userID,
		Rating:   rating,
		Comment:  comment,
		Status:   "PENDING",
	}
	if err := s.repo.CreateReview(rev); err != nil {
		return nil, err
	}
	return rev, nil
}

func (s *LmsService) ListReviews(courseID uuid.UUID, status string) ([]models.LmsReview, error) {
	return s.repo.ListReviews(courseID, status)
}

func (s *LmsService) UpdateReviewStatus(id uuid.UUID, status string) error {
	return s.repo.UpdateReviewStatus(id, status)
}

func (s *LmsService) ReplyToReview(id uuid.UUID, reply string) error {
	return s.repo.ReplyToReview(id, reply)
}

// ----------------------------
// Review Comments (Workflow)
// ----------------------------

func (s *LmsService) AddReviewComment(courseID, reviewerID uuid.UUID, comment, status string) (*models.LmsReviewComment, error) {
	rc := &models.LmsReviewComment{
		CourseID:   courseID,
		ReviewerID: reviewerID,
		Comment:    comment,
		Status:     status,
	}
	if err := s.repo.CreateReviewComment(rc); err != nil {
		return nil, err
	}
	return rc, nil
}

func (s *LmsService) ListReviewComments(courseID uuid.UUID) ([]models.LmsReviewComment, error) {
	return s.repo.ListReviewComments(courseID)
}
