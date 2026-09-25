package handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

type OpportunityHandler struct {
	service      *service.OpportunityService
	applications *service.OpportunityApplicationService
}

type opportunityRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	WorkField   string `json:"workField"`
	Location    string `json:"location"`
}

type companyOpportunityResponse struct {
	ID          uint                    `json:"id"`
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	WorkField   string                  `json:"workField"`
	Location    string                  `json:"location"`
	Status      model.OpportunityStatus `json:"status"`
	CreatedAt   time.Time               `json:"createdAt"`
	UpdatedAt   time.Time               `json:"updatedAt"`
}

type opportunityCompanyResponse struct {
	ID         uint    `json:"id"`
	Name       string  `json:"name"`
	Website    *string `json:"website,omitempty"`
	Phone      *string `json:"phone,omitempty"`
	Email      *string `json:"email,omitempty"`
	Address    *string `json:"address,omitempty"`
	IsApproved bool    `json:"isApproved"`
}

type existingApplicationResponse struct {
	ID     uint                    `json:"id"`
	Status model.ApplicationStatus `json:"status"`
}

type studentOpportunityResponse struct {
	ID                   uint                         `json:"id"`
	Title                string                       `json:"title"`
	Description          string                       `json:"description"`
	WorkField            string                       `json:"workField"`
	Location             string                       `json:"location"`
	CreatedAt            time.Time                    `json:"createdAt"`
	Company              opportunityCompanyResponse   `json:"company"`
	CanApply             bool                         `json:"canApply"`
	ApplyRestrictionCode string                       `json:"applyRestrictionCode,omitempty"`
	ExistingApplication  *existingApplicationResponse `json:"existingApplication,omitempty"`
}

func NewOpportunityHandler(opportunityService *service.OpportunityService, applicationServices ...*service.OpportunityApplicationService) *OpportunityHandler {
	handler := &OpportunityHandler{service: opportunityService}
	if len(applicationServices) > 0 {
		handler.applications = applicationServices[0]
	}
	return handler
}

func (handler *OpportunityHandler) Create(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	request, ok := bindOpportunityRequest(ctx)
	if !ok {
		return
	}
	opportunity, err := handler.service.Create(userID, request.input())
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, companyOpportunityView(*opportunity))
}

func (handler *OpportunityHandler) ListCompany(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	opportunities, err := handler.service.ListCompany(userID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	response := make([]companyOpportunityResponse, 0, len(opportunities))
	for _, opportunity := range opportunities {
		response = append(response, companyOpportunityView(opportunity))
	}
	ctx.JSON(http.StatusOK, response)
}

func (handler *OpportunityHandler) GetCompany(ctx *gin.Context) {
	userID, opportunityID, ok := requestUserAndOpportunityID(ctx)
	if !ok {
		return
	}
	opportunity, err := handler.service.GetCompany(userID, opportunityID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, companyOpportunityView(*opportunity))
}

func (handler *OpportunityHandler) Update(ctx *gin.Context) {
	userID, opportunityID, ok := requestUserAndOpportunityID(ctx)
	if !ok {
		return
	}
	request, ok := bindOpportunityRequest(ctx)
	if !ok {
		return
	}
	opportunity, err := handler.service.Update(userID, opportunityID, request.input())
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, companyOpportunityView(*opportunity))
}

func (handler *OpportunityHandler) Close(ctx *gin.Context) {
	userID, opportunityID, ok := requestUserAndOpportunityID(ctx)
	if !ok {
		return
	}
	opportunity, err := handler.service.Close(userID, opportunityID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, companyOpportunityView(*opportunity))
}

func (handler *OpportunityHandler) ListStudent(ctx *gin.Context) {
	opportunities, err := handler.service.ListStudent()
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	response := make([]studentOpportunityResponse, 0, len(opportunities))
	for _, opportunity := range opportunities {
		response = append(response, studentOpportunityView(opportunity))
	}
	ctx.JSON(http.StatusOK, response)
}

func (handler *OpportunityHandler) GetStudent(ctx *gin.Context) {
	studentID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	opportunityID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || opportunityID == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_OPPORTUNITY_ID", "error": "شناسه فرصت کارآموزی معتبر نیست."})
		return
	}
	opportunity, err := handler.service.GetStudent(uint(opportunityID))
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	response := studentOpportunityView(*opportunity)
	response.CanApply = true
	if handler.applications != nil {
		eligibility, eligibilityErr := handler.applications.CheckEligibility(studentID, uint(opportunityID))
		if eligibilityErr != nil {
			handler.writeError(ctx, eligibilityErr)
			return
		}
		response.CanApply = eligibility.CanApply
		response.ApplyRestrictionCode = eligibility.RestrictionCode
		if eligibility.ExistingApplicationID != nil && eligibility.ExistingStatus != nil {
			response.ExistingApplication = &existingApplicationResponse{
				ID: *eligibility.ExistingApplicationID, Status: *eligibility.ExistingStatus,
			}
		}
	}
	ctx.JSON(http.StatusOK, response)
}

func bindOpportunityRequest(ctx *gin.Context) (opportunityRequest, bool) {
	var request opportunityRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_OPPORTUNITY", "error": "اطلاعات فرصت کارآموزی معتبر نیست."})
		return opportunityRequest{}, false
	}
	return request, true
}

func (request opportunityRequest) input() service.OpportunityInput {
	return service.OpportunityInput{
		Title: request.Title, Description: request.Description,
		WorkField: request.WorkField, Location: request.Location,
	}
}

func requestUserAndOpportunityID(ctx *gin.Context) (uint, uint, bool) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return 0, 0, false
	}
	opportunityID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || opportunityID == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_OPPORTUNITY_ID", "error": "شناسه فرصت کارآموزی معتبر نیست."})
		return 0, 0, false
	}
	return userID, uint(opportunityID), true
}

func companyOpportunityView(opportunity model.InternshipOpportunity) companyOpportunityResponse {
	return companyOpportunityResponse{
		ID: opportunity.ID, Title: opportunity.Title, Description: opportunity.Description,
		WorkField: opportunity.WorkField, Location: opportunity.Location, Status: opportunity.Status,
		CreatedAt: opportunity.CreatedAt, UpdatedAt: opportunity.UpdatedAt,
	}
}

func studentOpportunityView(opportunity model.InternshipOpportunity) studentOpportunityResponse {
	return studentOpportunityResponse{
		ID: opportunity.ID, Title: opportunity.Title, Description: opportunity.Description,
		WorkField: opportunity.WorkField, Location: opportunity.Location, CreatedAt: opportunity.CreatedAt,
		Company: opportunityCompanyResponse{
			ID: opportunity.Company.ID, Name: opportunity.Company.Name, Website: opportunity.Company.Website,
			Phone: opportunity.Company.Phone, Email: opportunity.Company.Email,
			Address: opportunity.Company.Address, IsApproved: opportunity.Company.IsApproved,
		},
	}
}

func (handler *OpportunityHandler) writeError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidOpportunity):
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_OPPORTUNITY", "error": "عنوان، توضیحات، زمینه کاری و محل کارآموزی را به‌طور کامل وارد کنید."})
	case errors.Is(err, service.ErrOpportunityNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"code": "OPPORTUNITY_NOT_FOUND", "error": "فرصت کارآموزی یافت نشد."})
	case errors.Is(err, service.ErrOpportunityAlreadyClosed):
		ctx.JSON(http.StatusConflict, gin.H{"code": "OPPORTUNITY_ALREADY_CLOSED", "error": "این فرصت کارآموزی قبلاً بسته شده است."})
	case errors.Is(err, service.ErrOpportunityCompanyMissing):
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "COMPANY_REQUIRED", "error": "شرکت معتبری به حساب شما متصل نیست."})
	default:
		log.Printf("opportunity request failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "error": "خطایی در سرور رخ داد."})
	}
}
