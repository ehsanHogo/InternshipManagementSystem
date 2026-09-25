package handler

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

const maxResumeSize = 5 << 20

type OpportunityApplicationHandler struct {
	service   *service.OpportunityApplicationService
	uploadDir string
}

type reviewApplicationRequest struct {
	CompanyComment *string `json:"companyComment"`
}

type applicationOpportunityResponse struct {
	ID        uint                       `json:"id"`
	Title     string                     `json:"title"`
	WorkField string                     `json:"workField"`
	Location  string                     `json:"location"`
	Company   opportunityCompanyResponse `json:"company"`
}

type applicationResumeResponse struct {
	ID           uint      `json:"id"`
	OriginalName string    `json:"originalName"`
	UploadedAt   time.Time `json:"uploadedAt"`
}

type applicationStudentResponse struct {
	ID       uint   `json:"id"`
	FullName string `json:"fullName"`
	Email    string `json:"email"`
}

type acceptedOpportunityApplicationResponse struct {
	ID          uint                           `json:"id"`
	Status      model.ApplicationStatus        `json:"status"`
	Opportunity applicationOpportunityResponse `json:"opportunity"`
}

type opportunityApplicationResponse struct {
	ID             uint                           `json:"id"`
	Status         model.ApplicationStatus        `json:"status"`
	CompanyComment *string                        `json:"companyComment"`
	AppliedAt      time.Time                      `json:"appliedAt"`
	ReviewedAt     *time.Time                     `json:"reviewedAt"`
	Opportunity    applicationOpportunityResponse `json:"opportunity"`
	Student        *applicationStudentResponse    `json:"student,omitempty"`
	Resume         applicationResumeResponse      `json:"resume"`
}

func NewOpportunityApplicationHandler(applicationService *service.OpportunityApplicationService, uploadDirs ...string) *OpportunityApplicationHandler {
	uploadDir := "uploads"
	if len(uploadDirs) > 0 && uploadDirs[0] != "" {
		uploadDir = uploadDirs[0]
	}
	return &OpportunityApplicationHandler{service: applicationService, uploadDir: uploadDir}
}

func (handler *OpportunityApplicationHandler) Apply(ctx *gin.Context) {
	studentID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	opportunityID, err := parseID(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_OPPORTUNITY_ID", "error": "شناسه فرصت کارآموزی معتبر نیست."})
		return
	}
	if err := handler.service.EnsureCanApply(studentID, opportunityID); err != nil {
		handler.writeError(ctx, err)
		return
	}
	if ctx.Request.ContentLength > maxResumeSize+(1<<20) {
		handler.writeError(ctx, service.ErrResumeTooLarge)
		return
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxResumeSize+(1<<20))
	header, err := ctx.FormFile("resume")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "too large") {
			handler.writeError(ctx, service.ErrResumeTooLarge)
		} else {
			handler.writeError(ctx, service.ErrResumeRequired)
		}
		return
	}
	if header.Size > maxResumeSize {
		handler.writeError(ctx, service.ErrResumeTooLarge)
		return
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(header.Header.Get("Content-Type"), ";")[0]))
	if header.Size <= 0 || !strings.EqualFold(filepath.Ext(header.Filename), ".pdf") || contentType != "application/pdf" {
		handler.writeError(ctx, service.ErrInvalidResumeFile)
		return
	}
	source, err := header.Open()
	if err != nil {
		handler.writeError(ctx, service.ErrInvalidResumeFile)
		return
	}
	defer source.Close()
	signature := make([]byte, 5)
	if _, err := io.ReadFull(source, signature); err != nil || string(signature) != "%PDF-" {
		handler.writeError(ctx, service.ErrInvalidResumeFile)
		return
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		handler.writeError(ctx, service.ErrInvalidResumeFile)
		return
	}
	if err := os.MkdirAll(handler.uploadDir, 0o750); err != nil {
		handler.writeError(ctx, fmt.Errorf("create resume upload directory: %w", err))
		return
	}
	storedName, err := uniqueStoredName()
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	path := filepath.Join(handler.uploadDir, storedName)
	destination, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		handler.writeError(ctx, fmt.Errorf("create resume file: %w", err))
		return
	}
	written, copyErr := io.Copy(destination, io.LimitReader(source, maxResumeSize+1))
	closeErr := destination.Close()
	if copyErr != nil || closeErr != nil || written <= 0 || written > maxResumeSize {
		_ = os.Remove(path)
		if written > maxResumeSize {
			handler.writeError(ctx, service.ErrResumeTooLarge)
		} else {
			handler.writeError(ctx, service.ErrInvalidResumeFile)
		}
		return
	}
	file := &model.File{
		OriginalName: filepath.Base(header.Filename), StoredName: storedName, Path: path,
		MimeType: "application/pdf", SizeBytes: written, UploadedBy: studentID, UploadedAt: time.Now(),
	}
	application, err := handler.service.Apply(studentID, opportunityID, file)
	if err != nil {
		_ = os.Remove(path)
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, applicationView(*application, false))
}

func (handler *OpportunityApplicationHandler) ListStudent(ctx *gin.Context) {
	studentID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	applications, err := handler.service.ListStudent(studentID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	response := make([]opportunityApplicationResponse, 0, len(applications))
	for _, application := range applications {
		response = append(response, applicationView(application, false))
	}
	ctx.JSON(http.StatusOK, response)
}

func (handler *OpportunityApplicationHandler) ListAcceptedStudent(ctx *gin.Context) {
	studentID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	applications, err := handler.service.ListAcceptedStudent(studentID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	response := make([]acceptedOpportunityApplicationResponse, 0, len(applications))
	for _, application := range applications {
		response = append(response, acceptedApplicationView(application))
	}
	ctx.JSON(http.StatusOK, response)
}

func (handler *OpportunityApplicationHandler) GetStudent(ctx *gin.Context) {
	studentID, applicationID, ok := requestUserAndApplicationID(ctx)
	if !ok {
		return
	}
	application, err := handler.service.GetStudent(studentID, applicationID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, applicationView(*application, false))
}

func (handler *OpportunityApplicationHandler) ListCompany(ctx *gin.Context) {
	supervisorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	opportunityID, err := parseID(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_OPPORTUNITY_ID", "error": "شناسه فرصت کارآموزی معتبر نیست."})
		return
	}
	applications, err := handler.service.ListCompany(supervisorID, opportunityID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	response := make([]opportunityApplicationResponse, 0, len(applications))
	for _, application := range applications {
		response = append(response, applicationView(application, true))
	}
	ctx.JSON(http.StatusOK, response)
}

func (handler *OpportunityApplicationHandler) GetCompany(ctx *gin.Context) {
	supervisorID, applicationID, ok := requestUserAndApplicationID(ctx)
	if !ok {
		return
	}
	application, err := handler.service.GetCompany(supervisorID, applicationID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, applicationView(*application, true))
}

func (handler *OpportunityApplicationHandler) Accept(ctx *gin.Context) {
	handler.review(ctx, model.ApplicationStatusAccepted)
}

func (handler *OpportunityApplicationHandler) Reject(ctx *gin.Context) {
	handler.review(ctx, model.ApplicationStatusRejected)
}

func (handler *OpportunityApplicationHandler) review(ctx *gin.Context, status model.ApplicationStatus) {
	supervisorID, applicationID, ok := requestUserAndApplicationID(ctx)
	if !ok {
		return
	}
	var request reviewApplicationRequest
	if err := ctx.ShouldBindJSON(&request); err != nil && !errors.Is(err, io.EOF) {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_REVIEW", "error": "اطلاعات بررسی درخواست معتبر نیست."})
		return
	}
	application, err := handler.service.Review(supervisorID, applicationID, status, request.CompanyComment)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, applicationView(*application, true))
}

func requestUserAndApplicationID(ctx *gin.Context) (uint, uint, bool) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return 0, 0, false
	}
	applicationID, err := parseID(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_APPLICATION_ID", "error": "شناسه درخواست معتبر نیست."})
		return 0, 0, false
	}
	return userID, applicationID, true
}

func acceptedApplicationView(application model.OpportunityApplication) acceptedOpportunityApplicationResponse {
	return acceptedOpportunityApplicationResponse{
		ID: application.ID, Status: application.Status,
		Opportunity: applicationOpportunityResponse{
			ID: application.Opportunity.ID, Title: application.Opportunity.Title,
			WorkField: application.Opportunity.WorkField, Location: application.Opportunity.Location,
			Company: opportunityCompanyResponse{
				ID: application.Opportunity.Company.ID, Name: application.Opportunity.Company.Name,
				IsApproved: application.Opportunity.Company.IsApproved,
			},
		},
	}
}

func applicationView(application model.OpportunityApplication, includeStudent bool) opportunityApplicationResponse {
	response := opportunityApplicationResponse{
		ID: application.ID, Status: application.Status, CompanyComment: application.CompanyComment,
		AppliedAt: application.AppliedAt, ReviewedAt: application.ReviewedAt,
		Opportunity: applicationOpportunityResponse{
			ID: application.Opportunity.ID, Title: application.Opportunity.Title,
			WorkField: application.Opportunity.WorkField, Location: application.Opportunity.Location,
			Company: opportunityCompanyResponse{
				ID: application.Opportunity.Company.ID, Name: application.Opportunity.Company.Name,
				IsApproved: application.Opportunity.Company.IsApproved,
			},
		},
		Resume: applicationResumeResponse{
			ID: application.ResumeFile.ID, OriginalName: application.ResumeFile.OriginalName,
			UploadedAt: application.ResumeFile.UploadedAt,
		},
	}
	if includeStudent {
		response.Student = &applicationStudentResponse{
			ID: application.Student.ID, FullName: application.Student.FullName, Email: application.Student.Email,
		}
	}
	return response
}

func (handler *OpportunityApplicationHandler) writeError(ctx *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := "INTERNAL_ERROR"
	message := "خطایی در سرور رخ داد."
	switch {
	case errors.Is(err, service.ErrApplicationAlreadyExists):
		status, code, message = http.StatusConflict, "APPLICATION_ALREADY_EXISTS", "درخواست شما برای این فرصت قبلاً ثبت شده است."
	case errors.Is(err, service.ErrOpportunityNotOpen):
		status, code, message = http.StatusConflict, "OPPORTUNITY_NOT_OPEN", "این فرصت کارآموزی باز نیست و امکان ارسال درخواست جدید وجود ندارد."
	case errors.Is(err, service.ErrApplicationNotPending):
		status, code, message = http.StatusConflict, "APPLICATION_NOT_PENDING", "این درخواست قبلاً بررسی شده و تصمیم آن قابل تغییر نیست."
	case errors.Is(err, service.ErrResumeRequired):
		status, code, message = http.StatusBadRequest, "RESUME_REQUIRED", "انتخاب رزومه با قالب پی‌دی‌اف الزامی است."
	case errors.Is(err, service.ErrInvalidResumeFile):
		status, code, message = http.StatusBadRequest, "INVALID_RESUME_FILE", "فایل رزومه باید یک فایل پی‌دی‌اف معتبر باشد."
	case errors.Is(err, service.ErrResumeTooLarge):
		status, code, message = http.StatusRequestEntityTooLarge, "RESUME_TOO_LARGE", "حجم فایل رزومه نباید بیشتر از ۵ مگابایت باشد."
	case errors.Is(err, service.ErrInternshipCaseAlreadyInProgress):
		status, code, message = http.StatusConflict, "INTERNSHIP_CASE_ALREADY_IN_PROGRESS", "پرونده کارآموزی شما در حال بررسی یا اجرا است و تا زمان بسته شدن آن امکان ارسال درخواست جدید برای فرصت‌های کارآموزی وجود ندارد."
	case errors.Is(err, service.ErrInternshipAlreadyCompleted):
		status, code, message = http.StatusConflict, "INTERNSHIP_ALREADY_COMPLETED", "شما قبلاً دوره کارآموزی خود را با موفقیت تکمیل کرده‌اید و امکان ثبت درخواست جدید ندارید."
	case errors.Is(err, service.ErrOpportunityApplicationNotFound):
		status, code, message = http.StatusNotFound, "OPPORTUNITY_APPLICATION_NOT_FOUND", "درخواست فرصت کارآموزی یافت نشد."
	case errors.Is(err, service.ErrOpportunityNotFound):
		status, code, message = http.StatusNotFound, "OPPORTUNITY_NOT_FOUND", "فرصت کارآموزی یافت نشد."
	case errors.Is(err, service.ErrOpportunityCompanyMissing):
		status, code, message = http.StatusBadRequest, "COMPANY_REQUIRED", "شرکت معتبری به حساب شما متصل نیست."
	default:
		log.Printf("opportunity application request failed: %v", err)
	}
	ctx.JSON(status, gin.H{"code": code, "error": message})
}
