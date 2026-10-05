package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

type approveUniversityPlacementRequest struct {
	PreferenceID uint   `json:"preferenceId"`
	LetterNumber string `json:"letterNumber"`
	LetterDate   string `json:"letterDate"`
}

type cancelUniversityReviewRequest struct {
	Comment string `json:"comment"`
}

type placementDetailsRequest struct {
	InternshipSubject string `json:"internshipSubject"`
	StartDate         string `json:"startDate"`
	WorkplaceAddress  string `json:"workplaceAddress"`
	WorkplacePhone    string `json:"workplacePhone"`
}

func (handler *InternshipHandler) ListUniversityCases(ctx *gin.Context) {
	var status *model.InternshipCaseStatus
	if raw := strings.TrimSpace(ctx.Query("status")); raw != "" {
		value := model.InternshipCaseStatus(raw)
		status = &value
	}
	cases, err := handler.service.ListUniversityCases(status)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, caseResponses(cases))
}

func (handler *InternshipHandler) ListPendingUniversityReviewCases(ctx *gin.Context) {
	cases, err := handler.service.ListPendingUniversityReviewCases()
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, caseResponses(cases))
}

func (handler *InternshipHandler) GetUniversityCase(ctx *gin.Context) {
	caseID, ok := caseIDFromContext(ctx)
	if !ok {
		return
	}
	internshipCase, err := handler.service.GetUniversityCase(caseID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, caseResponse(internshipCase))
}

func (handler *InternshipHandler) ListPendingFinalApprovalCases(ctx *gin.Context) {
	cases, err := handler.service.ListPendingFinalApprovalCases()
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, caseResponses(cases))
}

func (handler *InternshipHandler) ApprovePlacementDetails(ctx *gin.Context) {
	handler.performUniversityAction(ctx, handler.service.ApprovePlacementDetails)
}

func (handler *InternshipHandler) RequestPlacementCorrection(ctx *gin.Context) {
	caseID, ok := caseIDFromContext(ctx)
	if !ok {
		return
	}
	var request struct {
		Comment string `json:"comment"`
	}
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_PLACEMENT_CORRECTION", "error": "توضیحات اصلاحات معتبر نیست؛ فقط توضیحات را ارسال کنید."})
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_PLACEMENT_CORRECTION", "error": "توضیحات اصلاحات معتبر نیست."})
		return
	}
	item, err := handler.service.RequestPlacementCorrection(caseID, request.Comment)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, caseResponse(item))
}

func (handler *InternshipHandler) ApproveUniversityPlacement(ctx *gin.Context) {
	caseID, ok := caseIDFromContext(ctx)
	if !ok {
		return
	}
	var request approveUniversityPlacementRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_UNIVERSITY_REVIEW", "error": "اطلاعات بررسی پرونده معتبر نیست."})
		return
	}
	if strings.TrimSpace(request.LetterDate) == "" {
		handler.writeError(ctx, service.ErrIntroductionLetterDateRequired)
		return
	}
	letterDate, err := parseDate(request.LetterDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INTRODUCTION_LETTER_DATE_REQUIRED", "error": "تاریخ معرفی‌نامه معتبر نیست."})
		return
	}
	internshipCase, err := handler.service.ApproveUniversityPlacement(caseID, service.UniversityPlacementApprovalInput{
		PreferenceID: request.PreferenceID,
		LetterNumber: request.LetterNumber,
		LetterDate:   letterDate,
	})
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, caseResponse(internshipCase))
}

func (handler *InternshipHandler) CancelUniversityReview(ctx *gin.Context) {
	caseID, ok := caseIDFromContext(ctx)
	if !ok {
		return
	}
	var request cancelUniversityReviewRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_UNIVERSITY_REVIEW", "error": "اطلاعات لغو پرونده معتبر نیست."})
		return
	}
	internshipCase, err := handler.service.CancelUniversityReview(caseID, request.Comment)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, caseResponse(internshipCase))
}

func (handler *InternshipHandler) RequestUniversityRevision(ctx *gin.Context) {
	supervisorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	caseID, ok := caseIDFromContext(ctx)
	if !ok {
		return
	}
	var request struct {
		Comment string `json:"comment"`
	}
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_UNIVERSITY_REVISION", "error": "توضیحات اصلاح اولویت‌ها معتبر نیست."})
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_UNIVERSITY_REVISION", "error": "توضیحات اصلاح اولویت‌ها معتبر نیست."})
		return
	}
	item, err := handler.service.RequestUniversityRevision(caseID, supervisorID, request.Comment)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, caseResponse(item))
}

func (handler *InternshipHandler) performUniversityAction(ctx *gin.Context, action func(uint) (*model.InternshipCase, error)) {
	caseID, ok := caseIDFromContext(ctx)
	if !ok {
		return
	}
	internshipCase, err := action(caseID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, caseResponse(internshipCase))
}

func (handler *InternshipHandler) ListCompanyCases(ctx *gin.Context) {
	supervisorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	cases, err := handler.service.ListCompanyCases(supervisorID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, companyCaseResponses(cases))
}

func (handler *InternshipHandler) GetCompanyCase(ctx *gin.Context) {
	supervisorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	caseID, ok := caseIDFromContext(ctx)
	if !ok {
		return
	}
	internshipCase, err := handler.service.GetCompanyCase(supervisorID, caseID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, companyCaseResponse(internshipCase))
}

func (handler *InternshipHandler) ListPendingCompanyDetailsCases(ctx *gin.Context) {
	supervisorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	cases, err := handler.service.ListPendingCompanyDetailsCases(supervisorID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, companyCaseResponses(cases))
}

func (handler *InternshipHandler) SubmitPlacementDetails(ctx *gin.Context) {
	supervisorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	caseID, ok := caseIDFromContext(ctx)
	if !ok {
		return
	}
	var request placementDetailsRequest
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_PLACEMENT_DETAILS", "error": "فقط اطلاعات شروع کارآموزی را ارسال کنید."})
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_PLACEMENT_DETAILS", "error": "اطلاعات شروع کارآموزی معتبر نیست."})
		return
	}
	if strings.TrimSpace(request.StartDate) == "" {
		handler.writeError(ctx, service.ErrStartDateRequired)
		return
	}
	startDate, err := parseDate(request.StartDate)
	if err != nil || startDate.Year() < 1 {
		handler.writeError(ctx, service.ErrInvalidStartDate)
		return
	}
	internshipCase, err := handler.service.SubmitPlacementDetails(supervisorID, caseID, service.PlacementDetailsInput{
		InternshipSubject: request.InternshipSubject, StartDate: startDate,
		WorkplaceAddress: request.WorkplaceAddress, WorkplacePhone: request.WorkplacePhone,
	})
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, companyCaseResponse(internshipCase))
}

// A company sees only its selected placement, without student preference priority
// or other recruitment preferences or passed-credit information.
func companyCaseResponse(internshipCase *model.InternshipCase) internshipCaseResponse {
	response := caseResponse(internshipCase)
	response.WeeklyReports = submittedWeeklyReports(response.WeeklyReports)
	response.PassedCredits = nil
	response.Preferences = []internshipPreferenceResponse{}
	if response.SelectedPreference != nil {
		response.SelectedPreference.Priority = 0 // omitted from the company JSON
		response.Preferences = append(response.Preferences, *response.SelectedPreference)
	}
	return response
}

func companyCaseResponses(cases []model.InternshipCase) []internshipCaseResponse {
	response := make([]internshipCaseResponse, 0, len(cases))
	for i := range cases {
		response = append(response, companyCaseResponse(&cases[i]))
	}
	return response
}

func caseResponses(cases []model.InternshipCase) []internshipCaseResponse {
	response := make([]internshipCaseResponse, 0, len(cases))
	for i := range cases {
		response = append(response, caseResponse(&cases[i]))
	}
	return response
}

func caseIDFromContext(ctx *gin.Context) (uint, bool) {
	caseID, err := parseID(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "شناسه پرونده کارآموزی معتبر نیست."})
		return 0, false
	}
	return caseID, true
}

func parseDate(value string) (time.Time, error) {
	date, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(value), time.UTC)
	if err != nil {
		return time.Time{}, err
	}
	return date.UTC(), nil
}

// RegisterUniversityCaseRoutes is shared by production and integration tests.
func (handler *InternshipHandler) RegisterUniversityCaseRoutes(university *gin.RouterGroup) {
	university.POST("/internship-cases/:id/approve-placement", handler.ApproveUniversityPlacement)
	university.POST("/internship-cases/:id/request-revision", handler.RequestUniversityRevision)
	university.POST("/internship-cases/:id/final-approve", handler.ApprovePlacementDetails)
	university.POST("/internship-cases/:id/request-placement-correction", handler.RequestPlacementCorrection)
	university.POST("/internship-cases/:id/cancel", handler.CancelUniversityReview)
}
