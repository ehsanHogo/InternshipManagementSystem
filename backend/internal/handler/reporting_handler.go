package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"

	appmiddleware "internship-management-system/backend/internal/middleware"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

const maxFinalReportSize = 10 << 20

type weeklyReportRequest struct {
	WeekNumber          int    `json:"weekNumber"`
	StartDate           string `json:"startDate"`
	EndDate             string `json:"endDate"`
	ActivityDescription string `json:"activityDescription"`
}

type weeklyReviewRequest struct {
	Comment *string `json:"comment"`
}

type companyEvaluationRequest struct {
	AttendanceRating         model.EvaluationRating `json:"attendanceRating"`
	ParticipationRating      model.EvaluationRating `json:"participationRating"`
	LearningRating           model.EvaluationRating `json:"learningRating"`
	InterestRating           model.EvaluationRating `json:"interestRating"`
	PersistenceRating        model.EvaluationRating `json:"persistenceRating"`
	SuggestionRating         model.EvaluationRating `json:"suggestionRating"`
	ResourceUsageRating      model.EvaluationRating `json:"resourceUsageRating"`
	ReportQualityRating      model.EvaluationRating `json:"reportQualityRating"`
	ProjectPerformanceRating model.EvaluationRating `json:"projectPerformanceRating"`
	LeaveDays                *int                   `json:"leaveDays"`
	AbsenceDays              *int                   `json:"absenceDays"`
	Suggestions              *string                `json:"suggestions"`
}

func (handler *InternshipHandler) ListStudentWeeklyReports(ctx *gin.Context) {
	studentID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	reports, err := handler.service.ListStudentWeeklyReports(studentID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, reports)
}

func (handler *InternshipHandler) CreateWeeklyReport(ctx *gin.Context) {
	studentID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	input, ok := bindWeeklyReport(ctx)
	if !ok {
		return
	}
	report, err := handler.service.CreateWeeklyReport(studentID, input)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, report)
}

func (handler *InternshipHandler) UpdateWeeklyReport(ctx *gin.Context) {
	studentID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	reportID, err := parseID(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "شناسه گزارش هفتگی معتبر نیست."})
		return
	}
	input, ok := bindWeeklyReport(ctx)
	if !ok {
		return
	}
	report, err := handler.service.UpdateWeeklyReport(studentID, reportID, input)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, report)
}

func (handler *InternshipHandler) ListCompanyWeeklyReports(ctx *gin.Context) {
	supervisorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	caseID, ok := caseIDFromContext(ctx)
	if !ok {
		return
	}
	reports, err := handler.service.ListCompanyWeeklyReports(supervisorID, caseID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, reports)
}

func (handler *InternshipHandler) GetStudentWeeklyReport(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	reportID, err := parseID(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "شناسه گزارش معتبر نیست."})
		return
	}
	report, err := handler.service.GetStudentWeeklyReport(userID, reportID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, report)
}
func (handler *InternshipHandler) SubmitWeeklyReport(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	reportID, err := parseID(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "شناسه گزارش معتبر نیست."})
		return
	}
	report, err := handler.service.SubmitWeeklyReport(userID, reportID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, report)
}
func (handler *InternshipHandler) ListProfessorWeeklyReports(ctx *gin.Context) {
	userID, caseID, ok := professorCaseRequest(ctx)
	if !ok {
		return
	}
	reports, err := handler.service.ListProfessorWeeklyReports(userID, caseID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, reports)
}
func (handler *InternshipHandler) GetCompanyWeeklyReport(ctx *gin.Context) {
	handler.getReviewerWeeklyReport(ctx, model.RoleCompanySupervisor)
}
func (handler *InternshipHandler) GetProfessorWeeklyReport(ctx *gin.Context) {
	handler.getReviewerWeeklyReport(ctx, model.RoleProfessor)
}
func (handler *InternshipHandler) getReviewerWeeklyReport(ctx *gin.Context, role model.Role) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	caseID, ok := caseIDFromContext(ctx)
	if !ok {
		return
	}
	reportID, err := parseID(ctx.Param("reportId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "شناسه گزارش معتبر نیست."})
		return
	}
	report, err := handler.service.GetReviewerWeeklyReport(userID, caseID, reportID, role)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, report)
}
func (handler *InternshipHandler) ApproveWeeklyReportByCompany(ctx *gin.Context) {
	handler.reviewWeeklyReport(ctx, model.RoleCompanySupervisor, model.WeeklyReviewApproved)
}
func (handler *InternshipHandler) RequestWeeklyReportRevisionByCompany(ctx *gin.Context) {
	handler.reviewWeeklyReport(ctx, model.RoleCompanySupervisor, model.WeeklyReviewRevisionRequested)
}
func (handler *InternshipHandler) ApproveWeeklyReportByProfessor(ctx *gin.Context) {
	handler.reviewWeeklyReport(ctx, model.RoleProfessor, model.WeeklyReviewApproved)
}
func (handler *InternshipHandler) RequestWeeklyReportRevisionByProfessor(ctx *gin.Context) {
	handler.reviewWeeklyReport(ctx, model.RoleProfessor, model.WeeklyReviewRevisionRequested)
}
func (handler *InternshipHandler) reviewWeeklyReport(ctx *gin.Context, role model.Role, decision model.WeeklyReviewStatus) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	caseID, ok := caseIDFromContext(ctx)
	if !ok {
		return
	}
	reportID, err := parseID(ctx.Param("reportId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "شناسه گزارش معتبر نیست."})
		return
	}
	var request weeklyReviewRequest
	if err := ctx.ShouldBindJSON(&request); err != nil && !errors.Is(err, io.EOF) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "اطلاعات درخواست معتبر نیست."})
		return
	}
	var report *model.WeeklyReport
	if role == model.RoleCompanySupervisor {
		report, err = handler.service.ReviewWeeklyReportByCompany(userID, caseID, reportID, decision, request.Comment)
	} else {
		report, err = handler.service.ReviewWeeklyReportByProfessor(userID, caseID, reportID, decision, request.Comment)
	}
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, report)
}

// Keep production and integration-test route wiring identical. Role middleware
// is installed by the caller on each authenticated group.
func (handler *InternshipHandler) RegisterStudentWeeklyReportRoutes(group *gin.RouterGroup) {
	group.GET("/internship-case/weekly-reports", handler.ListStudentWeeklyReports)
	group.GET("/internship-case/weekly-reports/:id", handler.GetStudentWeeklyReport)
	group.POST("/internship-case/weekly-reports", handler.CreateWeeklyReport)
	group.PUT("/internship-case/weekly-reports/:id", handler.UpdateWeeklyReport)
	group.POST("/internship-case/weekly-reports/:id/submit", handler.SubmitWeeklyReport)
}
func (handler *InternshipHandler) RegisterCompanyWeeklyReportRoutes(group *gin.RouterGroup) {
	group.GET("/internship-cases/:id/weekly-reports", handler.ListCompanyWeeklyReports)
	group.GET("/internship-cases/:id/weekly-reports/:reportId", handler.GetCompanyWeeklyReport)
	group.POST("/internship-cases/:id/weekly-reports/:reportId/approve", handler.ApproveWeeklyReportByCompany)
	group.POST("/internship-cases/:id/weekly-reports/:reportId/request-revision", handler.RequestWeeklyReportRevisionByCompany)
}
func (handler *InternshipHandler) RegisterProfessorWeeklyReportRoutes(group *gin.RouterGroup) {
	group.GET("/internship-cases/:id/weekly-reports", handler.ListProfessorWeeklyReports)
	group.GET("/internship-cases/:id/weekly-reports/:reportId", handler.GetProfessorWeeklyReport)
	group.POST("/internship-cases/:id/weekly-reports/:reportId/approve", handler.ApproveWeeklyReportByProfessor)
	group.POST("/internship-cases/:id/weekly-reports/:reportId/request-revision", handler.RequestWeeklyReportRevisionByProfessor)
}

func (handler *InternshipHandler) GetCompanyEvaluation(ctx *gin.Context) {
	supervisorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	caseID, ok := caseIDFromContext(ctx)
	if !ok {
		return
	}
	evaluation, err := handler.service.GetCompanyEvaluation(supervisorID, caseID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, evaluation)
}

func (handler *InternshipHandler) CreateCompanyEvaluation(ctx *gin.Context) {
	supervisorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	caseID, ok := caseIDFromContext(ctx)
	if !ok {
		return
	}
	var request companyEvaluationRequest
	if err := ctx.ShouldBindJSON(&request); err != nil || request.LeaveDays == nil || request.AbsenceDays == nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "اطلاعات درخواست معتبر نیست."})
		return
	}
	evaluation, err := handler.service.CreateCompanyEvaluation(supervisorID, caseID, service.CompanyEvaluationInput{
		AttendanceRating: request.AttendanceRating, ParticipationRating: request.ParticipationRating,
		LearningRating: request.LearningRating, InterestRating: request.InterestRating,
		PersistenceRating: request.PersistenceRating, SuggestionRating: request.SuggestionRating,
		ResourceUsageRating: request.ResourceUsageRating, ReportQualityRating: request.ReportQualityRating,
		ProjectPerformanceRating: request.ProjectPerformanceRating,
		LeaveDays:                *request.LeaveDays, AbsenceDays: *request.AbsenceDays, Suggestions: request.Suggestions,
	})
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, evaluation)
}

func (handler *InternshipHandler) UploadFinalReport(ctx *gin.Context) {
	studentID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	if err := handler.service.EnsureCanUploadFinalReport(studentID); err != nil {
		handler.writeError(ctx, err)
		return
	}

	if ctx.Request.ContentLength > maxFinalReportSize+(1<<20) {
		handler.writeError(ctx, service.ErrFinalReportTooLarge)
		return
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxFinalReportSize+(1<<20))
	header, err := ctx.FormFile("file")
	if ctx.Request.MultipartForm != nil {
		defer ctx.Request.MultipartForm.RemoveAll()
	}
	if err != nil {
		var sizeErr *http.MaxBytesError
		if errors.As(err, &sizeErr) {
			handler.writeError(ctx, service.ErrFinalReportTooLarge)
		} else {
			handler.writeError(ctx, service.ErrInvalidFinalReportFile)
		}
		return
	}
	if header.Size > maxFinalReportSize {
		handler.writeError(ctx, service.ErrFinalReportTooLarge)
		return
	}
	source, err := openValidatedPDF(header)
	if err != nil {
		handler.writeError(ctx, service.ErrInvalidFinalReportFile)
		return
	}
	defer source.Close()
	if err := os.MkdirAll(handler.uploadDir, 0o750); err != nil {
		handler.writeError(ctx, fmt.Errorf("create upload directory: %w", err))
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
		handler.writeError(ctx, fmt.Errorf("create uploaded file: %w", err))
		return
	}
	written, copyErr := io.Copy(destination, io.LimitReader(source, maxFinalReportSize+1))
	closeErr := destination.Close()
	if copyErr != nil || closeErr != nil || written <= 0 || written > maxFinalReportSize {
		removeUploadedFile(path)
		if written > maxFinalReportSize {
			handler.writeError(ctx, service.ErrFinalReportTooLarge)
		} else {
			handler.writeError(ctx, service.ErrInvalidFinalReportFile)
		}
		return
	}

	file := &model.File{
		OriginalName: filepath.Base(header.Filename), StoredName: storedName, Path: path,
		MimeType: "application/pdf", SizeBytes: written, UploadedBy: studentID, UploadedAt: time.Now(),
	}
	saved, previous, err := handler.service.AttachFinalReport(studentID, file)
	if err != nil {
		removeUploadedFile(path)
		handler.writeError(ctx, err)
		return
	}
	if previous != nil {
		removeUploadedFile(previous.Path)
	}
	ctx.JSON(http.StatusCreated, saved)
}

func (handler *InternshipHandler) DownloadFile(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	roleValue, exists := ctx.Get(appmiddleware.ContextRole)
	role, valid := roleValue.(model.Role)
	if !exists || !valid {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "ورود به سامانه الزامی است."})
		return
	}
	fileID, err := parseID(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "شناسه فایل معتبر نیست."})
		return
	}
	file, err := handler.service.GetAccessibleFile(userID, role, fileID)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	if _, err := os.Stat(file.Path); err != nil {
		if os.IsNotExist(err) {
			handler.writeError(ctx, service.ErrFileNotFound)
		} else {
			handler.writeError(ctx, err)
		}
		return
	}
	ctx.Header("Content-Type", file.MimeType)
	ctx.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": file.OriginalName}))
	ctx.File(file.Path)
}

func bindWeeklyReport(ctx *gin.Context) (service.WeeklyReportInput, bool) {
	var request weeklyReportRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "اطلاعات درخواست معتبر نیست."})
		return service.WeeklyReportInput{}, false
	}
	startDate, err := parseDate(request.StartDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "تاریخ شروع معتبر نیست."})
		return service.WeeklyReportInput{}, false
	}
	endDate, err := parseDate(request.EndDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "تاریخ پایان معتبر نیست."})
		return service.WeeklyReportInput{}, false
	}
	return service.WeeklyReportInput{
		WeekNumber: request.WeekNumber, StartDate: startDate, EndDate: endDate,
		ActivityDescription: request.ActivityDescription,
	}, true
}

func uniqueStoredName() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate stored filename: %w", err)
	}
	return hex.EncodeToString(bytes) + ".pdf", nil
}

func submittedWeeklyReports(reports []model.WeeklyReport) []model.WeeklyReport {
	visible := []model.WeeklyReport{}
	for _, report := range reports {
		if report.SubmittedAt != nil {
			visible = append(visible, report)
		}
	}
	return visible
}
