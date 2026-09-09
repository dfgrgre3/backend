package models

import "time"

// EnrollmentLifecycle is the canonical state machine for a user's relation
// to a course. Progress and CompletedAt remain persistence facts; they do not
// independently represent the lifecycle in application code.
type EnrollmentLifecycle string

const (
	EnrollmentLifecycleEligible       EnrollmentLifecycle = "ELIGIBLE"
	EnrollmentLifecyclePendingPayment EnrollmentLifecycle = "PENDING_PAYMENT"
	EnrollmentLifecycleActive         EnrollmentLifecycle = "ACTIVE"
	EnrollmentLifecycleCompleted      EnrollmentLifecycle = "COMPLETED"
	EnrollmentLifecycleCancelled      EnrollmentLifecycle = "CANCELLED"
)

type CourseAccessState string

const (
	CourseAccessUnavailable CourseAccessState = "UNAVAILABLE"
	CourseAccessPreview     CourseAccessState = "PREVIEW"
	CourseAccessEnrolled    CourseAccessState = "ENROLLED"
	CourseAccessCompleted   CourseAccessState = "COMPLETED"
)

// CourseControlState is the normalized state consumed by authorization and
// learning flows. The legacy isActive/isPublished flags are inputs only.
type CourseControlState struct {
	Lifecycle           CourseStatus        `json:"lifecycle"`
	Enrollment          EnrollmentLifecycle `json:"enrollment"`
	Access              CourseAccessState   `json:"access"`
	Progress            float64             `json:"progress"`
	IsComplete          bool                `json:"isComplete"`
	CertificateEligible bool                `json:"certificateEligible"`
}

func DeriveCourseControlState(
	lifecycle CourseStatus,
	isActive, isPublished, isEnrolled bool,
	progress float64,
	completedAt *time.Time,
) CourseControlState {
	if lifecycle == "" {
		lifecycle = CourseStatusDraft
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	isComplete := completedAt != nil || progress >= 100
	isPublic := lifecycle == CourseStatusPublished && isActive && isPublished

	enrollment := EnrollmentLifecycleCancelled
	if isComplete {
		enrollment = EnrollmentLifecycleCompleted
	} else if isEnrolled {
		enrollment = EnrollmentLifecycleActive
	} else if isPublic {
		enrollment = EnrollmentLifecycleEligible
	}

	access := CourseAccessUnavailable
	if isComplete {
		access = CourseAccessCompleted
	} else if isEnrolled {
		access = CourseAccessEnrolled
	} else if isPublic {
		access = CourseAccessPreview
	}

	return CourseControlState{
		Lifecycle: lifecycle, Enrollment: enrollment, Access: access,
		Progress: progress, IsComplete: isComplete,
		CertificateEligible: isComplete && isEnrolled,
	}
}

func CanTransitionEnrollment(from, to EnrollmentLifecycle) bool {
	transitions := map[EnrollmentLifecycle][]EnrollmentLifecycle{
		EnrollmentLifecycleEligible:       {EnrollmentLifecyclePendingPayment, EnrollmentLifecycleActive, EnrollmentLifecycleCancelled},
		EnrollmentLifecyclePendingPayment: {EnrollmentLifecycleActive, EnrollmentLifecycleCancelled},
		EnrollmentLifecycleActive:         {EnrollmentLifecycleCompleted, EnrollmentLifecycleCancelled},
		EnrollmentLifecycleCancelled:      {EnrollmentLifecycleEligible, EnrollmentLifecyclePendingPayment},
	}
	for _, target := range transitions[from] {
		if target == to {
			return true
		}
	}
	return false
}
