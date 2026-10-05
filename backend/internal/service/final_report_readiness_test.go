package service_test

import (
	"strconv"
	"testing"
	"time"

	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

func TestFinalReportReadinessRequiresExactApprovedWeeks(t *testing.T) {
	now := time.Now()
	ready := make([]model.WeeklyReport, 8)
	for i := range ready {
		ready[i] = model.WeeklyReport{WeekNumber: i + 1, SubmittedAt: &now, CompanyReviewStatus: model.WeeklyReviewApproved, ProfessorReviewStatus: model.WeeklyReviewApproved}
	}
	item := model.InternshipCase{Status: model.InternshipCaseStatusActive, WeeklyReports: ready}
	if !service.CanUploadFinalReport(&item) {
		t.Fatal("all eight approved weeks should allow the initial upload")
	}
	for _, week := range []int{0, 2, 9} {
		t.Run("invalid or duplicate week "+strconv.Itoa(week), func(t *testing.T) {
			item.WeeklyReports = append([]model.WeeklyReport(nil), ready...)
			item.WeeklyReports[0].WeekNumber = week
			if service.CanUploadFinalReport(&item) {
				t.Fatalf("eight approved rows with week %d replacing week 1 must not allow upload", week)
			}
		})
	}
	item.WeeklyReports = append(append([]model.WeeklyReport(nil), ready...), ready[0])
	if service.CanUploadFinalReport(&item) {
		t.Fatal("extra weekly rows must not allow the initial upload")
	}
	// An existing revision is eligible even without any historical weekly rows.
	item.WeeklyReports = nil
	item.FinalReport = &model.FinalReport{Status: model.FinalReportRevisionRequested}
	if !service.CanUploadFinalReport(&item) {
		t.Fatal("revision must not depend on initial weekly readiness")
	}
	for _, status := range []model.FinalReportStatus{model.FinalReportSubmitted, model.FinalReportApproved} {
		item.FinalReport.Status = status
		if service.CanUploadFinalReport(&item) {
			t.Fatalf("report in %s must not allow replacement", status)
		}
	}
	for _, status := range []model.InternshipCaseStatus{
		model.InternshipCaseStatusDraft, model.InternshipCaseStatusRevisionRequested,
		model.InternshipCaseStatusPendingUniversityReview, model.InternshipCaseStatusPendingCompanyDetails,
		model.InternshipCaseStatusPendingFinalApproval, model.InternshipCaseStatusPassed,
		model.InternshipCaseStatusFailed, model.InternshipCaseStatusCancelled,
	} {
		item.Status, item.WeeklyReports = status, ready
		for _, report := range []*model.FinalReport{nil, {Status: model.FinalReportRevisionRequested}} {
			item.FinalReport = report
			if service.CanUploadFinalReport(&item) {
				t.Fatalf("case in %s must not allow initial or revision uploads", status)
			}
		}
	}
}
