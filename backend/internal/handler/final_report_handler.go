package handler

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"internship-management-system/backend/internal/model"
)

func (handler *InternshipHandler) RegisterStudentFinalReportRoutes(group *gin.RouterGroup) {
	group.GET("/internship-case/final-report", handler.GetStudentFinalReport)
	group.POST("/internship-case/final-report", handler.UploadFinalReport)
}

func (handler *InternshipHandler) RegisterProfessorFinalReportRoutes(group *gin.RouterGroup) {
	group.GET("/internship-cases/:id/final-report", handler.GetProfessorFinalReport)
	group.POST("/internship-cases/:id/final-report/approve", handler.ApproveFinalReport)
	group.POST("/internship-cases/:id/final-report/request-revision", handler.RequestFinalReportRevision)
}

func (handler *InternshipHandler) GetStudentFinalReport(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	report, err := handler.service.GetStudentFinalReport(userID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, report)
}

func (handler *InternshipHandler) GetProfessorFinalReport(ctx *gin.Context) {
	professorID, caseID, ok := professorCaseRequest(ctx)
	if !ok {
		return
	}
	report, err := handler.service.GetProfessorFinalReport(professorID, caseID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, report)
}

func (handler *InternshipHandler) ApproveFinalReport(ctx *gin.Context) {
	handler.reviewFinalReport(ctx, model.FinalReportApproved)
}

func (handler *InternshipHandler) RequestFinalReportRevision(ctx *gin.Context) {
	handler.reviewFinalReport(ctx, model.FinalReportRevisionRequested)
}

func (handler *InternshipHandler) reviewFinalReport(ctx *gin.Context, status model.FinalReportStatus) {
	professorID, caseID, ok := professorCaseRequest(ctx)
	if !ok {
		return
	}
	var request weeklyReviewRequest
	if err := ctx.ShouldBindJSON(&request); err != nil && !errors.Is(err, io.EOF) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "اطلاعات بررسی گزارش نهایی معتبر نیست."})
		return
	}
	report, err := handler.service.ReviewFinalReport(professorID, caseID, status, request.Comment)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, report)
}
