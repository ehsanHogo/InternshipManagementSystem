package handler

import (
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	appmiddleware "internship-management-system/backend/internal/middleware"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

type CompanyApprovalHandler struct {
	service *service.CompanyApprovalService
}

func NewCompanyApprovalHandler(approvalService *service.CompanyApprovalService) *CompanyApprovalHandler {
	return &CompanyApprovalHandler{service: approvalService}
}

// Role guards are part of registration so production and integration tests share them.
// The supplied group must already have RequireAuth installed.
func (handler *CompanyApprovalHandler) RegisterUniversityRoutes(group *gin.RouterGroup) {
	companies := group.Group("/companies", appmiddleware.RequireRole(model.RoleUniversitySupervisor))
	companies.GET("/eligible-for-approval", handler.ListEligible)
	companies.GET("/approved", handler.ListApproved)
	companies.GET("/:id", handler.GetCompany)
	companies.POST("/:id/approve", handler.Approve)
}

func (handler *CompanyApprovalHandler) RegisterStudentRoutes(group *gin.RouterGroup) {
	group.GET("/approved-companies", appmiddleware.RequireRole(model.RoleStudent), handler.ListStudentApproved)
}

func (handler *CompanyApprovalHandler) ListEligible(ctx *gin.Context) {
	companies, err := handler.service.ListEligibleCompanies()
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, companies)
}

func (handler *CompanyApprovalHandler) ListApproved(ctx *gin.Context) {
	companies, err := handler.service.ListApprovedCompanies()
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, companies)
}

func (handler *CompanyApprovalHandler) ListStudentApproved(ctx *gin.Context) {
	companies, err := handler.service.ListStudentApprovedCompanies()
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, companies)
}

func (handler *CompanyApprovalHandler) GetCompany(ctx *gin.Context) {
	id, ok := approvalCompanyID(ctx)
	if !ok {
		return
	}
	company, err := handler.service.GetCompany(id)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, company)
}

func (handler *CompanyApprovalHandler) Approve(ctx *gin.Context) {
	id, ok := approvalCompanyID(ctx)
	if !ok {
		return
	}
	if ctx.Request.Body != nil {
		body, err := io.ReadAll(http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 1024))
		if err != nil || strings.TrimSpace(string(body)) != "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_COMPANY_APPROVAL", "error": "برای تأیید شرکت بدنه درخواست ارسال نکنید."})
			return
		}
	}
	company, err := handler.service.ApproveCompany(id)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, company)
}

func approvalCompanyID(ctx *gin.Context) (uint, bool) {
	id, err := parseID(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_COMPANY_ID", "error": "شناسه شرکت معتبر نیست."})
		return 0, false
	}
	return id, true
}

func (handler *CompanyApprovalHandler) writeError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrApprovalCompanyNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"code": "COMPANY_NOT_FOUND", "error": "شرکت موردنظر یافت نشد."})
	case errors.Is(err, service.ErrCompanyAlreadyApproved):
		ctx.JSON(http.StatusConflict, gin.H{"code": "COMPANY_ALREADY_APPROVED", "error": "این شرکت قبلاً مورد تأیید دانشکده قرار گرفته است."})
	case errors.Is(err, service.ErrCompanyNotEligibleForApproval):
		ctx.JSON(http.StatusConflict, gin.H{"code": "COMPANY_NOT_ELIGIBLE_FOR_APPROVAL", "error": "این شرکت هنوز سابقه کارآموزی موفق مورد نیاز برای تأیید دانشکده را ندارد."})
	default:
		log.Printf("company approval request failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "error": "خطایی در سرور رخ داد."})
	}
}
