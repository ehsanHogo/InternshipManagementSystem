package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

type professorCaseListResponse struct {
	CaseID                     uint                        `json:"caseId"`
	Student                    model.PublicUser            `json:"student"`
	Company                    string                      `json:"company"`
	InternshipSubject          *string                     `json:"internshipSubject"`
	Status                     model.InternshipCaseStatus  `json:"status"`
	WeeklyReportCount          int                         `json:"weeklyReportCount"`
	ConfirmedWeeklyReportCount int                         `json:"confirmedWeeklyReportCount"`
	HasCompanyEvaluation       bool                        `json:"hasCompanyEvaluation"`
	HasFinalReport             bool                        `json:"hasFinalReport"`
	CanProfessorComplete       bool                        `json:"canProfessorComplete"`
	FinalResult                *model.ProfessorFinalResult `json:"finalResult,omitempty"`
}

type professorStudentResponse struct {
	FullName      string  `json:"fullName"`
	StudentNumber *string `json:"studentNumber"`
	Major         *string `json:"major"`
	PassedCredits *int    `json:"passedCredits"`
	Mobile        *string `json:"mobile"`
}

type professorInternshipResponse struct {
	Company           string                     `json:"company"`
	InternshipSubject *string                    `json:"internshipSubject"`
	StartDate         *time.Time                 `json:"startDate"`
	WorkplaceAddress  *string                    `json:"workplaceAddress"`
	WorkplacePhone    *string                    `json:"workplacePhone"`
	CompanySupervisor *model.PublicUser          `json:"companySupervisor,omitempty"`
	Status            model.InternshipCaseStatus `json:"status"`
}

type professorCompanyEvaluationResponse struct {
	ID                       uint                   `json:"id"`
	InternshipCaseID         uint                   `json:"internshipCaseId"`
	CompanySupervisorID      uint                   `json:"companySupervisorId"`
	AttendanceRating         model.EvaluationRating `json:"attendanceRating"`
	ParticipationRating      model.EvaluationRating `json:"participationRating"`
	LearningRating           model.EvaluationRating `json:"learningRating"`
	InterestRating           model.EvaluationRating `json:"interestRating"`
	PersistenceRating        model.EvaluationRating `json:"persistenceRating"`
	SuggestionRating         model.EvaluationRating `json:"suggestionRating"`
	ResourceUsageRating      model.EvaluationRating `json:"resourceUsageRating"`
	ReportQualityRating      model.EvaluationRating `json:"reportQualityRating"`
	ProjectPerformanceRating model.EvaluationRating `json:"projectPerformanceRating"`
	LeaveDays                int                    `json:"leaveDays"`
	AbsenceDays              int                    `json:"absenceDays"`
	Suggestions              *string                `json:"suggestions"`
	SubmittedAt              time.Time              `json:"submittedAt"`
	CompanySupervisor        *model.PublicUser      `json:"companySupervisor,omitempty"`
}

type professorCaseDetailResponse struct {
	CaseID                     uint                                `json:"caseId"`
	Student                    professorStudentResponse            `json:"student"`
	Internship                 professorInternshipResponse         `json:"internship"`
	WeeklyReports              []model.WeeklyReport                `json:"weeklyReports"`
	CompanyEvaluation          *professorCompanyEvaluationResponse `json:"companyEvaluation,omitempty"`
	FinalReport                *fileMetadataResponse               `json:"finalReport,omitempty"`
	WeeklyReportCount          int                                 `json:"weeklyReportCount"`
	ConfirmedWeeklyReportCount int                                 `json:"confirmedWeeklyReportCount"`
	HasCompanyEvaluation       bool                                `json:"hasCompanyEvaluation"`
	HasFinalReport             bool                                `json:"hasFinalReport"`
	CanProfessorComplete       bool                                `json:"canProfessorComplete"`
	FinalResult                *model.ProfessorFinalResult         `json:"finalResult,omitempty"`
	ProfessorComment           *string                             `json:"professorComment,omitempty"`
	CompletedAt                *time.Time                          `json:"completedAt,omitempty"`
}

type completeProfessorCaseRequest struct {
	Result  model.ProfessorFinalResult `json:"result"`
	Comment *string                    `json:"comment"`
}

func (handler *InternshipHandler) ListProfessorCases(ctx *gin.Context) {
	professorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	cases, err := handler.service.ListProfessorCases(professorID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	response := make([]professorCaseListResponse, 0, len(cases))
	for index := range cases {
		response = append(response, professorListResponse(&cases[index]))
	}
	ctx.JSON(http.StatusOK, response)
}

func (handler *InternshipHandler) GetProfessorCase(ctx *gin.Context) {
	professorID, caseID, ok := professorCaseRequest(ctx)
	if !ok {
		return
	}
	internshipCase, err := handler.service.GetProfessorCase(professorID, caseID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, professorDetailResponse(internshipCase))
}

func (handler *InternshipHandler) CompleteProfessorCase(ctx *gin.Context) {
	professorID, caseID, ok := professorCaseRequest(ctx)
	if !ok {
		return
	}
	var request completeProfessorCaseRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "اطلاعات درخواست معتبر نیست."})
		return
	}
	internshipCase, err := handler.service.CompleteProfessorCase(professorID, caseID, service.ProfessorCompletionInput{
		Result: request.Result, Comment: request.Comment,
	})
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, professorDetailResponse(internshipCase))
}

func professorCaseRequest(ctx *gin.Context) (uint, uint, bool) {
	professorID, ok := currentUserID(ctx)
	if !ok {
		return 0, 0, false
	}
	caseID, err := parseID(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "شناسه پرونده کارآموزی معتبر نیست."})
		return 0, 0, false
	}
	return professorID, caseID, true
}

func professorListResponse(internshipCase *model.InternshipCase) professorCaseListResponse {
	reportCount, confirmedCount, hasEvaluation, hasFinalReport, canComplete := professorReadiness(internshipCase)
	return professorCaseListResponse{
		CaseID: internshipCase.ID, Student: internshipCase.Student.Public(), Company: professorCompanyName(internshipCase),
		InternshipSubject: internshipCase.InternshipSubject, Status: internshipCase.Status,
		WeeklyReportCount: reportCount, ConfirmedWeeklyReportCount: confirmedCount,
		HasCompanyEvaluation: hasEvaluation, HasFinalReport: hasFinalReport,
		CanProfessorComplete: canComplete, FinalResult: internshipCase.FinalResult,
	}
}

func professorDetailResponse(internshipCase *model.InternshipCase) professorCaseDetailResponse {
	reportCount, confirmedCount, hasEvaluation, hasFinalReport, canComplete := professorReadiness(internshipCase)
	response := professorCaseDetailResponse{
		CaseID: internshipCase.ID,
		Student: professorStudentResponse{
			FullName: internshipCase.Student.FullName, StudentNumber: internshipCase.Student.StudentNumber,
			Major: internshipCase.Student.Major, PassedCredits: internshipCase.PassedCredits, Mobile: internshipCase.Mobile,
		},
		Internship: professorInternshipResponse{
			Company: professorCompanyName(internshipCase), InternshipSubject: internshipCase.InternshipSubject,
			StartDate: internshipCase.StartDate, WorkplaceAddress: internshipCase.WorkplaceAddress,
			WorkplacePhone: internshipCase.WorkplacePhone, Status: internshipCase.Status,
		},
		WeeklyReports:     internshipCase.WeeklyReports,
		WeeklyReportCount: reportCount, ConfirmedWeeklyReportCount: confirmedCount,
		HasCompanyEvaluation: hasEvaluation, HasFinalReport: hasFinalReport, CanProfessorComplete: canComplete,
		FinalResult: internshipCase.FinalResult, ProfessorComment: internshipCase.ProfessorComment,
		CompletedAt: internshipCase.CompletedAt,
	}
	if response.WeeklyReports == nil {
		response.WeeklyReports = []model.WeeklyReport{}
	}
	if internshipCase.CompanySupervisor != nil {
		supervisor := internshipCase.CompanySupervisor.Public()
		response.Internship.CompanySupervisor = &supervisor
	}
	if internshipCase.CompanyEvaluation != nil {
		evaluation := internshipCase.CompanyEvaluation
		response.CompanyEvaluation = &professorCompanyEvaluationResponse{
			ID: evaluation.ID, InternshipCaseID: evaluation.InternshipCaseID,
			CompanySupervisorID: evaluation.CompanySupervisorID,
			AttendanceRating:    evaluation.AttendanceRating, ParticipationRating: evaluation.ParticipationRating,
			LearningRating: evaluation.LearningRating, InterestRating: evaluation.InterestRating,
			PersistenceRating: evaluation.PersistenceRating, SuggestionRating: evaluation.SuggestionRating,
			ResourceUsageRating: evaluation.ResourceUsageRating, ReportQualityRating: evaluation.ReportQualityRating,
			ProjectPerformanceRating: evaluation.ProjectPerformanceRating, LeaveDays: evaluation.LeaveDays,
			AbsenceDays: evaluation.AbsenceDays, Suggestions: evaluation.Suggestions, SubmittedAt: evaluation.SubmittedAt,
			CompanySupervisor: response.Internship.CompanySupervisor,
		}
	}
	if internshipCase.FinalReportFile != nil {
		response.FinalReport = &fileMetadataResponse{
			ID: internshipCase.FinalReportFile.ID, OriginalName: internshipCase.FinalReportFile.OriginalName,
			UploadedAt: internshipCase.FinalReportFile.UploadedAt,
		}
	}
	return response
}

func professorReadiness(internshipCase *model.InternshipCase) (int, int, bool, bool, bool) {
	reportCount := len(internshipCase.WeeklyReports)
	confirmedCount := 0
	for _, report := range internshipCase.WeeklyReports {
		if report.IsConfirmed {
			confirmedCount++
		}
	}
	hasEvaluation := internshipCase.CompanyEvaluation != nil
	hasFinalReport := internshipCase.FinalReportFile != nil
	canComplete := internshipCase.Status == model.InternshipCaseStatusActive && reportCount == 8 && confirmedCount == 8 && hasEvaluation && hasFinalReport
	return reportCount, confirmedCount, hasEvaluation, hasFinalReport, canComplete
}

func professorCompanyName(internshipCase *model.InternshipCase) string {
	if internshipCase.SelectedPreference == nil {
		return ""
	}
	if internshipCase.SelectedPreference.Company != nil {
		return internshipCase.SelectedPreference.Company.Name
	}
	if internshipCase.SelectedPreference.ProposedCompanyName != nil {
		return strings.TrimSpace(*internshipCase.SelectedPreference.ProposedCompanyName)
	}
	return ""
}
