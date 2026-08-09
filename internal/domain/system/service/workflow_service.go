package systemservice

import (
	"encoding/json"
	"errors"
	models "thanawy-backend/internal/domain/common"
	"time"

	db "thanawy-backend/internal/infrastructure/database"

	"github.com/google/uuid"
)

// WorkflowService handles course lifecycle status transitions
type WorkflowService struct{}

// NewWorkflowService creates a new WorkflowService
func NewWorkflowService() *WorkflowService {
	return &WorkflowService{}
}

// TransitionResult represents the result of a status transition
type TransitionResult struct {
	FromStatus     models.CourseStatus `json:"fromStatus"`
	ToStatus       models.CourseStatus `json:"toStatus"`
	SubjectID      string              `json:"subjectId"`
	TransitionedAt time.Time           `json:"transitionedAt"`
	Message        string              `json:"message"`
}

// ValidateTransition checks if a status transition is valid
func (s *WorkflowService) ValidateTransition(currentStatus, targetStatus models.CourseStatus) error {
	validTransitions := map[models.CourseStatus][]models.CourseStatus{
		models.CourseStatusDraft: {
			models.CourseStatusUnderReview,
			models.CourseStatusArchived,
			models.CourseStatusPublished, // Admin direct publish
		},
		models.CourseStatusUnderReview: {
			models.CourseStatusPublished,
			models.CourseStatusRejected,
			models.CourseStatusDraft, // Return to draft
		},
		models.CourseStatusRejected: {
			models.CourseStatusDraft,
			models.CourseStatusArchived,
		},
		models.CourseStatusPublished: {
			models.CourseStatusDraft,
			models.CourseStatusArchived,
		},
		models.CourseStatusArchived: {
			models.CourseStatusDraft,
		},
	}

	valid := false
	for _, allowed := range validTransitions[currentStatus] {
		if allowed == targetStatus {
			valid = true
			break
		}
	}
	if !valid {
		return errors.New("invalid status transition from " + string(currentStatus) + " to " + string(targetStatus))
	}
	return nil
}

// SubmitForReview transitions a course from DRAFT to UNDER_REVIEW
func (s *WorkflowService) SubmitForReview(subjectID, userID string) (*TransitionResult, error) {
	return s.transition(subjectID, userID, models.CourseStatusUnderReview, "SUBMIT_FOR_REVIEW")
}

// ApproveCourse transitions a course from UNDER_REVIEW to PUBLISHED
func (s *WorkflowService) ApproveCourse(subjectID, reviewerID, notes string) (*TransitionResult, error) {
	return s.transitionWithNotes(subjectID, reviewerID, models.CourseStatusPublished, "APPROVE", notes)
}

// RejectCourse transitions a course from UNDER_REVIEW to REJECTED
func (s *WorkflowService) RejectCourse(subjectID, reviewerID, reason string, reasons []models.RejectionReason) (*TransitionResult, error) {
	return s.transitionWithRejection(subjectID, reviewerID, reason, reasons)
}

// ArchiveCourse transitions a course to ARCHIVED
func (s *WorkflowService) ArchiveCourse(subjectID, userID string) (*TransitionResult, error) {
	return s.transition(subjectID, userID, models.CourseStatusArchived, "ARCHIVE")
}

// UnarchiveCourse restores a course from ARCHIVED to DRAFT
func (s *WorkflowService) UnarchiveCourse(subjectID, userID string) (*TransitionResult, error) {
	return s.transition(subjectID, userID, models.CourseStatusDraft, "UNARCHIVE")
}

// PublishCourse directly publishes (for admin override)
func (s *WorkflowService) PublishCourse(subjectID, userID string) (*TransitionResult, error) {
	return s.transition(subjectID, userID, models.CourseStatusPublished, "DIRECT_PUBLISH")
}

// DraftCourse returns a course to draft status (unpublish)
func (s *WorkflowService) DraftCourse(subjectID, userID string) (*TransitionResult, error) {
	return s.transition(subjectID, userID, models.CourseStatusDraft, "RETURN_TO_DRAFT")
}

func (s *WorkflowService) transition(subjectID, userID string, targetStatus models.CourseStatus, changeType string) (*TransitionResult, error) {
	var subject models.Subject
	if err := db.DB.First(&subject, "id = ?", subjectID).Error; err != nil {
		return nil, errors.New("course not found")
	}

	if err := s.ValidateTransition(subject.Status, targetStatus); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status":     targetStatus,
		"updated_at": now,
	}

	switch targetStatus {
	case models.CourseStatusPublished:
		updates["is_published"] = true
		updates["published_at"] = now
	case models.CourseStatusArchived:
		updates["is_published"] = false
		updates["archived_at"] = now
		updates["archived_by"] = userID
	case models.CourseStatusUnderReview:
		updates["submitted_for_review_at"] = now
	case models.CourseStatusDraft:
		updates["is_published"] = false
		updates["published_at"] = nil
		updates["archived_at"] = nil
	case models.CourseStatusRejected:
		updates["reviewed_at"] = now
		updates["reviewed_by"] = userID
	}

	if err := db.DB.Model(&subject).Updates(updates).Error; err != nil {
		return nil, err
	}

	// Record changelog
	s.recordChange(subjectID, changeType, userID, map[string]interface{}{
		"from": string(subject.Status),
		"to":   string(targetStatus),
	})

	return &TransitionResult{
		FromStatus:     subject.Status,
		ToStatus:       targetStatus,
		SubjectID:      subjectID,
		TransitionedAt: now,
		Message:        "Status changed to " + string(targetStatus),
	}, nil
}

func (s *WorkflowService) transitionWithNotes(subjectID, userID string, targetStatus models.CourseStatus, changeType, notes string) (*TransitionResult, error) {
	result, err := s.transition(subjectID, userID, targetStatus, changeType)
	if err != nil {
		return nil, err
	}

	if notes != "" {
		db.DB.Model(&models.CourseReviewSubmission{}).
			Where("subject_id = ? AND status = ?", subjectID, "PENDING").
			Update("reviewer_notes", notes)
	}

	return result, nil
}

func (s *WorkflowService) transitionWithRejection(subjectID, reviewerID, reason string, reasons []models.RejectionReason) (*TransitionResult, error) {
	result, err := s.transition(subjectID, reviewerID, models.CourseStatusRejected, "REJECT")
	if err != nil {
		return nil, err
	}

	reasonsJSON, _ := json.Marshal(reasons)
	db.DB.Model(&models.Subject{}).
		Where("id = ?", subjectID).
		Update("rejection_reason", reason)

	db.DB.Model(&models.CourseReviewSubmission{}).
		Where("subject_id = ? AND status = ?", subjectID, "PENDING").
		Updates(map[string]interface{}{
			"status":            "REJECTED",
			"reviewer_id":       reviewerID,
			"reviewer_notes":    reason,
			"rejection_reasons": reasonsJSON,
			"reviewed_at":       time.Now().UTC(),
		})

	return result, nil
}

func (s *WorkflowService) recordChange(subjectID, changeType, userID string, changes map[string]interface{}) {
	changesJSON, _ := json.Marshal(changes)
	changelog := models.SubjectChangelog{
		ID:         uuid.New().String(),
		SubjectID:  subjectID,
		Version:    time.Now().UTC().Format("2006-01-02"),
		ChangeType: changeType,
		Changes:    changesJSON,
		CreatedAt:  time.Now().UTC(),
	}
	if userID != "" {
		changelog.ChangedBy = &userID
	}
	db.DB.Create(&changelog)
}

// GetCourseWorkflowHistory returns the workflow history for a course
func (s *WorkflowService) GetCourseWorkflowHistory(subjectID string) ([]models.SubjectChangelog, error) {
	var history []models.SubjectChangelog
	if err := db.DB.Where("subject_id = ?", subjectID).
		Order("created_at DESC").
		Find(&history).Error; err != nil {
		return nil, err
	}
	return history, nil
}

// GetPendingReviewCount returns the number of courses awaiting review
func (s *WorkflowService) GetPendingReviewCount() (int64, error) {
	var count int64
	if err := db.DB.Model(&models.Subject{}).
		Where("status = ?", models.CourseStatusUnderReview).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// BulkTransition performs a status transition on multiple courses
func (s *WorkflowService) BulkTransition(subjectIDs []string, targetStatus models.CourseStatus, userID string) (int, []string, error) {
	updated := 0
	failed := []string{}

	for _, id := range subjectIDs {
		_, err := s.transition(id, userID, targetStatus, "BULK_TRANSITION")
		if err != nil {
			failed = append(failed, id)
		} else {
			updated++
		}
	}

	return updated, failed, nil
}
