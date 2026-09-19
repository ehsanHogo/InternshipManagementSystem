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
