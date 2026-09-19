package handler

import (
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
		reports[index] = model.WeeklyReport{WeekNumber: index + 1, IsConfirmed: true}
	}
	internshipCase := &model.InternshipCase{
		Status: model.InternshipCaseStatusActive, WeeklyReports: reports,
	}

	response := caseResponse(internshipCase)
	if response.WeeklyReportCount != 8 || response.ConfirmedReportCount != 8 || !response.CanSubmitCompanyEvaluation {
		t.Fatalf("unexpected ready response: %+v", response)
	}

	internshipCase.CompanyEvaluation = &model.CompanyEvaluation{}
	if caseResponse(internshipCase).CanSubmitCompanyEvaluation {
		t.Fatal("case with an existing evaluation was reported as ready")
	}
}
