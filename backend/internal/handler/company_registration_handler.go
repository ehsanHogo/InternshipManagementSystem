package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

type CompanyRegistrationHandler struct {
	service *service.CompanyRegistrationService
}

func NewCompanyRegistrationHandler(s *service.CompanyRegistrationService) *CompanyRegistrationHandler {
	return &CompanyRegistrationHandler{service: s}
}
func (handler *CompanyRegistrationHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/company-registrations", handler.List)
	group.GET("/company-registrations/:id", handler.Detail)
	group.POST("/company-registrations/:id/approve", handler.Approve)
	group.POST("/company-registrations/:id/reject", handler.Reject)
}
func (handler *CompanyRegistrationHandler) List(ctx *gin.Context) {
	companies, err := handler.service.List(model.CompanyRegistrationStatus(ctx.Query("status")))
	if err != nil {
		writeCompanyAccountError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, companies)
}
func (handler *CompanyRegistrationHandler) Detail(ctx *gin.Context) {
	id, err := parseID(ctx.Param("id"))
	if err != nil {
		writeCompanyAccountError(ctx, service.ErrInvalidCompanyRegistration)
		return
	}
	detail, err := handler.service.Detail(id)
	if err != nil {
		writeCompanyAccountError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, detail)
}
func (handler *CompanyRegistrationHandler) Approve(ctx *gin.Context) {
	handler.review(ctx, model.CompanyRegistrationStatusApproved, "")
}
func (handler *CompanyRegistrationHandler) Reject(ctx *gin.Context) {
	var request struct {
		Reason string `json:"reason"`
	}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		writeCompanyAccountError(ctx, service.ErrRegistrationReviewReason)
		return
	}
	handler.review(ctx, model.CompanyRegistrationStatusRejected, request.Reason)
}
func (handler *CompanyRegistrationHandler) review(ctx *gin.Context, status model.CompanyRegistrationStatus, reason string) {
	adminID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	id, err := parseID(ctx.Param("id"))
	if err != nil {
		writeCompanyAccountError(ctx, service.ErrInvalidCompanyRegistration)
		return
	}
	company, err := handler.service.Review(adminID, id, status, reason)
	if err != nil {
		writeCompanyAccountError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, company)
}

type companyProfileUpdateRequest struct {
	Company    companyRegistrationDetailsRequest `json:"company"`
	Supervisor struct {
		FullName string `json:"fullName"`
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		JobTitle string `json:"jobTitle"`
	} `json:"supervisor"`
}

func (handler *CompanyAccountHandler) UpdateProfile(ctx *gin.Context) {
	id, ok := currentUserID(ctx)
	if !ok {
		return
	}
	var request companyProfileUpdateRequest
	// Explicit DTO and strict decoding reject attempts to mutate approval, role,
	// membership, password, or audit fields rather than silently accepting them.
	decoder := json.NewDecoder(http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeCompanyAccountError(ctx, service.ErrInvalidCompanyRegistration)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeCompanyAccountError(ctx, service.ErrInvalidCompanyRegistration)
		return
	}
	account, err := handler.service.UpdateProfile(id, service.CompanyRegistrationInput{
		Company:    service.CompanyInput{Name: request.Company.Name, NationalID: request.Company.NationalID, EconomicCode: request.Company.EconomicCode, Website: request.Company.Website, Phone: request.Company.Phone, Email: request.Company.Email, Address: request.Company.Address},
		Supervisor: service.CompanySupervisorRegistrationInput{FullName: request.Supervisor.FullName, Email: request.Supervisor.Email, Phone: request.Supervisor.Phone, JobTitle: request.Supervisor.JobTitle},
	})
	if err != nil {
		writeCompanyAccountError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, companyAccountResponse{Company: account.Company, Supervisor: account.Supervisor.Public()})
}
func (handler *CompanyAccountHandler) Resubmit(ctx *gin.Context) {
	id, ok := currentUserID(ctx)
	if !ok {
		return
	}
	account, err := handler.service.Resubmit(id)
	if err != nil {
		writeCompanyAccountError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, companyAccountResponse{Company: account.Company, Supervisor: account.Supervisor.Public()})
}
