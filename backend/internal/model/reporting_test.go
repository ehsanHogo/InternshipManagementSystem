package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestWeeklyReportDerivedStatus(t *testing.T) {
	now := time.Now()
	for _, submitted := range []bool{false, true} {
		for _, company := range []WeeklyReviewStatus{WeeklyReviewPending, WeeklyReviewApproved, WeeklyReviewRevisionRequested} {
			for _, professor := range []WeeklyReviewStatus{WeeklyReviewPending, WeeklyReviewApproved, WeeklyReviewRevisionRequested} {
				t.Run(string(company)+"/"+string(professor)+"/"+map[bool]string{false: "draft", true: "submitted"}[submitted], func(t *testing.T) {
					report := WeeklyReport{CompanyReviewStatus: company, ProfessorReviewStatus: professor}
					want := WeeklyReportDraft
					if submitted {
						report.SubmittedAt = &now
						want = WeeklyReportSubmitted
						if company == WeeklyReviewRevisionRequested || professor == WeeklyReviewRevisionRequested {
							want = WeeklyReportRevisionRequested
						} else if company == WeeklyReviewApproved && professor == WeeklyReviewApproved {
							want = WeeklyReportApproved
						}
					}
					if report.Status() != want {
						t.Fatalf("status=%s want=%s", report.Status(), want)
					}
					data, err := json.Marshal(report)
					if err != nil {
						t.Fatal(err)
					}
					var response map[string]any
					if err := json.Unmarshal(data, &response); err != nil {
						t.Fatal(err)
					}
					if response["status"] != string(want) {
						t.Fatalf("serialized status=%v want=%s", response["status"], want)
					}
					for _, old := range []string{"isConfirmed", "confirmedAt", "supervisorComment"} {
						if _, exists := response[old]; exists {
							t.Fatalf("obsolete response field: %s", old)
						}
					}
				})
			}
		}
	}
}
