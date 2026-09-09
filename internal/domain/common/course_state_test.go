package models

import "testing"

func TestDeriveCourseControlState(t *testing.T) {
	state := DeriveCourseControlState(CourseStatusPublished, true, true, true, 100, nil)
	if state.Enrollment != EnrollmentLifecycleCompleted || state.Access != CourseAccessCompleted || !state.CertificateEligible {
		t.Fatalf("expected completed enrolled state, got %+v", state)
	}

	draft := DeriveCourseControlState(CourseStatusDraft, true, false, false, 0, nil)
	if draft.Access != CourseAccessUnavailable || draft.Enrollment != EnrollmentLifecycleCancelled {
		t.Fatalf("expected unavailable draft state, got %+v", draft)
	}
}

func TestCanTransitionEnrollment(t *testing.T) {
	if CanTransitionEnrollment(EnrollmentLifecycleEligible, EnrollmentLifecycleCompleted) {
		t.Fatal("eligible must not skip directly to completed")
	}
	if !CanTransitionEnrollment(EnrollmentLifecycleActive, EnrollmentLifecycleCompleted) {
		t.Fatal("active should transition to completed")
	}
}
