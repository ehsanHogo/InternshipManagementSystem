package handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	appmiddleware "internship-management-system/backend/internal/middleware"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

type InternshipHandler struct {
	service *service.InternshipService
}

type updateCaseRequest struct {
	PassedCredits *int    `json:"passedCredits"`
	Mobile        *string `json:"mobile"`
}

type preferenceRequest struct {
	Priority               int     `json:"priority"`
	CompanyID              *uint   `json:"companyId"`
	ProposedCompanyName    *string `json:"proposedCompanyName"`
	ProposedWebsite        *string `json:"proposedWebsite"`
	ProposedPhone          *string `json:"proposedPhone"`
	ProposedEmail          *string `json:"proposedEmail"`
	ProposedSupervisorName *string `json:"proposedSupervisorName"`
	City                   string  `json:"city"`
	WorkField              string  `json:"workField"`
}

type internshipCaseResponse struct {
	ID                   uint                           `json:"id"`
	Status               model.InternshipCaseStatus     `json:"status"`
	PassedCredits        *int                           `json:"passedCredits"`
	Mobile               *string                        `json:"mobile"`
	Student              model.PublicUser               `json:"student"`
	Professor            model.PublicUser               `json:"professor"`
	Preferences          []internshipPreferenceResponse `json:"preferences"`
	SelectedPreferenceID *uint                          `json:"selectedPreferenceId"`
	SelectedPreference   *internshipPreferenceResponse  `json:"selectedPreference,omitempty"`
	CompanySupervisorID  *uint                          `json:"companySupervisorId"`
	CompanySupervisor    *model.PublicUser              `json:"companySupervisor,omitempty"`
	LetterNumber         *string                        `json:"letterNumber"`
	LetterDate           *time.Time                     `json:"letterDate"`
	InternshipSubject    *string                        `json:"internshipSubject"`
	StartDate            *time.Time                     `json:"startDate"`
	WorkplaceAddress     *string                        `json:"workplaceAddress"`
	WorkplacePhone       *string                        `json:"workplacePhone"`
	CreatedAt            time.Time                      `json:"createdAt"`
	UpdatedAt            time.Time                      `json:"updatedAt"`
	SubmittedAt          *time.Time                     `json:"submittedAt"`
	CompanyConfirmedAt   *time.Time                     `json:"companyConfirmedAt"`
	UniversityApprovedAt *time.Time                     `json:"universityApprovedAt"`
	ActivatedAt          *time.Time                     `json:"activatedAt"`
}

type internshipPreferenceResponse struct {
	ID                     uint           `json:"id"`
	Priority               int            `json:"priority"`
	CompanyID              *uint          `json:"companyId"`
	Company                *model.Company `json:"company,omitempty"`
	ProposedCompanyName    *string        `json:"proposedCompanyName,omitempty"`
	ProposedWebsite        *string        `json:"proposedWebsite,omitempty"`
	ProposedPhone          *string        `json:"proposedPhone,omitempty"`
	ProposedEmail          *string        `json:"proposedEmail,omitempty"`
	ProposedSupervisorName *string        `json:"proposedSupervisorName,omitempty"`
	City                   string         `json:"city"`
	WorkField              string         `json:"workField"`
}

func NewInternshipHandler(internshipService *service.InternshipService) *InternshipHandler {
	return &InternshipHandler{service: internshipService}
}

func (handler *InternshipHandler) ListCompanies(ctx *gin.Context) {
	companies, err := handler.service.ListApprovedCompanies()
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, companies)
}

func (handler *InternshipHandler) GetCurrentCase(ctx *gin.Context) {
	studentID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	internshipCase, err := handler.service.GetCurrentCase(studentID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, caseResponse(internshipCase))
}

func (handler *InternshipHandler) CreateOrGetCase(ctx *gin.Context) {
	studentID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	internshipCase, created, err := handler.service.CreateOrGetCase(studentID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	ctx.JSON(status, caseResponse(internshipCase))
}

func (handler *InternshipHandler) UpdateCase(ctx *gin.Context) {
	studentID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	var request updateCaseRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	internshipCase, err := handler.service.UpdateCase(studentID, request.PassedCredits, request.Mobile)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, caseResponse(internshipCase))
}

func (handler *InternshipHandler) AddPreference(ctx *gin.Context) {
	studentID, request, ok := handler.preferenceRequest(ctx)
	if !ok {
		return
	}
	preference, err := handler.service.AddPreference(studentID, request.preferenceInput())
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, preferenceResponse(*preference))
}

func (handler *InternshipHandler) UpdatePreference(ctx *gin.Context) {
	studentID, request, ok := handler.preferenceRequest(ctx)
	if !ok {
		return
	}
	preferenceID, err := parseID(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid preference id"})
		return
	}
	preference, err := handler.service.UpdatePreference(studentID, preferenceID, request.preferenceInput())
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, preferenceResponse(*preference))
}

func (handler *InternshipHandler) DeletePreference(ctx *gin.Context) {
	studentID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	preferenceID, err := parseID(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid preference id"})
		return
	}
	if err := handler.service.DeletePreference(studentID, preferenceID); err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (handler *InternshipHandler) SubmitCase(ctx *gin.Context) {
	studentID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	internshipCase, err := handler.service.SubmitCase(studentID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, caseResponse(internshipCase))
}

func (handler *InternshipHandler) preferenceRequest(ctx *gin.Context) (uint, preferenceRequest, bool) {
	studentID, ok := currentUserID(ctx)
	if !ok {
		return 0, preferenceRequest{}, false
	}
	var request preferenceRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return 0, preferenceRequest{}, false
	}
	return studentID, request, true
}

func (request preferenceRequest) preferenceInput() service.PreferenceInput {
	return service.PreferenceInput{
		Priority: request.Priority, CompanyID: request.CompanyID,
		ProposedCompanyName: request.ProposedCompanyName, ProposedWebsite: request.ProposedWebsite,
		ProposedPhone: request.ProposedPhone, ProposedEmail: request.ProposedEmail,
		ProposedSupervisorName: request.ProposedSupervisorName,
		City:                   request.City, WorkField: request.WorkField,
	}
}

func (handler *InternshipHandler) writeError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrCaseNotFound), errors.Is(err, service.ErrPreferenceNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrAssignmentNotFound), errors.Is(err, service.ErrInvalidApplication),
		errors.Is(err, service.ErrInvalidPreference), errors.Is(err, service.ErrInvalidCaseStatus),
		errors.Is(err, service.ErrCompanySupervisor):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrCaseNotEditable), errors.Is(err, service.ErrPreferenceLimit), errors.Is(err, service.ErrDuplicatePriority):
		ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrInvalidTransition):
		ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrCaseAccessDenied):
		ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	default:
		log.Printf("internship request failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

func currentUserID(ctx *gin.Context) (uint, bool) {
	value, exists := ctx.Get(appmiddleware.ContextUserID)
	userID, ok := value.(uint)
	if !exists || !ok || userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return 0, false
	}
	return userID, true
}

func parseID(value string) (uint, error) {
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("invalid id")
	}
	return uint(id), nil
}

func caseResponse(internshipCase *model.InternshipCase) internshipCaseResponse {
	preferences := make([]internshipPreferenceResponse, 0, len(internshipCase.Preferences))
	for _, preference := range internshipCase.Preferences {
		preferences = append(preferences, preferenceResponse(preference))
	}
	response := internshipCaseResponse{
		ID: internshipCase.ID, Status: internshipCase.Status,
		PassedCredits: internshipCase.PassedCredits, Mobile: internshipCase.Mobile,
		Student: internshipCase.Student.Public(), Professor: internshipCase.Professor.Public(),
		Preferences: preferences, CreatedAt: internshipCase.CreatedAt, UpdatedAt: internshipCase.UpdatedAt,
		SubmittedAt:          internshipCase.SubmittedAt,
		SelectedPreferenceID: internshipCase.SelectedPreferenceID,
		CompanySupervisorID:  internshipCase.CompanySupervisorID,
		LetterNumber:         internshipCase.LetterNumber, LetterDate: internshipCase.LetterDate,
		InternshipSubject: internshipCase.InternshipSubject, StartDate: internshipCase.StartDate,
		WorkplaceAddress: internshipCase.WorkplaceAddress, WorkplacePhone: internshipCase.WorkplacePhone,
		CompanyConfirmedAt:   internshipCase.CompanyConfirmedAt,
		UniversityApprovedAt: internshipCase.UniversityApprovedAt, ActivatedAt: internshipCase.ActivatedAt,
	}
	if internshipCase.SelectedPreference != nil {
		selected := preferenceResponse(*internshipCase.SelectedPreference)
		response.SelectedPreference = &selected
	}
	if internshipCase.CompanySupervisor != nil {
		supervisor := internshipCase.CompanySupervisor.Public()
		response.CompanySupervisor = &supervisor
	}
	return response
}

func preferenceResponse(preference model.InternshipPreference) internshipPreferenceResponse {
	return internshipPreferenceResponse{
		ID: preference.ID, Priority: preference.Priority,
		CompanyID: preference.CompanyID, Company: preference.Company,
		ProposedCompanyName: preference.ProposedCompanyName, ProposedWebsite: preference.ProposedWebsite,
		ProposedPhone: preference.ProposedPhone, ProposedEmail: preference.ProposedEmail,
		ProposedSupervisorName: preference.ProposedSupervisorName,
		City:                   preference.City, WorkField: preference.WorkField,
	}
}
