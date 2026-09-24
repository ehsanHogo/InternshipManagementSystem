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
	service   *service.InternshipService
	uploadDir string
}

type updateCaseRequest struct {
	PassedCredits *int    `json:"passedCredits"`
	Mobile        *string `json:"mobile"`
}

type preferenceRequest struct {
	Priority                 int  `json:"priority"`
	OpportunityApplicationID uint `json:"opportunityApplicationId"`
}

type internshipCaseResponse struct {
	ID                            uint                           `json:"id"`
	Status                        model.InternshipCaseStatus     `json:"status"`
	PassedCredits                 *int                           `json:"passedCredits"`
	Mobile                        *string                        `json:"mobile"`
	Student                       model.PublicUser               `json:"student"`
	Professor                     model.PublicUser               `json:"professor"`
	Preferences                   []internshipPreferenceResponse `json:"preferences"`
	SelectedPreferenceID          *uint                          `json:"selectedPreferenceId"`
	SelectedPreference            *internshipPreferenceResponse  `json:"selectedPreference,omitempty"`
	CompanySupervisorID           *uint                          `json:"companySupervisorId"`
	CompanySupervisor             *model.PublicUser              `json:"companySupervisor,omitempty"`
	LetterNumber                  *string                        `json:"letterNumber"`
	LetterDate                    *time.Time                     `json:"letterDate"`
	InternshipSubject             *string                        `json:"internshipSubject"`
	StartDate                     *time.Time                     `json:"startDate"`
	WorkplaceAddress              *string                        `json:"workplaceAddress"`
	WorkplacePhone                *string                        `json:"workplacePhone"`
	CreatedAt                     time.Time                      `json:"createdAt"`
	UpdatedAt                     time.Time                      `json:"updatedAt"`
	SubmittedAt                   *time.Time                     `json:"submittedAt"`
	CancellationComment           *string                        `json:"cancellationComment,omitempty"`
	CancelledAt                   *time.Time                     `json:"cancelledAt,omitempty"`
	CompanyDetailsRevisionComment *string                        `json:"companyDetailsRevisionComment,omitempty"`
	ActivatedAt                   *time.Time                     `json:"activatedAt"`
	FinalReport                   *fileMetadataResponse          `json:"finalReport,omitempty"`
	WeeklyReportCount             int                            `json:"weeklyReportCount"`
	ConfirmedReportCount          int                            `json:"confirmedReportCount"`
	CanSubmitCompanyEvaluation    bool                           `json:"canSubmitCompanyEvaluation"`
	WeeklyReports                 []model.WeeklyReport           `json:"weeklyReports"`
	CompanyEvaluation             *model.CompanyEvaluation       `json:"companyEvaluation,omitempty"`
	FinalResult                   *model.ProfessorFinalResult    `json:"finalResult,omitempty"`
	ProfessorComment              *string                        `json:"professorComment,omitempty"`
	CompletedAt                   *time.Time                     `json:"completedAt,omitempty"`
}

type fileMetadataResponse struct {
	ID           uint      `json:"id"`
	OriginalName string    `json:"originalName"`
	UploadedAt   time.Time `json:"uploadedAt"`
}

type internshipPreferenceResponse struct {
	ID                       uint `json:"id"`
	Priority                 int  `json:"priority"`
	OpportunityApplicationID uint `json:"opportunityApplicationId"`
}

func NewInternshipHandler(internshipService *service.InternshipService, uploadDirs ...string) *InternshipHandler {
	uploadDir := "uploads"
	if len(uploadDirs) > 0 && uploadDirs[0] != "" {
		uploadDir = uploadDirs[0]
	}
	return &InternshipHandler{service: internshipService, uploadDir: uploadDir}
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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "اطلاعات درخواست معتبر نیست."})
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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "شناسه اولویت معتبر نیست."})
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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "شناسه اولویت معتبر نیست."})
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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "اطلاعات درخواست معتبر نیست."})
		return 0, preferenceRequest{}, false
	}
	return studentID, request, true
}

func (request preferenceRequest) preferenceInput() service.PreferenceInput {
	return service.PreferenceInput{
		Priority: request.Priority, OpportunityApplicationID: request.OpportunityApplicationID,
	}
}

func (handler *InternshipHandler) writeError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrCaseNotFound), errors.Is(err, service.ErrPreferenceNotFound),
		errors.Is(err, service.ErrWeeklyReportNotFound), errors.Is(err, service.ErrEvaluationNotFound),
		errors.Is(err, service.ErrFileNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": publicInternshipError(err)})
	case errors.Is(err, service.ErrAssignmentNotFound), errors.Is(err, service.ErrInvalidApplication),
		errors.Is(err, service.ErrInvalidPreference), errors.Is(err, service.ErrInvalidCaseStatus),
		errors.Is(err, service.ErrCompanySupervisor), errors.Is(err, service.ErrInvalidWeeklyReport),
		errors.Is(err, service.ErrInvalidEvaluation), errors.Is(err, service.ErrInvalidProfessorResult):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": publicInternshipError(err)})
	case errors.Is(err, service.ErrCaseNotEditable), errors.Is(err, service.ErrPreferenceLimit), errors.Is(err, service.ErrDuplicatePriority),
		errors.Is(err, service.ErrDuplicateWeeklyReport), errors.Is(err, service.ErrWeeklyReportConfirmed),
		errors.Is(err, service.ErrDuplicateEvaluation), errors.Is(err, service.ErrWeeklyReportsIncomplete),
		errors.Is(err, service.ErrProfessorCaseNotActive), errors.Is(err, service.ErrProfessorWeeklyReportsIncomplete),
		errors.Is(err, service.ErrProfessorCompanyEvaluationRequired), errors.Is(err, service.ErrProfessorFinalReportRequired):
		ctx.JSON(http.StatusConflict, gin.H{"error": publicInternshipError(err)})
	case errors.Is(err, service.ErrInvalidTransition), errors.Is(err, service.ErrInternshipCompleted),
		errors.Is(err, service.ErrObsoleteWorkflow):
		ctx.JSON(http.StatusConflict, gin.H{"error": publicInternshipError(err)})
	case errors.Is(err, service.ErrCaseAccessDenied):
		ctx.JSON(http.StatusForbidden, gin.H{"error": publicInternshipError(err)})
	default:
		log.Printf("internship request failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "خطایی در سرور رخ داد."})
	}
}

func currentUserID(ctx *gin.Context) (uint, bool) {
	value, exists := ctx.Get(appmiddleware.ContextUserID)
	userID, ok := value.(uint)
	if !exists || !ok || userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "ورود به سامانه الزامی است."})
		return 0, false
	}
	return userID, true
}

func publicInternshipError(err error) string {
	switch {
	case errors.Is(err, service.ErrCaseNotFound):
		return "پرونده کارآموزی یافت نشد."
	case errors.Is(err, service.ErrAssignmentNotFound):
		return "استادی برای دانشجو تخصیص داده نشده است."
	case errors.Is(err, service.ErrCaseNotEditable):
		return "این پرونده در وضعیت قابل ویرایش نیست."
	case errors.Is(err, service.ErrPreferenceNotFound):
		return "اولویت کارآموزی یافت نشد."
	case errors.Is(err, service.ErrPreferenceLimit):
		return "حداکثر تعداد اولویت‌های مجاز ثبت شده است."
	case errors.Is(err, service.ErrDuplicatePriority):
		return "این شماره اولویت قبلاً ثبت شده است."
	case errors.Is(err, service.ErrInvalidPreference):
		return "اطلاعات اولویت کارآموزی معتبر نیست."
	case errors.Is(err, service.ErrInvalidApplication):
		return "اطلاعات درخواست کارآموزی کامل نیست."
	case errors.Is(err, service.ErrInvalidCaseStatus):
		return "وضعیت پرونده کارآموزی معتبر نیست."
	case errors.Is(err, service.ErrInvalidTransition):
		return "تغییر وضعیت در مرحله فعلی امکان‌پذیر نیست."
	case errors.Is(err, service.ErrInternshipCompleted):
		return "دانشجو دوره کارآموزی را تکمیل کرده است."
	case errors.Is(err, service.ErrObsoleteWorkflow):
		return "این گردش‌کار در نسخه جدید هنوز فعال نشده است."
	case errors.Is(err, service.ErrCompanySupervisor):
		return "سرپرست شرکت معتبر نیست."
	case errors.Is(err, service.ErrCaseAccessDenied):
		return "اجازه دسترسی به این پرونده را ندارید."
	case errors.Is(err, service.ErrWeeklyReportNotFound):
		return "گزارش هفتگی یافت نشد."
	case errors.Is(err, service.ErrInvalidWeeklyReport):
		return "اطلاعات گزارش هفتگی معتبر نیست."
	case errors.Is(err, service.ErrDuplicateWeeklyReport):
		return "گزارش این هفته قبلاً ثبت شده است."
	case errors.Is(err, service.ErrWeeklyReportConfirmed):
		return "گزارش هفتگی تأییدشده قابل تغییر نیست."
	case errors.Is(err, service.ErrEvaluationNotFound):
		return "ارزیابی شرکت یافت نشد."
	case errors.Is(err, service.ErrInvalidEvaluation):
		return "اطلاعات ارزیابی شرکت معتبر نیست."
	case errors.Is(err, service.ErrDuplicateEvaluation):
		return "ارزیابی شرکت قبلاً ثبت شده است."
	case errors.Is(err, service.ErrWeeklyReportsIncomplete), errors.Is(err, service.ErrProfessorWeeklyReportsIncomplete):
		return "هر ۸ گزارش هفتگی باید ثبت و تأیید شده باشند."
	case errors.Is(err, service.ErrFileNotFound):
		return "فایل یافت نشد."
	case errors.Is(err, service.ErrInvalidProfessorResult):
		return "نتیجه نهایی استاد معتبر نیست."
	case errors.Is(err, service.ErrProfessorCaseNotActive):
		return "پرونده کارآموزی فعال نیست."
	case errors.Is(err, service.ErrProfessorCompanyEvaluationRequired):
		return "ثبت ارزیابی شرکت الزامی است."
	case errors.Is(err, service.ErrProfessorFinalReportRequired):
		return "بارگذاری گزارش نهایی کارآموزی الزامی است."
	default:
		return "انجام عملیات امکان‌پذیر نیست."
	}
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
		CancellationComment: internshipCase.CancellationComment, CancelledAt: internshipCase.CancelledAt,
		CompanyDetailsRevisionComment: internshipCase.CompanyDetailsRevisionComment,
		ActivatedAt:                   internshipCase.ActivatedAt,
		FinalResult:                   internshipCase.FinalResult, ProfessorComment: internshipCase.ProfessorComment,
		CompletedAt: internshipCase.CompletedAt,
	}
	if internshipCase.SelectedPreference != nil {
		selected := preferenceResponse(*internshipCase.SelectedPreference)
		response.SelectedPreference = &selected
	}
	if internshipCase.CompanySupervisor != nil {
		supervisor := internshipCase.CompanySupervisor.Public()
		response.CompanySupervisor = &supervisor
	}
	response.WeeklyReportCount = len(internshipCase.WeeklyReports)
	response.WeeklyReports = internshipCase.WeeklyReports
	if response.WeeklyReports == nil {
		response.WeeklyReports = []model.WeeklyReport{}
	}
	response.CompanyEvaluation = internshipCase.CompanyEvaluation
	for _, report := range internshipCase.WeeklyReports {
		if report.IsConfirmed {
			response.ConfirmedReportCount++
		}
	}
	if internshipCase.FinalReportFile != nil {
		response.FinalReport = &fileMetadataResponse{
			ID: internshipCase.FinalReportFile.ID, OriginalName: internshipCase.FinalReportFile.OriginalName,
			UploadedAt: internshipCase.FinalReportFile.UploadedAt,
		}
	}
	response.CanSubmitCompanyEvaluation = internshipCase.Status == model.InternshipCaseStatusActive &&
		response.WeeklyReportCount == 8 && response.ConfirmedReportCount == 8 && internshipCase.CompanyEvaluation == nil
	return response
}

func preferenceResponse(preference model.InternshipPreference) internshipPreferenceResponse {
	return internshipPreferenceResponse{
		ID: preference.ID, Priority: preference.Priority,
		OpportunityApplicationID: preference.OpportunityApplicationID,
	}
}
