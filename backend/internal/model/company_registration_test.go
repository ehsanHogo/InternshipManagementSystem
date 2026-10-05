package model

import "testing"

func TestCompanyRegistrationStatuses(t *testing.T) {
	for _, status := range []CompanyRegistrationStatus{CompanyRegistrationStatusPending, CompanyRegistrationStatusApproved, CompanyRegistrationStatusRejected} {
		if !status.Valid() {
			t.Fatalf("valid registration status rejected: %s", status)
		}
	}
	for _, status := range []CompanyRegistrationStatus{"", "READY", "approved"} {
		if status.Valid() {
			t.Fatalf("unknown registration status accepted: %s", status)
		}
	}
}
