package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"internship-management-system/backend/internal/auth"
	appmiddleware "internship-management-system/backend/internal/middleware"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

func TestOpportunityApplicationHTTPFlowAndResumeIsolation(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect to integration database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Company{}, &model.User{}, &model.InternshipOpportunity{}, &model.File{},
		&model.OpportunityApplication{}, &model.InternshipCase{},
	); err != nil {
		t.Fatalf("migrate integration database: %v", err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	defer tx.Rollback()

	uploadDir := t.TempDir()
	const jwtSecret = "application-handler-integration-secret"
	router := applicationTestRouter(tx, jwtSecret, uploadDir)
	suffix := time.Now().UnixNano()
	companyA := createOpportunityTestCompany(t, tx, suffix, "application-a", false)
	companyB := createOpportunityTestCompany(t, tx, suffix, "application-b", true)
	supervisorA := createOpportunityTestUser(t, tx, suffix, "application-supervisor-a", model.RoleCompanySupervisor, &companyA.ID)
	supervisorB := createOpportunityTestUser(t, tx, suffix, "application-supervisor-b", model.RoleCompanySupervisor, &companyB.ID)
	student := createOpportunityTestUser(t, tx, suffix, "application-student", model.RoleStudent, nil)
	otherStudent := createOpportunityTestUser(t, tx, suffix, "application-other-student", model.RoleStudent, nil)
	professor := createOpportunityTestUser(t, tx, suffix, "application-professor", model.RoleProfessor, nil)
	university := createOpportunityTestUser(t, tx, suffix, "application-university", model.RoleUniversitySupervisor, nil)
	tokens := map[string]string{}
	for label, user := range map[string]model.User{"supervisorA": supervisorA, "supervisorB": supervisorB, "student": student, "otherStudent": otherStudent, "professor": professor, "university": university} {
		token, tokenErr := auth.CreateToken(user, jwtSecret, time.Hour)
		if tokenErr != nil {
			t.Fatalf("create %s token: %v", label, tokenErr)
		}
		tokens[label] = token
	}
	opportunity := model.InternshipOpportunity{CompanyID: companyA.ID, CreatedBy: supervisorA.ID, Title: "فرصت آزمون درخواست", Description: "شرح", WorkField: "نرم‌افزار", Location: "تهران", Status: model.OpportunityStatusOpen}
	if err := tx.Create(&opportunity).Error; err != nil {
		t.Fatalf("create opportunity: %v", err)
	}

	createdResponse := performApplicationUpload(t, router, fmt.Sprintf("/api/student/opportunities/%d/apply", opportunity.ID), "resume.pdf", "application/pdf", []byte("%PDF-1.4\nresume"), tokens["student"])
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("apply status = %d, body = %s", createdResponse.Code, createdResponse.Body.String())
	}
	var created opportunityApplicationResponse
	if err := json.Unmarshal(createdResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode application: %v", err)
	}
	if created.Status != model.ApplicationStatusPending || created.Resume.ID == 0 || created.Student != nil {
		t.Fatalf("unexpected student application response: %+v", created)
	}

	duplicate := performApplicationUpload(t, router, fmt.Sprintf("/api/student/opportunities/%d/apply", opportunity.ID), "again.pdf", "application/pdf", []byte("%PDF-1.4\nagain"), tokens["student"])
	assertAPIError(t, duplicate, http.StatusConflict, "APPLICATION_ALREADY_EXISTS")
	entries, err := os.ReadDir(uploadDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("resume files after duplicate = %d, err = %v", len(entries), err)
	}

	studentList := performJSONRequest(t, router, http.MethodGet, "/api/student/opportunity-applications", nil, tokens["student"])
	if studentList.Code != http.StatusOK {
		t.Fatalf("student list status = %d, body = %s", studentList.Code, studentList.Body.String())
	}
	studentDetail := performJSONRequest(t, router, http.MethodGet, fmt.Sprintf("/api/student/opportunity-applications/%d", created.ID), nil, tokens["student"])
	if studentDetail.Code != http.StatusOK {
		t.Fatalf("student detail status = %d, body = %s", studentDetail.Code, studentDetail.Body.String())
	}
	assertAPIError(t, performJSONRequest(t, router, http.MethodGet, fmt.Sprintf("/api/student/opportunity-applications/%d", created.ID), nil, tokens["otherStudent"]), http.StatusNotFound, "OPPORTUNITY_APPLICATION_NOT_FOUND")

	companyList := performJSONRequest(t, router, http.MethodGet, fmt.Sprintf("/api/company/opportunities/%d/applications", opportunity.ID), nil, tokens["supervisorA"])
	if companyList.Code != http.StatusOK {
		t.Fatalf("company list status = %d, body = %s", companyList.Code, companyList.Body.String())
	}
	var companyApplications []opportunityApplicationResponse
	if err := json.Unmarshal(companyList.Body.Bytes(), &companyApplications); err != nil || len(companyApplications) != 1 || companyApplications[0].Student == nil {
		t.Fatalf("company applicant list = %+v, err = %v", companyApplications, err)
	}
	if companyApplications[0].Student.FullName != student.FullName || companyApplications[0].Student.Email != student.Email {
		t.Fatalf("minimal applicant identity mismatch: %+v", companyApplications[0].Student)
	}
	assertAPIError(t, performJSONRequest(t, router, http.MethodGet, fmt.Sprintf("/api/company/opportunity-applications/%d", created.ID), nil, tokens["supervisorB"]), http.StatusNotFound, "OPPORTUNITY_APPLICATION_NOT_FOUND")
	assertAPIError(t, performJSONRequest(t, router, http.MethodPost, fmt.Sprintf("/api/company/opportunity-applications/%d/accept", created.ID), map[string]any{}, tokens["supervisorB"]), http.StatusNotFound, "OPPORTUNITY_APPLICATION_NOT_FOUND")

	filePath := fmt.Sprintf("/api/files/%d/download", created.Resume.ID)
	for label, token := range map[string]string{"student": tokens["student"], "supervisor": tokens["supervisorA"]} {
		response := performJSONRequest(t, router, http.MethodGet, filePath, nil, token)
		if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/pdf" {
			t.Fatalf("%s resume download status = %d", label, response.Code)
		}
	}
	for label, token := range map[string]string{"otherStudent": tokens["otherStudent"], "otherSupervisor": tokens["supervisorB"], "professor": tokens["professor"], "university": tokens["university"]} {
		response := performJSONRequest(t, router, http.MethodGet, filePath, nil, token)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s resume download status = %d, body = %s", label, response.Code, response.Body.String())
		}
	}
	if response := performJSONRequest(t, router, http.MethodGet, filePath, nil, ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated resume download status = %d", response.Code)
	}

	comment := "پذیرش برای تیم فنی"
	acceptedResponse := performJSONRequest(t, router, http.MethodPost, fmt.Sprintf("/api/company/opportunity-applications/%d/accept", created.ID), map[string]any{"companyComment": comment}, tokens["supervisorA"])
	if acceptedResponse.Code != http.StatusOK {
		t.Fatalf("accept status = %d, body = %s", acceptedResponse.Code, acceptedResponse.Body.String())
	}
	var accepted opportunityApplicationResponse
	if err := json.Unmarshal(acceptedResponse.Body.Bytes(), &accepted); err != nil || accepted.Status != model.ApplicationStatusAccepted || accepted.ReviewedAt == nil || accepted.CompanyComment == nil || *accepted.CompanyComment != comment {
		t.Fatalf("accepted response = %+v, err = %v", accepted, err)
	}
	assertAPIError(t, performJSONRequest(t, router, http.MethodPost, fmt.Sprintf("/api/company/opportunity-applications/%d/reject", created.ID), map[string]any{}, tokens["supervisorA"]), http.StatusConflict, "APPLICATION_NOT_PENDING")

	pendingOpportunity := model.InternshipOpportunity{CompanyID: companyA.ID, CreatedBy: supervisorA.ID, Title: "فرصت بسته‌شونده", Description: "شرح", WorkField: "داده", Location: "قم", Status: model.OpportunityStatusOpen}
	if err := tx.Create(&pendingOpportunity).Error; err != nil {
		t.Fatalf("create close-after opportunity: %v", err)
	}
	pendingResponse := performApplicationUpload(t, router, fmt.Sprintf("/api/student/opportunities/%d/apply", pendingOpportunity.ID), "second.pdf", "application/pdf", []byte("%PDF-1.4\nsecond"), tokens["otherStudent"])
	if pendingResponse.Code != http.StatusCreated {
		t.Fatalf("apply before close status = %d, body = %s", pendingResponse.Code, pendingResponse.Body.String())
	}
	var pending opportunityApplicationResponse
	if err := json.Unmarshal(pendingResponse.Body.Bytes(), &pending); err != nil {
		t.Fatalf("decode pending: %v", err)
	}
	if err := tx.Model(&pendingOpportunity).Update("status", model.OpportunityStatusClosed).Error; err != nil {
		t.Fatalf("close opportunity: %v", err)
	}
	rejectedResponse := performJSONRequest(t, router, http.MethodPost, fmt.Sprintf("/api/company/opportunity-applications/%d/reject", pending.ID), map[string]any{}, tokens["supervisorA"])
	if rejectedResponse.Code != http.StatusOK {
		t.Fatalf("review after close status = %d, body = %s", rejectedResponse.Code, rejectedResponse.Body.String())
	}
}

func applicationTestRouter(db *gorm.DB, jwtSecret, uploadDir string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	applicationService := service.NewOpportunityApplicationService(db)
	applications := NewOpportunityApplicationHandler(applicationService, uploadDir)
	internships := NewInternshipHandler(service.NewInternshipService(db), uploadDir)
	router := gin.New()
	authenticated := router.Group("/api")
	authenticated.Use(appmiddleware.RequireAuth(jwtSecret))
	student := authenticated.Group("/student")
	student.Use(appmiddleware.RequireRole(model.RoleStudent))
	student.POST("/opportunities/:id/apply", applications.Apply)
	student.GET("/opportunity-applications", applications.ListStudent)
	student.GET("/opportunity-applications/:id", applications.GetStudent)
	company := authenticated.Group("/company")
	company.Use(appmiddleware.RequireRole(model.RoleCompanySupervisor))
	company.GET("/opportunities/:id/applications", applications.ListCompany)
	company.GET("/opportunity-applications/:id", applications.GetCompany)
	company.POST("/opportunity-applications/:id/accept", applications.Accept)
	company.POST("/opportunity-applications/:id/reject", applications.Reject)
	authenticated.GET("/files/:id/download", internships.DownloadFile)
	return router
}

func performApplicationUpload(t *testing.T, router http.Handler, path, filename, contentType string, content []byte, token string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="resume"; filename="%s"`, filename))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
