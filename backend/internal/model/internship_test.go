package model

import "testing"

func TestProfessorFinalResultValidation(t *testing.T) {
	for _, result := range []ProfessorFinalResult{
		ProfessorFinalResultExcellent,
		ProfessorFinalResultGood,
		ProfessorFinalResultFailed,
	} {
		if !result.Valid() {
			t.Fatalf("expected %q to be valid", result)
		}
	}
	for _, result := range []ProfessorFinalResult{"", "18", "20", "AVERAGE", "WEAK", "PASS"} {
		if result.Valid() {
			t.Fatalf("expected %q to be invalid", result)
		}
	}
}

func TestInternshipCaseStatusValidation(t *testing.T) {
	valid := []InternshipCaseStatus{
		InternshipCaseStatusDraft,
		InternshipCaseStatusPendingUniversityReview,
		InternshipCaseStatusPendingCompanyDetails,
		InternshipCaseStatusPendingFinalApproval,
		InternshipCaseStatusReadyToStart,
		InternshipCaseStatusActive,
		InternshipCaseStatusCompleted,
		InternshipCaseStatusCancelled,
	}
	for _, status := range valid {
		if !status.Valid() {
			t.Fatalf("expected %q to be valid", status)
		}
	}
	for _, status := range []InternshipCaseStatus{
		"", "PENDING_UNIVERSITY_APPROVAL", "PENDING_COMPANY_APPROVAL", "COMPANY_APPROVED", "UNIVERSITY_APPROVED",
	} {
		if status.Valid() {
			t.Fatalf("expected obsolete status %q to be invalid", status)
		}
	}
}
