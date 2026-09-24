package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	appmiddleware "internship-management-system/backend/internal/middleware"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

type CompanyAccountHandler struct {
	service *service.CompanyAccountService
}

type companySupervisorRegistrationRequest struct {
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Phone    string `json:"phone"`
	JobTitle string `json:"jobTitle"`
}

type companyRegistrationDetailsRequest struct {
	Name         string `json:"name"`
	NationalID   string `json:"nationalId"`
	EconomicCode string `json:"economicCode"`
	Website      string `json:"website"`
	Phone        string `json:"phone"`
	Email        string `json:"email"`
	Address      string `json:"address"`
}

type companyRegistrationRequest struct {
	Supervisor companySupervisorRegistrationRequest `json:"supervisor"`
	Company    companyRegistrationDetailsRequest    `json:"company"`
}

type companyAccountResponse struct {
	Company    model.Company    `json:"company"`
	Supervisor model.PublicUser `json:"supervisor"`
}

type companyRegistrationResponse struct {
	Message string                 `json:"message"`
	Account companyAccountResponse `json:"account"`
}

func NewCompanyAccountHandler(companyAccountService *service.CompanyAccountService) *CompanyAccountHandler {
	return &CompanyAccountHandler{service: companyAccountService}
}

func (handler *CompanyAccountHandler) Register(ctx *gin.Context) {
	var request companyRegistrationRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		writeCompanyAccountError(ctx, service.ErrInvalidCompanyRegistration)
		return
	}

	account, err := handler.service.Register(service.CompanyRegistrationInput{
		Supervisor: service.CompanySupervisorRegistrationInput{
			FullName: request.Supervisor.FullName,
			Email:    request.Supervisor.Email,
			Password: request.Supervisor.Password,
			Phone:    request.Supervisor.Phone,
			JobTitle: request.Supervisor.JobTitle,
		},
		Company: service.CompanyInput{
			Name:         request.Company.Name,
			NationalID:   request.Company.NationalID,
			EconomicCode: request.Company.EconomicCode,
			Website:      request.Company.Website,
			Phone:        request.Company.Phone,
			Email:        request.Company.Email,
			Address:      request.Company.Address,
		},
	})
	if err != nil {
		writeCompanyAccountError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, companyRegistrationResponse{
		Message: "ثبت‌نام شرکت با موفقیت انجام شد.",
		Account: companyAccountResponse{Company: account.Company, Supervisor: account.Supervisor.Public()},
	})
}

func (handler *CompanyAccountHandler) Profile(ctx *gin.Context) {
	userID, exists := ctx.Get(appmiddleware.ContextUserID)
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"code": "AUTHENTICATION_REQUIRED", "error": "ورود به سامانه الزامی است."})
		return
	}
	id, ok := userID.(uint)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"code": "AUTHENTICATION_REQUIRED", "error": "ورود به سامانه الزامی است."})
		return
	}

	account, err := handler.service.GetProfile(id)
	if err != nil {
		writeCompanyAccountError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, companyAccountResponse{Company: account.Company, Supervisor: account.Supervisor.Public()})
}

func writeCompanyAccountError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidCompanyRegistration):
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_REGISTRATION", "error": "اطلاعات واردشده کامل یا معتبر نیست."})
	case errors.Is(err, service.ErrEmailAlreadyExists):
		ctx.JSON(http.StatusConflict, gin.H{"code": "EMAIL_ALREADY_EXISTS", "error": "این ایمیل قبلاً در سامانه ثبت شده است."})
	case errors.Is(err, service.ErrNationalIDAlreadyExists):
		ctx.JSON(http.StatusConflict, gin.H{"code": "NATIONAL_ID_ALREADY_EXISTS", "error": "شرکتی با این شناسه ملی قبلاً ثبت شده است."})
	case errors.Is(err, service.ErrEconomicCodeAlreadyExists):
		ctx.JSON(http.StatusConflict, gin.H{"code": "ECONOMIC_CODE_ALREADY_EXISTS", "error": "شرکتی با این کد اقتصادی قبلاً ثبت شده است."})
	case errors.Is(err, service.ErrCompanyNameAlreadyExists):
		ctx.JSON(http.StatusConflict, gin.H{"code": "COMPANY_NAME_ALREADY_EXISTS", "error": "شرکتی با این نام قبلاً ثبت شده است."})
	case errors.Is(err, service.ErrCompanyProfileNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"code": "COMPANY_PROFILE_NOT_FOUND", "error": "اطلاعات شرکت برای این حساب یافت نشد."})
	default:
		log.Printf("company account request failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "error": "خطایی در سرور رخ داد."})
	}
}
