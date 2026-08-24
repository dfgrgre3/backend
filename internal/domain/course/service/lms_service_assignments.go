package courseservice

import (
	models "thanawy-backend/internal/domain/common"
	"time"

	"github.com/google/uuid"
)

// ----------------------------
// Assignments
// ----------------------------

func (s *LmsService) CreateAssignment(courseID uuid.UUID, title string, description *string, dueDate *time.Time, maxScore float64) (*models.LmsAssignment, error) {
	a := &models.LmsAssignment{
		CourseID:    courseID,
		Title:       title,
		Description: description,
		DueDate:     dueDate,
		MaxScore:    maxScore,
	}
	if err := s.repo.CreateAssignment(a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *LmsService) ListCourseAssignments(courseID uuid.UUID) ([]models.LmsAssignment, error) {
	return s.repo.ListAssignmentsByCourseID(courseID)
}

func (s *LmsService) DeleteAssignment(id uuid.UUID) error {
	return s.repo.DeleteAssignment(id)
}

// LinkAssignment associates an assignment with a lesson (one lesson per
// assignment; linking a new one replaces any existing link).
func (s *LmsService) LinkAssignment(assignmentID, lessonID uuid.UUID) (*models.LmsAssignment, error) {
	if err := s.repo.LinkAssignmentToLesson(assignmentID, &lessonID); err != nil {
		return nil, err
	}
	return s.repo.GetAssignmentByID(assignmentID)
}

// UnlinkAssignment removes an assignment's lesson link, if any (the
// assignment itself is not deleted — it stays in the course's catalog).
func (s *LmsService) UnlinkAssignment(assignmentID uuid.UUID) error {
	return s.repo.LinkAssignmentToLesson(assignmentID, nil)
}
