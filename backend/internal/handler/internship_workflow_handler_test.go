package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

func TestParseDateUsesUTC(t *testing.T) {
	date, err := parseDate(" 2026-09-19 ")
	if err != nil {
		t.Fatalf("parseDate returned an error: %v", err)
	}

	want := time.Date(2026, time.September, 19, 0, 0, 0, 0, time.UTC)
	if !date.Equal(want) {
		t.Fatalf("parseDate() = %v, want %v", date, want)
	}
	if date.Location() != time.UTC {
		t.Fatalf("parseDate() location = %v, want UTC", date.Location())
	}
}

func TestParseDateRejectsNonGregorianAPIValue(t *testing.T) {
	if _, err := parseDate("1405/06/28"); err == nil {
		t.Fatal("parseDate accepted a Jalali display value; the API must receive YYYY-MM-DD")
	}
}

func TestIncompleteWeeklyReportsUseConflictResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	(&InternshipHandler{}).writeError(ctx, service.ErrWeeklyReportsIncomplete)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}
}

func TestCaseResponseDerivesCompanyEvaluationReadiness(t *testing.T) {
	reports := make([]model.WeeklyReport, 8)
	for index := range reports {
		reports[index] = model.WeeklyReport{WeekNumber: index + 1, CompanyReviewStatus: model.WeeklyReviewApproved, ProfessorReviewStatus: model.WeeklyReviewPending, SubmittedAt: timePointerForTest()}
	}
	internshipCase := &model.InternshipCase{
		Status: model.InternshipCaseStatusActive, WeeklyReports: reports,
	}

	response := caseResponse(internshipCase)
	if response.WeeklyReportCount != 8 || response.CompanyApprovedReportCount != 8 || response.ApprovedReportCount != 0 || !response.CanSubmitCompanyEvaluation {
		t.Fatalf("unexpected ready response: %+v", response)
	}

	internshipCase.CompanyEvaluation = &model.CompanyEvaluation{}
	if caseResponse(internshipCase).CanSubmitCompanyEvaluation {
		t.Fatal("case with an existing evaluation was reported as ready")
	}
}

func TestProfessorReadinessRequiresEveryPrerequisite(t *testing.T) {
	reports := make([]model.WeeklyReport, 8)
	for index := range reports {
		reports[index] = model.WeeklyReport{WeekNumber: index + 1, CompanyReviewStatus: model.WeeklyReviewApproved, ProfessorReviewStatus: model.WeeklyReviewApproved, SubmittedAt: timePointerForTest()}
	}
	internshipCase := &model.InternshipCase{
		Status: model.InternshipCaseStatusActive, WeeklyReports: reports,
		CompanyEvaluation: &model.CompanyEvaluation{}, FinalReport: &model.FinalReport{Status: model.FinalReportApproved},
	}

	reportCount, approvedCount, hasEvaluation, hasFinalReport, canComplete := professorReadiness(internshipCase)
	if reportCount != 8 || approvedCount != 8 || !hasEvaluation || !hasFinalReport || !canComplete {
		t.Fatalf("unexpected ready response: %d %d %v %v %v", reportCount, approvedCount, hasEvaluation, hasFinalReport, canComplete)
	}

	internshipCase.WeeklyReports[0].ProfessorReviewStatus = model.WeeklyReviewPending
	if _, _, _, _, ready := professorReadiness(internshipCase); ready {
		t.Fatal("case with an unconfirmed report was reported as ready")
	}
	internshipCase.WeeklyReports[0].ProfessorReviewStatus = model.WeeklyReviewApproved
	internshipCase.Status = model.InternshipCaseStatusPassed
	if _, _, _, _, ready := professorReadiness(internshipCase); ready {
		t.Fatal("completed case was reported as completable")
	}
}

func TestProfessorPrerequisiteErrorsUseConflictResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, testErr := range []error{
		service.ErrProfessorCaseNotActive,
		service.ErrProfessorWeeklyReportsIncomplete,
		service.ErrProfessorCompanyEvaluationRequired,
		service.ErrProfessorFinalReportRequired,
	} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		(&InternshipHandler{}).writeError(ctx, testErr)
		if recorder.Code != http.StatusConflict {
			t.Fatalf("error %v status = %d, want %d", testErr, recorder.Code, http.StatusConflict)
		}
	}
}

func TestProfessorCompletionRejectsNumericJSONResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	ctx.Set("auth_user_id", uint(1))
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/professor/internship-cases/1/complete", bytes.NewBufferString(`{"result":18}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	(&InternshipHandler{}).CompleteProfessorCase(ctx)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func timePointerForTest() *time.Time { now := time.Now(); return &now }

func TestStudentEvaluationResponseVisibility(t *testing.T) {
	for _, status := range []model.InternshipCaseStatus{
		model.InternshipCaseStatusDraft, model.InternshipCaseStatusRevisionRequested,
		model.InternshipCaseStatusPendingUniversityReview, model.InternshipCaseStatusPendingCompanyDetails,
		model.InternshipCaseStatusPendingFinalApproval, model.InternshipCaseStatusActive,
		model.InternshipCaseStatusPassed, model.InternshipCaseStatusFailed, model.InternshipCaseStatusCancelled,
	} {
		t.Run(string(status), func(t *testing.T) {
			item := &model.InternshipCase{Status: status, CompanyEvaluation: &model.CompanyEvaluation{ID: 1}}
			data, err := json.Marshal(studentCaseResponse(item))
			if err != nil {
				t.Fatal(err)
			}
			visible := status == model.InternshipCaseStatusPassed || status == model.InternshipCaseStatusFailed
			if bytes.Contains(data, []byte(`"companyEvaluation"`)) != visible {
				t.Fatalf("unexpected student evaluation visibility: %s", data)
			}
			if caseResponse(item).CompanyEvaluation == nil || professorDetailResponse(item).CompanyEvaluation == nil {
				t.Fatal("student redaction modified reviewer response")
			}
		})
	}
}

func TestCompanyRegistrationErrorWritesOneJSONResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	(&InternshipHandler{}).writeError(ctx, service.ErrCompanyRegistrationRequired)
	var response struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("invalid error JSON: %s / %v", recorder.Body.String(), err)
	}
	if recorder.Code != http.StatusForbidden || response.Code != "COMPANY_REGISTRATION_NOT_APPROVED" || response.Error == "" {
		t.Fatalf("incorrect permission response: %+v", response)
	}
}

func TestProfessorResponsesUseSelectedPlacementCompany(t *testing.T) {
	selected := model.InternshipPreference{OpportunityApplication: model.OpportunityApplication{Opportunity: model.InternshipOpportunity{Company: model.Company{Name: "selected company"}}}}
	item := &model.InternshipCase{Preferences: []model.InternshipPreference{{OpportunityApplication: model.OpportunityApplication{Opportunity: model.InternshipOpportunity{Company: model.Company{Name: "first unselected company"}}}}}, SelectedPreference: &selected}
	if professorListResponse(item).Company != "selected company" || professorDetailResponse(item).Internship.Company != "selected company" {
		t.Fatal("professor response must display the selected placement company")
	}
	item.SelectedPreference = nil
	if professorListResponse(item).Company != "" || professorDetailResponse(item).Internship.Company != "" {
		t.Fatal("unselected preferences must not become professor placement display")
	}
}
