package handler

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

const maxImportFileSize = 10 << 20

const (
	importHeaderFullName      = "نام و نام خانوادگی"
	importHeaderEmail         = "ایمیل"
	importHeaderStudentNumber = "شماره دانشجویی"
	importHeaderMajor         = "رشته"
)

type UniversityManagementHandler struct {
	service *service.UniversityManagementService
}

type managedUserRequest struct {
	FullName      string `json:"fullName"`
	Email         string `json:"email"`
	StudentNumber string `json:"studentNumber"`
	Major         string `json:"major"`
	CompanyID     uint   `json:"companyId"`
	Phone         string `json:"phone"`
	JobTitle      string `json:"jobTitle"`
}

type managedUserCreatedResponse struct {
	User              model.PublicUser `json:"user"`
	TemporaryPassword string           `json:"temporaryPassword"`
}

type managedCompanyRequest struct {
	Name         string `json:"name"`
	NationalID   string `json:"nationalId"`
	EconomicCode string `json:"economicCode"`
	Website      string `json:"website"`
	Phone        string `json:"phone"`
	Email        string `json:"email"`
	Address      string `json:"address"`
}

type managedCompanySupervisorResponse struct {
	model.PublicUser
	Company *model.Company `json:"company,omitempty"`
}

type professorAssignmentRequest struct {
	StudentID   uint `json:"studentId"`
	ProfessorID uint `json:"professorId"`
}

type professorAssignmentResponse struct {
	ID        uint              `json:"id"`
	Student   model.PublicUser  `json:"student"`
	Professor *model.PublicUser `json:"professor,omitempty"`
}

type importRowResult struct {
	Row               int    `json:"row"`
	Name              string `json:"name,omitempty"`
	Email             string `json:"email,omitempty"`
	Status            string `json:"status"`
	TemporaryPassword string `json:"temporaryPassword,omitempty"`
	Message           string `json:"message,omitempty"`
}

type importResponse struct {
	Created int               `json:"created"`
	Skipped int               `json:"skipped"`
	Results []importRowResult `json:"results"`
}

func NewUniversityManagementHandler(managementService *service.UniversityManagementService) *UniversityManagementHandler {
	return &UniversityManagementHandler{service: managementService}
}

func (handler *UniversityManagementHandler) ListStudents(ctx *gin.Context) {
	handler.listUsers(ctx, model.RoleStudent)
}

func (handler *UniversityManagementHandler) CreateStudent(ctx *gin.Context) {
	handler.createUser(ctx, model.RoleStudent)
}

func (handler *UniversityManagementHandler) ListProfessors(ctx *gin.Context) {
	handler.listUsers(ctx, model.RoleProfessor)
}

func (handler *UniversityManagementHandler) CreateProfessor(ctx *gin.Context) {
	handler.createUser(ctx, model.RoleProfessor)
}

func (handler *UniversityManagementHandler) ListCompanySupervisors(ctx *gin.Context) {
	users, err := handler.service.ListUsers(model.RoleCompanySupervisor)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	response := make([]managedCompanySupervisorResponse, 0, len(users))
	for _, user := range users {
		response = append(response, managedCompanySupervisorResponse{PublicUser: user.Public(), Company: user.Company})
	}
	ctx.JSON(http.StatusOK, response)
}

func (handler *UniversityManagementHandler) CreateCompanySupervisor(ctx *gin.Context) {
	handler.createUser(ctx, model.RoleCompanySupervisor)
}

func (handler *UniversityManagementHandler) listUsers(ctx *gin.Context, role model.Role) {
	users, err := handler.service.ListUsers(role)
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	response := make([]model.PublicUser, 0, len(users))
	for _, user := range users {
		response = append(response, user.Public())
	}
	ctx.JSON(http.StatusOK, response)
}

func (handler *UniversityManagementHandler) createUser(ctx *gin.Context, role model.Role) {
	var request managedUserRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "اطلاعات درخواست معتبر نیست."})
		return
	}
	user, password, err := handler.service.CreateManagedUser(role, service.ManagedUserInput{
		FullName: request.FullName, Email: request.Email, StudentNumber: request.StudentNumber,
		Major: request.Major, CompanyID: request.CompanyID, Phone: request.Phone, JobTitle: request.JobTitle,
	})
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, managedUserCreatedResponse{User: user.Public(), TemporaryPassword: password})
}

func (handler *UniversityManagementHandler) ImportStudents(ctx *gin.Context) {
	handler.importUsers(ctx, model.RoleStudent, []string{
		importHeaderFullName,
		importHeaderEmail,
		importHeaderStudentNumber,
		importHeaderMajor,
	})
}

func (handler *UniversityManagementHandler) ImportProfessors(ctx *gin.Context) {
	handler.importUsers(ctx, model.RoleProfessor, []string{importHeaderFullName, importHeaderEmail})
}

func (handler *UniversityManagementHandler) importUsers(ctx *gin.Context, role model.Role, expectedHeaders []string) {
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "انتخاب فایل اکسل الزامی است."})
		return
	}
	if !strings.EqualFold(filepath.Ext(fileHeader.Filename), ".xlsx") || fileHeader.Size > maxImportFileSize {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "فایل باید با فرمت اکسل و حداکثر حجم ۱۰ مگابایت باشد."})
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "خواندن فایل بارگذاری‌شده امکان‌پذیر نیست."})
		return
	}
	defer file.Close()

	workbook, err := excelize.OpenReader(file)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "ساختار فایل اکسل معتبر نیست."})
		return
	}
	defer workbook.Close()
	sheets := workbook.GetSheetList()
	if len(sheets) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "فایل اکسل فاقد صفحه کاری است."})
		return
	}
	rows, err := workbook.GetRows(sheets[0])
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "خواندن ردیف‌های فایل اکسل امکان‌پذیر نیست."})
		return
	}
	if len(rows) == 0 || blankRow(rows[0]) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "ساختار فایل اکسل معتبر نیست."})
		return
	}
	headerIndexes, missingHeader, valid := resolveHeaderIndexes(rows[0], expectedHeaders)
	if !valid {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "ساختار فایل اکسل معتبر نیست."})
		return
	}
	if missingHeader != "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("ستون «%s» در فایل اکسل یافت نشد.", missingHeader)})
		return
	}

	result := importResponse{Results: make([]importRowResult, 0)}
	for index, row := range rows[1:] {
		if blankRow(row) {
			continue
		}
		input := service.ManagedUserInput{
			FullName: cell(row, headerIndexes[importHeaderFullName]),
			Email:    cell(row, headerIndexes[importHeaderEmail]),
		}
		if role == model.RoleStudent {
			input.StudentNumber = cell(row, headerIndexes[importHeaderStudentNumber])
			input.Major = cell(row, headerIndexes[importHeaderMajor])
		}
		rowResult := importRowResult{Row: index + 2, Name: strings.TrimSpace(input.FullName), Email: strings.ToLower(strings.TrimSpace(input.Email))}
		if validationMessage := importRowValidationMessage(role, input); validationMessage != "" {
			rowResult.Status = "SKIPPED"
			rowResult.Message = validationMessage
			result.Skipped++
			result.Results = append(result.Results, rowResult)
			continue
		}
		user, password, createErr := handler.service.CreateManagedUser(role, input)
		if createErr != nil {
			rowResult.Status = "SKIPPED"
			rowResult.Message = publicImportError(createErr)
			result.Skipped++
		} else {
			rowResult.Status = "CREATED"
			rowResult.Name = user.FullName
			rowResult.Email = user.Email
			rowResult.TemporaryPassword = password
			result.Created++
		}
		result.Results = append(result.Results, rowResult)
	}
	ctx.JSON(http.StatusOK, result)
}

func (handler *UniversityManagementHandler) ListCompanies(ctx *gin.Context) {
	companies, err := handler.service.ListCompanies()
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, companies)
}

func (handler *UniversityManagementHandler) CreateCompany(ctx *gin.Context) {
	var request managedCompanyRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "اطلاعات درخواست معتبر نیست."})
		return
	}
	company, err := handler.service.CreateCompany(service.CompanyInput{
		Name: request.Name, NationalID: request.NationalID, EconomicCode: request.EconomicCode, Website: request.Website, Phone: request.Phone, Email: request.Email, Address: request.Address,
	})
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, company)
}

func (handler *UniversityManagementHandler) ListProfessorAssignments(ctx *gin.Context) {
	assignments, err := handler.service.ListProfessorAssignments()
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	response := make([]professorAssignmentResponse, 0, len(assignments))
	for _, assignment := range assignments {
		item := professorAssignmentResponse{ID: assignment.ID, Student: assignment.Student.Public()}
		if assignment.Professor != nil {
			professor := assignment.Professor.Public()
			item.Professor = &professor
		}
		response = append(response, item)
	}
	ctx.JSON(http.StatusOK, response)
}

func (handler *UniversityManagementHandler) CreateProfessorAssignment(ctx *gin.Context) {
	handler.saveProfessorAssignment(ctx, 0)
}

func (handler *UniversityManagementHandler) UpdateProfessorAssignment(ctx *gin.Context) {
	id, err := parseID(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "شناسه تخصیص استاد معتبر نیست."})
		return
	}
	handler.saveProfessorAssignment(ctx, id)
}

func (handler *UniversityManagementHandler) saveProfessorAssignment(ctx *gin.Context, assignmentID uint) {
	var request professorAssignmentRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "اطلاعات درخواست معتبر نیست."})
		return
	}
	var err error
	if assignmentID == 0 {
		_, err = handler.service.AssignProfessor(request.StudentID, request.ProfessorID)
	} else {
		_, err = handler.service.UpdateProfessorAssignment(assignmentID, request.StudentID, request.ProfessorID)
	}
	if err != nil {
		handler.writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"studentId": request.StudentID, "professorId": request.ProfessorID})
}

func (handler *UniversityManagementHandler) writeError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidManagementInput):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": publicManagementError(err)})
	case errors.Is(err, service.ErrEmailAlreadyExists), errors.Is(err, service.ErrStudentNumberExists), errors.Is(err, service.ErrCompanyAlreadyExists):
		ctx.JSON(http.StatusConflict, gin.H{"error": publicManagementError(err)})
	case errors.Is(err, service.ErrManagedUserNotFound), errors.Is(err, service.ErrCompanyNotFound), errors.Is(err, service.ErrAssignmentNotFoundMgmt):
		ctx.JSON(http.StatusNotFound, gin.H{"error": publicManagementError(err)})
	default:
		log.Printf("university management request failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "خطایی در سرور رخ داد."})
	}
}

func resolveHeaderIndexes(row, expected []string) (map[string]int, string, bool) {
	expectedSet := make(map[string]struct{}, len(expected))
	for _, header := range expected {
		expectedSet[normalizeExcelHeader(header)] = struct{}{}
	}

	indexes := make(map[string]int, len(expected))
	for index, value := range row {
		header := normalizeExcelHeader(value)
		if header == "" {
			continue
		}
		if _, required := expectedSet[header]; !required {
			continue
		}
		if _, duplicate := indexes[header]; duplicate {
			return nil, "", false
		}
		indexes[header] = index
	}
	for _, header := range expected {
		normalized := normalizeExcelHeader(header)
		if _, found := indexes[normalized]; !found {
			return nil, header, true
		}
	}
	return indexes, "", true
}

func normalizeExcelHeader(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSpace(strings.TrimPrefix(value, "\ufeff"))
	return strings.NewReplacer("ي", "ی", "ى", "ی", "ك", "ک").Replace(value)
}

func blankRow(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func cell(row []string, index int) string {
	if index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func importRowValidationMessage(role model.Role, input service.ManagedUserInput) string {
	if strings.TrimSpace(input.FullName) == "" {
		return "نام و نام خانوادگی وارد نشده است."
	}
	if strings.TrimSpace(input.Email) == "" {
		return "ایمیل وارد نشده است."
	}
	if role == model.RoleStudent {
		if strings.TrimSpace(input.StudentNumber) == "" {
			return "شماره دانشجویی وارد نشده است."
		}
		if strings.TrimSpace(input.Major) == "" {
			return "رشته تحصیلی وارد نشده است."
		}
	}
	return ""
}

func publicImportError(err error) string {
	switch {
	case errors.Is(err, service.ErrEmailAlreadyExists):
		return "کاربری با این ایمیل قبلاً ثبت شده است."
	case errors.Is(err, service.ErrStudentNumberExists):
		return "دانشجویی با این شماره دانشجویی قبلاً ثبت شده است."
	default:
		return "اطلاعات این ردیف معتبر نیست."
	}
}

func publicManagementError(err error) string {
	switch {
	case errors.Is(err, service.ErrEmailAlreadyExists):
		return "کاربری با این ایمیل قبلاً ثبت شده است."
	case errors.Is(err, service.ErrStudentNumberExists):
		return "دانشجویی با این شماره دانشجویی قبلاً ثبت شده است."
	case errors.Is(err, service.ErrCompanyAlreadyExists):
		return "شرکتی با این نام قبلاً ثبت شده است."
	case errors.Is(err, service.ErrCompanyNotFound):
		return "شرکت تأییدشده یافت نشد."
	case errors.Is(err, service.ErrManagedUserNotFound):
		return "دانشجو یا استاد یافت نشد."
	case errors.Is(err, service.ErrAssignmentNotFoundMgmt):
		return "تخصیص استاد یافت نشد."
	case errors.Is(err, service.ErrInvalidManagementInput):
		return "اطلاعات واردشده کامل یا معتبر نیست."
	default:
		return "اطلاعات این ردیف معتبر نیست."
	}
}
