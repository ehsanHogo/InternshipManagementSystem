package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

type sendToCompanyRequest struct {
	PreferenceID        uint   `json:"preferenceId"`
	CompanySupervisorID uint   `json:"companySupervisorId"`
	LetterNumber        string `json:"letterNumber"`
	LetterDate          string `json:"letterDate"`
}

type companyConfirmationRequest struct {
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

func (handler *InternshipHandler) ListCompanySupervisors(ctx *gin.Context) {
	users, err := handler.service.ListCompanySupervisors()
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	response := make([]model.PublicUser, 0, len(users))
	for _, user := range users {
		response = append(response, user.Public())
	}
	ctx.JSON(http.StatusOK, response)
}

func (handler *InternshipHandler) SendToCompany(ctx *gin.Context) {
	caseID, ok := caseIDFromContext(ctx)
	if !ok {
		return
	}
	var request sendToCompanyRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	letterDate, err := parseDate(request.LetterDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid letter date"})
		return
	}
	internshipCase, err := handler.service.SendToCompany(caseID, service.SendToCompanyInput{
		PreferenceID: request.PreferenceID, CompanySupervisorID: request.CompanySupervisorID,
		LetterNumber: request.LetterNumber, LetterDate: letterDate,
	})
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, caseResponse(internshipCase))
}

func (handler *InternshipHandler) ApproveUniversityCase(ctx *gin.Context) {
	handler.performUniversityAction(ctx, handler.service.ApproveUniversityCase)
}

func (handler *InternshipHandler) ActivateUniversityCase(ctx *gin.Context) {
	handler.performUniversityAction(ctx, handler.service.ActivateUniversityCase)
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
	ctx.JSON(http.StatusOK, caseResponses(cases))
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
	ctx.JSON(http.StatusOK, caseResponse(internshipCase))
}

func (handler *InternshipHandler) ConfirmCompanyCase(ctx *gin.Context) {
	supervisorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	caseID, ok := caseIDFromContext(ctx)
	if !ok {
		return
	}
	var request companyConfirmationRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	startDate, err := parseDate(request.StartDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid start date"})
		return
	}
	internshipCase, err := handler.service.ConfirmCompanyCase(supervisorID, caseID, service.CompanyConfirmationInput{
		InternshipSubject: request.InternshipSubject, StartDate: startDate,
		WorkplaceAddress: request.WorkplaceAddress, WorkplacePhone: request.WorkplacePhone,
	})
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, caseResponse(internshipCase))
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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid internship case id"})
		return 0, false
	}
	return caseID, true
}

func parseDate(value string) (time.Time, error) {
	return time.Parse("2006-01-02", strings.TrimSpace(value))
}
