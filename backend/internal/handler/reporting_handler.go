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
	"strings"
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

type confirmWeeklyReportRequest struct {
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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid weekly report id"})
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

func (handler *InternshipHandler) ConfirmWeeklyReport(ctx *gin.Context) {
	supervisorID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	caseID, ok := caseIDFromContext(ctx)
	if !ok {
		return
	}
	reportID, err := parseID(ctx.Param("reportId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid weekly report id"})
		return
	}
	var request confirmWeeklyReportRequest
	if err := ctx.ShouldBindJSON(&request); err != nil && !errors.Is(err, io.EOF) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	report, err := handler.service.ConfirmWeeklyReport(supervisorID, caseID, reportID, request.Comment)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, report)
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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
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
	if err := handler.service.EnsureStudentActiveCase(studentID); err != nil {
		handler.writeError(ctx, err)
		return
	}

	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxFinalReportSize+(1<<20))
	header, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "PDF file is required"})
		return
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(header.Header.Get("Content-Type"), ";")[0]))
	if !strings.EqualFold(filepath.Ext(header.Filename), ".pdf") || contentType != "application/pdf" || header.Size <= 0 || header.Size > maxFinalReportSize {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "a PDF file up to 10 MB is required"})
		return
	}
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
	source, err := header.Open()
	if err != nil {
		handler.writeError(ctx, fmt.Errorf("open upload: %w", err))
		return
	}
	defer source.Close()
	destination, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		handler.writeError(ctx, fmt.Errorf("create uploaded file: %w", err))
		return
	}
	written, copyErr := io.Copy(destination, io.LimitReader(source, maxFinalReportSize+1))
	closeErr := destination.Close()
	if copyErr != nil || closeErr != nil || written <= 0 || written > maxFinalReportSize {
		_ = os.Remove(path)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid uploaded file"})
		return
	}

	file := &model.File{
		OriginalName: filepath.Base(header.Filename), StoredName: storedName, Path: path,
		MimeType: "application/pdf", SizeBytes: written, UploadedBy: studentID, UploadedAt: time.Now(),
	}
	saved, previous, err := handler.service.AttachFinalReport(studentID, file)
	if err != nil {
		_ = os.Remove(path)
		handler.writeError(ctx, err)
		return
	}
	if previous != nil {
		_ = os.Remove(previous.Path)
		_ = handler.service.DeleteFileMetadata(previous.ID)
	}
	ctx.JSON(http.StatusCreated, fileMetadataResponse{ID: saved.ID, OriginalName: saved.OriginalName, UploadedAt: saved.UploadedAt})
}

func (handler *InternshipHandler) DownloadFile(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		return
	}
	roleValue, exists := ctx.Get(appmiddleware.ContextRole)
	role, valid := roleValue.(model.Role)
	if !exists || !valid {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	fileID, err := parseID(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return service.WeeklyReportInput{}, false
	}
	startDate, err := parseDate(request.StartDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid start date"})
		return service.WeeklyReportInput{}, false
	}
	endDate, err := parseDate(request.EndDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid end date"})
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
