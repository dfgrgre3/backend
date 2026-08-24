package courseservice

import (
	"encoding/json"
	models "thanawy-backend/internal/domain/common"

	"github.com/google/uuid"
)

// ----------------------------
// Instructors
// ----------------------------

func (s *LmsService) AddInstructor(courseID, instructorID uuid.UUID, role string, permissions json.RawMessage) error {
	i := &models.LmsInstructor{
		CourseID:     courseID,
		InstructorID: instructorID,
		Role:         role,
		Permissions:  permissions,
	}
	return s.repo.AddInstructor(i)
}

func (s *LmsService) RemoveInstructor(courseID, instructorID uuid.UUID) error {
	return s.repo.RemoveInstructor(courseID, instructorID)
}

func (s *LmsService) ListInstructors(courseID uuid.UUID) ([]models.LmsInstructor, error) {
	return s.repo.ListInstructors(courseID)
}
