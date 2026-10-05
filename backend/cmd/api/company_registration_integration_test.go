package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"internship-management-system/backend/internal/auth"
	"internship-management-system/backend/internal/config"
	"internship-management-system/backend/internal/database"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

const registrationTestSecret = "m17-registration-tests"

func registrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := database.MigrateAndSeed(db); err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	return tx
}
func registrationRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return newRouter(config.Config{JWT: config.JWTConfig{Secret: registrationTestSecret, ExpiresHours: 1}, FrontendOrigin: "http://localhost:4200", UploadDir: t.TempDir()}, db)
}
func registrationRequest(t *testing.T, router http.Handler, method, path, token string, body any, want int) *httptest.ResponseRecorder {
	t.Helper()
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(data))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != want {
		t.Fatalf("%s %s: status %d want %d: %s", method, path, recorder.Code, want, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "passwordHash") || strings.Contains(recorder.Body.String(), "PasswordHash") {
		t.Fatalf("password hash leaked: %s", recorder.Body.String())
	}
	return recorder
}
func registrationDecode[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(recorder.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func registrationToken(t *testing.T, user model.User) string {
	t.Helper()
	token, err := auth.CreateToken(user, registrationTestSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return token
}
func registrationDemoUser(t *testing.T, db *gorm.DB, role string) model.User {
	t.Helper()
	var user model.User
	if err := db.Where("email = ?", role+"@demo.local").First(&user).Error; err != nil {
		t.Fatal(err)
	}
	return user
}
func registrationFixture(t *testing.T, router http.Handler, label string) (service.CompanyRegistrationInput, model.Company, model.PublicUser, string) {
	t.Helper()
	key := fmt.Sprintf("%d-%s", time.Now().UnixNano(), label)
	input := service.CompanyRegistrationInput{
		Company:    service.CompanyInput{Name: "Company " + key, NationalID: "n-" + key, EconomicCode: "e-" + key, Website: "https://example.test", Phone: "02111111111", Email: "office-" + key + "@example.test", Address: "تهران"},
		Supervisor: service.CompanySupervisorRegistrationInput{FullName: "سرپرست", Email: "supervisor-" + key + "@example.test", Password: "Registration123!", Phone: "09121111111", JobTitle: "مدیر"},
	}
	payload := registrationProfilePayload(input)
	payload["supervisor"].(map[string]any)["password"] = input.Supervisor.Password
	response := registrationRequest(t, router, "POST", "/api/auth/company-register", "", payload, http.StatusCreated)
	account := registrationDecode[struct {
		Account struct {
			Company    model.Company
			Supervisor model.PublicUser
		}
	}](t, response).Account
	login := registrationRequest(t, router, "POST", "/api/auth/login", "", map[string]string{"email": input.Supervisor.Email, "password": input.Supervisor.Password}, http.StatusOK)
	session := registrationDecode[struct {
		Token string
		User  model.PublicUser
	}](t, login)
	if session.User.CompanyRegistrationStatus == nil || *session.User.CompanyRegistrationStatus != model.CompanyRegistrationStatusPending {
		t.Fatal("login must report current registration permission")
	}
	return input, account.Company, account.Supervisor, session.Token
}
func registrationProfilePayload(input service.CompanyRegistrationInput) map[string]any {
	c, u := input.Company, input.Supervisor
	return map[string]any{
		"company":    map[string]any{"name": c.Name, "nationalId": c.NationalID, "economicCode": c.EconomicCode, "website": c.Website, "phone": c.Phone, "email": c.Email, "address": c.Address},
		"supervisor": map[string]any{"fullName": u.FullName, "email": u.Email, "phone": u.Phone, "jobTitle": u.JobTitle},
	}
}
func assertRegistrationBusinessBlocked(t *testing.T, router *gin.Engine, token string) {
	t.Helper()
	checked := 0
	for _, route := range router.Routes() {
		if !strings.HasPrefix(route.Path, "/api/company/") || route.Path == "/api/company/profile" || route.Path == "/api/company/registration/resubmit" || strings.Contains(route.Path, "/history") {
			continue
		}
		path := strings.ReplaceAll(strings.ReplaceAll(route.Path, ":id", "999999"), ":reportId", "999999")
		response := registrationRequest(t, router, route.Method, path, token, map[string]any{}, http.StatusForbidden)
		if !strings.Contains(response.Body.String(), "COMPANY_REGISTRATION_NOT_APPROVED") {
			t.Fatalf("missing permission error: %s", response.Body.String())
		}
		checked++
	}
	if checked < 19 {
		t.Fatalf("only %d company business endpoints checked", checked)
	}
}

func TestCompanyRegistrationGovernanceProductionRoutes(t *testing.T) {
	db := registrationTestDB(t)
	router := registrationRouter(t, db)
	input, company, supervisor, token := registrationFixture(t, router, "lifecycle")
	admin := registrationDemoUser(t, db, "admin")
	adminToken := registrationToken(t, admin)
	reviewPath := fmt.Sprintf("/api/admin/company-registrations/%d", company.ID)
	if company.RegistrationStatus != model.CompanyRegistrationStatusPending || company.IsApproved || supervisor.Role != model.RoleCompanySupervisor || supervisor.CompanyID == nil || *supervisor.CompanyID != company.ID {
		t.Fatal("incorrect self-registration defaults or membership")
	}
	registrationRequest(t, router, "GET", "/api/company/profile", token, nil, http.StatusOK)
	assertRegistrationBusinessBlocked(t, router, token)
	registrationRequest(t, router, "GET", "/api/companies", token, nil, http.StatusForbidden)
	registrationRequest(t, router, "POST", "/api/company/registration/resubmit", token, nil, http.StatusConflict)

	// Every review route is Admin-only, including all other authenticated roles.
	for _, role := range []string{"student", "professor", "university", "company"} {
		nonAdmin := registrationToken(t, registrationDemoUser(t, db, role))
		for _, request := range []struct{ method, path string }{{"GET", "/api/admin/company-registrations"}, {"GET", reviewPath}, {"POST", reviewPath + "/approve"}, {"POST", reviewPath + "/reject"}} {
			registrationRequest(t, router, request.method, request.path, nonAdmin, map[string]string{"reason": "reason"}, http.StatusForbidden)
		}
	}
	registrationRequest(t, router, "GET", reviewPath, "", nil, http.StatusUnauthorized)
	list := registrationDecode[[]model.Company](t, registrationRequest(t, router, "GET", "/api/admin/company-registrations?status=PENDING", adminToken, nil, http.StatusOK))
	found := false
	for _, c := range list {
		if c.RegistrationStatus != model.CompanyRegistrationStatusPending {
			t.Fatal("status filter ignored")
		}
		found = found || c.ID == company.ID
	}
	if !found {
		t.Fatal("new registration missing from review queue")
	}
	registrationRequest(t, router, "GET", "/api/admin/company-registrations?status=INVALID", adminToken, nil, http.StatusBadRequest)

	input.Company.Name += " corrected"
	input.Supervisor.FullName = "نماینده جدید"
	payload := registrationProfilePayload(input)
	registrationRequest(t, router, "PUT", "/api/company/profile", token, payload, http.StatusOK)
	detail := registrationDecode[service.CompanyRegistrationDetail](t, registrationRequest(t, router, "GET", reviewPath, adminToken, nil, http.StatusOK))
	if detail.Company.Name != input.Company.Name || detail.Company.RegistrationStatus != model.CompanyRegistrationStatusPending || len(detail.Supervisors) != 1 || detail.Supervisors[0].FullName != input.Supervisor.FullName {
		t.Fatal("Admin did not see latest pending edits")
	}
	for _, field := range []string{"id", "isApproved", "registrationStatus", "registrationReviewedAt", "registrationReviewedBy", "registrationRejectionReason"} {
		attack := registrationProfilePayload(input)
		attack["company"].(map[string]any)[field] = "unauthorized"
		registrationRequest(t, router, "PUT", "/api/company/profile", token, attack, http.StatusBadRequest)
	}
	for _, field := range []string{"id", "role", "companyId", "password"} {
		attack := registrationProfilePayload(input)
		attack["supervisor"].(map[string]any)[field] = "unauthorized"
		registrationRequest(t, router, "PUT", "/api/company/profile", token, attack, http.StatusBadRequest)
	}
	invalid := registrationProfilePayload(input)
	invalid["company"].(map[string]any)["address"] = " "
	registrationRequest(t, router, "PUT", "/api/company/profile", token, invalid, http.StatusBadRequest)
	for _, reason := range []string{"", "   "} {
		registrationRequest(t, router, "POST", reviewPath+"/reject", adminToken, map[string]string{"reason": reason}, http.StatusBadRequest)
	}
	rejected := registrationDecode[model.Company](t, registrationRequest(t, router, "POST", reviewPath+"/reject", adminToken, map[string]string{"reason": "  اطلاعات ناقص  "}, http.StatusOK))
	if rejected.RegistrationStatus != model.CompanyRegistrationStatusRejected || rejected.RegistrationReviewedBy == nil || *rejected.RegistrationReviewedBy != admin.ID || rejected.RegistrationReviewedAt == nil || rejected.RegistrationRejectionReason == nil || *rejected.RegistrationRejectionReason != "اطلاعات ناقص" || rejected.IsApproved {
		t.Fatal("rejection audit or trust fields incorrect")
	}
	rejectedReviewAt := *rejected.RegistrationReviewedAt
	registrationRequest(t, router, "POST", "/api/auth/login", "", map[string]string{"email": input.Supervisor.Email, "password": input.Supervisor.Password}, http.StatusOK)
	assertRegistrationBusinessBlocked(t, router, token)
	registrationRequest(t, router, "POST", reviewPath+"/approve", adminToken, nil, http.StatusConflict)
	registrationRequest(t, router, "POST", reviewPath+"/reject", adminToken, map[string]string{"reason": "again"}, http.StatusConflict)
	input.Company.Address = "نشانی اصلاح‌شده"
	edited := registrationDecode[struct {
		Company    model.Company
		Supervisor model.PublicUser
	}](t, registrationRequest(t, router, "PUT", "/api/company/profile", token, registrationProfilePayload(input), http.StatusOK))
	if edited.Company.RegistrationStatus != model.CompanyRegistrationStatusRejected || edited.Company.RegistrationRejectionReason == nil {
		t.Fatal("edit auto-resubmitted rejected registration")
	}
	// Check ownership: another company can only resubmit its own record.
	_, otherCompany, _, otherToken := registrationFixture(t, router, "other")
	registrationRequest(t, router, "POST", "/api/company/registration/resubmit", otherToken, nil, http.StatusConflict)
	resubmitted := registrationDecode[struct {
		Company    model.Company
		Supervisor model.PublicUser
	}](t, registrationRequest(t, router, "POST", "/api/company/registration/resubmit", token, nil, http.StatusOK))
	if resubmitted.Company.ID != company.ID || resubmitted.Supervisor.ID != supervisor.ID || resubmitted.Company.IsApproved || resubmitted.Company.RegistrationStatus != model.CompanyRegistrationStatusPending || resubmitted.Company.RegistrationRejectionReason != nil || resubmitted.Company.RegistrationReviewedAt == nil || !resubmitted.Company.RegistrationReviewedAt.Equal(rejectedReviewAt) {
		t.Fatal("resubmission did not preserve identity/trust/review timestamp")
	}
	var supervisorCount int64
	if err := db.Model(&model.User{}).Where("company_id = ?", company.ID).Count(&supervisorCount).Error; err != nil || supervisorCount != 1 {
		t.Fatal("resubmission created another account")
	}
	assertRegistrationBusinessBlocked(t, router, token)
	approved := registrationDecode[model.Company](t, registrationRequest(t, router, "POST", reviewPath+"/approve", adminToken, nil, http.StatusOK))
	if approved.RegistrationStatus != model.CompanyRegistrationStatusApproved || approved.IsApproved || approved.RegistrationReviewedBy == nil || *approved.RegistrationReviewedBy != admin.ID || approved.RegistrationReviewedAt == nil || approved.RegistrationRejectionReason != nil {
		t.Fatal("approval audit/trust separation failed")
	}
	registrationRequest(t, router, "GET", "/api/company/opportunities", token, nil, http.StatusOK)
	me := registrationDecode[model.PublicUser](t, registrationRequest(t, router, "GET", "/api/auth/me", token, nil, http.StatusOK))
	if me.CompanyRegistrationStatus == nil || *me.CompanyRegistrationStatus != model.CompanyRegistrationStatusApproved {
		t.Fatal("existing JWT does not reflect Admin decision")
	}
	for _, action := range []string{"approve", "reject"} {
		registrationRequest(t, router, "POST", reviewPath+"/"+action, adminToken, map[string]string{"reason": "again"}, http.StatusConflict)
	}
	registrationRequest(t, router, "PUT", "/api/company/profile", token, registrationProfilePayload(input), http.StatusConflict)
	registrationRequest(t, router, "POST", "/api/company/registration/resubmit", token, nil, http.StatusConflict)
	registrationRequest(t, router, "GET", fmt.Sprintf("/api/admin/company-registrations/%d", otherCompany.ID), adminToken, nil, http.StatusOK)
	registrationRequest(t, router, "GET", "/api/admin/company-registrations?status=APPROVED", adminToken, nil, http.StatusOK)
	registrationRequest(t, router, "GET", "/api/admin/company-registrations?status=REJECTED", adminToken, nil, http.StatusOK)
	university := registrationToken(t, registrationDemoUser(t, db, "university"))
	for _, path := range []string{"/api/university/companies", "/api/university/company-supervisors"} {
		registrationRequest(t, router, "POST", path, university, map[string]any{}, http.StatusNotFound)
		registrationRequest(t, router, "GET", path, university, nil, http.StatusOK)
		for _, route := range router.Routes() {
			if route.Method == "POST" && route.Path == path {
				t.Fatalf("removed production endpoint still registered: %s", path)
			}
		}
	}
	registrationRequest(t, router, "GET", fmt.Sprintf("/api/university/companies/%d", company.ID), university, nil, http.StatusOK)
}

func TestRegistrationStudentPlacementAndTrustSeparation(t *testing.T) {
	db := registrationTestDB(t)
	router := registrationRouter(t, db)
	_, company, supervisor, token := registrationFixture(t, router, "business")
	adminToken := registrationToken(t, registrationDemoUser(t, db, "admin"))
	student := registrationDemoUser(t, db, "student")
	studentToken := registrationToken(t, student)
	university := registrationDemoUser(t, db, "university")
	universityToken := registrationToken(t, university)
	professor := registrationDemoUser(t, db, "professor")
	create := map[string]string{"title": "فرصت", "description": "شرح", "workField": "نرم‌افزار", "location": "تهران"}
	reviewPath := fmt.Sprintf("/api/admin/company-registrations/%d", company.ID)
	registrationRequest(t, router, "POST", reviewPath+"/approve", adminToken, nil, http.StatusOK)
	opportunity := registrationDecode[struct{ ID uint }](t, registrationRequest(t, router, "POST", "/api/company/opportunities", token, create, http.StatusCreated))
	opportunityPath := fmt.Sprintf("/api/company/opportunities/%d", opportunity.ID)
	registrationRequest(t, router, "PUT", opportunityPath, token, create, http.StatusOK)
	registrationRequest(t, router, "GET", opportunityPath, token, nil, http.StatusOK)
	registrationRequest(t, router, "GET", fmt.Sprintf("/api/student/opportunities/%d", opportunity.ID), studentToken, nil, http.StatusOK)
	// Use the production multipart apply endpoint with a real PDF signature.
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="resume"; filename="resume.pdf"`)
	header.Set("Content-Type", "application/pdf")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("%PDF-1.4\nresume\n%%EOF"))
	writer.Close()
	request := httptest.NewRequest("POST", fmt.Sprintf("/api/student/opportunities/%d/apply", opportunity.ID), &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+studentToken)
	applied := httptest.NewRecorder()
	router.ServeHTTP(applied, request)
	if applied.Code != http.StatusCreated {
		t.Fatalf("approved but untrusted company application: %d %s", applied.Code, applied.Body.String())
	}
	application := registrationDecode[struct {
		ID     uint
		Resume struct{ ID uint }
	}](t, applied)
	resumePath := fmt.Sprintf("/api/files/%d/download", application.Resume.ID)
	registrationRequest(t, router, "GET", resumePath, token, nil, http.StatusOK)
	applicationPath := fmt.Sprintf("/api/company/opportunity-applications/%d", application.ID)
	registrationRequest(t, router, "GET", opportunityPath+"/applications", token, nil, http.StatusOK)
	registrationRequest(t, router, "GET", applicationPath, token, nil, http.StatusOK)
	for _, status := range []model.CompanyRegistrationStatus{model.CompanyRegistrationStatusPending, model.CompanyRegistrationStatusRejected} {
		if err := db.Model(&company).Update("registration_status", status).Error; err != nil {
			t.Fatal(err)
		}
		assertRegistrationBusinessBlocked(t, router, token)
		registrationRequest(t, router, "GET", resumePath, token, nil, http.StatusForbidden)
		registrationRequest(t, router, "GET", fmt.Sprintf("/api/student/opportunities/%d", opportunity.ID), studentToken, nil, http.StatusNotFound)
		list := registrationRequest(t, router, "GET", "/api/student/opportunities", studentToken, nil, http.StatusOK)
		var items []struct{ ID uint }
		if err := json.Unmarshal(list.Body.Bytes(), &items); err != nil {
			t.Fatal(err)
		}
		for _, item := range items {
			if item.ID == opportunity.ID {
				t.Fatal("non-approved opportunity in catalog")
			}
		}
		registrationRequest(t, router, "POST", fmt.Sprintf("/api/student/opportunities/%d/apply", opportunity.ID), studentToken, nil, http.StatusForbidden)
		// Apply rechecks independently of the catalog and pre-upload eligibility.
		if _, err := service.NewOpportunityApplicationService(db).Apply(student.ID, opportunity.ID, &model.File{OriginalName: "resume.pdf", StoredName: "resume.pdf", Path: "fixture.pdf", MimeType: "application/pdf", SizeBytes: 10}); !errors.Is(err, service.ErrCompanyRegistrationRequired) {
			t.Fatalf("service application bypass: %v", err)
		}
	}
	if err := db.Model(&company).Update("registration_status", model.CompanyRegistrationStatusApproved).Error; err != nil {
		t.Fatal(err)
	}
	registrationRequest(t, router, "POST", applicationPath+"/accept", token, nil, http.StatusOK)
	registrationRequest(t, router, "POST", applicationPath+"/reject", token, nil, http.StatusConflict)
	term := model.InternshipTerm{AcademicYear: 1405, TermType: model.InternshipTermTypeSummer, Status: model.InternshipTermStatusOpen, OpenedAt: time.Now(), CreatedBy: university.ID}
	if err := db.Create(&term).Error; err != nil {
		t.Fatal(err)
	}
	item := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, TermID: &term.ID, Status: model.InternshipCaseStatusPendingUniversityReview}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	preference := model.InternshipPreference{InternshipCaseID: item.ID, OpportunityApplicationID: application.ID, Priority: 1}
	if err := db.Create(&preference).Error; err != nil {
		t.Fatal(err)
	}
	placement := service.UniversityPlacementApprovalInput{PreferenceID: preference.ID, LetterNumber: "17", LetterDate: time.Now()}
	workflow := service.NewInternshipService(db)
	for _, status := range []model.CompanyRegistrationStatus{model.CompanyRegistrationStatusPending, model.CompanyRegistrationStatusRejected} {
		if err := db.Model(&company).Update("registration_status", status).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := workflow.ApproveUniversityPlacement(item.ID, placement); !errors.Is(err, service.ErrCompanyRegistrationRequired) {
			t.Fatalf("non-approved placement selected: %v", err)
		}
	}
	if err := db.Model(&company).Update("registration_status", model.CompanyRegistrationStatusApproved).Error; err != nil {
		t.Fatal(err)
	}
	selected, err := workflow.ApproveUniversityPlacement(item.ID, placement)
	if err != nil {
		t.Fatal(err)
	}
	if selected.CompanySupervisorID == nil || *selected.CompanySupervisorID != supervisor.ID {
		t.Fatal("placement lost supervisor ownership")
	}
	casePath := fmt.Sprintf("/api/company/internship-cases/%d", item.ID)
	registrationRequest(t, router, "GET", casePath, token, nil, http.StatusOK)
	_, _, _, otherToken := registrationFixture(t, router, "ownership")
	// Approve the other company through the actual review API.
	var other model.User
	claims, err := auth.ParseToken(otherToken, registrationTestSecret)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.First(&other, claims.UserID).Error; err != nil {
		t.Fatal(err)
	}
	registrationRequest(t, router, "POST", fmt.Sprintf("/api/admin/company-registrations/%d/approve", *other.CompanyID), adminToken, nil, http.StatusOK)
	registrationRequest(t, router, "GET", casePath, otherToken, nil, http.StatusForbidden)
	registrationRequest(t, router, "GET", opportunityPath, otherToken, nil, http.StatusNotFound)
	registrationRequest(t, router, "GET", resumePath, otherToken, nil, http.StatusForbidden)
	registrationRequest(t, router, "POST", casePath+"/placement-details", token, map[string]string{"internshipSubject": "موضوع", "startDate": "2026-10-05", "workplaceAddress": "تهران", "workplacePhone": "02111111111"}, http.StatusOK)
	if _, err := workflow.ApprovePlacementDetails(item.ID); err != nil {
		t.Fatal(err)
	}
	registrationRequest(t, router, "GET", casePath+"/weekly-reports", token, nil, http.StatusOK)
	historyPath := fmt.Sprintf("/api/company/internship-cases/history/%d", item.ID)
	registrationRequest(t, router, "GET", historyPath, token, nil, http.StatusForbidden)
	// Historical report files remain available only through existing assignment authorization.
	var finalFile model.File
	if err := db.First(&finalFile, application.Resume.ID).Error; err != nil {
		t.Fatal(err)
	}
	finalFile.ID = 0
	finalFile.StoredName = fmt.Sprintf("final-%d.pdf", item.ID)
	if err := db.Create(&finalFile).Error; err != nil {
		t.Fatal(err)
	}
	finalReport := model.FinalReport{InternshipCaseID: item.ID, CurrentFileID: finalFile.ID, Status: model.FinalReportApproved, SubmittedAt: time.Now()}
	if err := db.Create(&finalReport).Error; err != nil {
		t.Fatal(err)
	}
	finalPath := fmt.Sprintf("/api/files/%d/download", finalFile.ID)
	if err := db.Model(&company).Update("registration_status", model.CompanyRegistrationStatusRejected).Error; err != nil {
		t.Fatal(err)
	}
	registrationRequest(t, router, "GET", finalPath, token, nil, http.StatusForbidden)
	// Terminal history remains readable despite inconsistent legacy registration.
	for _, status := range []model.InternshipCaseStatus{model.InternshipCaseStatusFailed, model.InternshipCaseStatusCancelled, model.InternshipCaseStatusPassed} {
		if err := db.Model(&item).Update("status", status).Error; err != nil {
			t.Fatal(err)
		}
		registrationRequest(t, router, "GET", historyPath, token, nil, http.StatusOK)
		registrationRequest(t, router, "GET", "/api/company/internship-cases/history", token, nil, http.StatusOK)
		registrationRequest(t, router, "GET", fmt.Sprintf("/api/student/internship-cases/history/%d", item.ID), studentToken, nil, http.StatusOK)
		registrationRequest(t, router, "GET", fmt.Sprintf("/api/university/internship-cases/%d", item.ID), universityToken, nil, http.StatusOK)
		registrationRequest(t, router, "GET", finalPath, token, nil, http.StatusOK)
		registrationRequest(t, router, "GET", finalPath, otherToken, nil, http.StatusForbidden)
	}

	if err := db.First(&company, company.ID).Error; err != nil {
		t.Fatal(err)
	}
	if company.IsApproved {
		t.Fatal("PASSED automatically set university trust")
	}
	registrationRequest(t, router, "POST", fmt.Sprintf("/api/university/companies/%d/approve", company.ID), universityToken, nil, http.StatusOK)
	if err := db.First(&company, company.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !company.IsApproved || company.RegistrationStatus != model.CompanyRegistrationStatusRejected {
		t.Fatal("university approval changed registration")
	}
	directory := registrationRequest(t, router, "GET", "/api/student/approved-companies", studentToken, nil, http.StatusOK)
	if !strings.Contains(directory.Body.String(), company.Name) {
		t.Fatal("trusted-company directory was conflated with registration permission")
	}
	// Admin approval preserves a pre-existing true university trust flag as well.
	if err := db.Model(&company).Update("registration_status", model.CompanyRegistrationStatusPending).Error; err != nil {
		t.Fatal(err)
	}
	registrationRequest(t, router, "POST", reviewPath+"/approve", adminToken, nil, http.StatusOK)
	if err := db.First(&company, company.ID).Error; err != nil || !company.IsApproved {
		t.Fatal("Admin approval overwrote university trust")
	}
	registrationRequest(t, router, "POST", opportunityPath+"/close", token, nil, http.StatusOK)
}

func TestRegistrationProfileCorrectionValidationAndIdentity(t *testing.T) {
	db := registrationTestDB(t)
	router := registrationRouter(t, db)
	input, company, supervisor, token := registrationFixture(t, router, "validation")
	otherInput, _, _, _ := registrationFixture(t, router, "unique")
	// Uniqueness failures roll back both records and never change status.
	for _, field := range []string{"name", "nationalId", "economicCode", "email"} {
		payload := registrationProfilePayload(input)
		if field == "email" {
			payload["supervisor"].(map[string]any)[field] = otherInput.Supervisor.Email
		} else {
			other := registrationProfilePayload(otherInput)
			payload["company"].(map[string]any)[field] = other["company"].(map[string]any)[field]
		}
		registrationRequest(t, router, "PUT", "/api/company/profile", token, payload, http.StatusConflict)
	}
	invalid := registrationProfilePayload(input)
	invalid["company"].(map[string]any)["name"] = strings.Repeat("x", 251)
	registrationRequest(t, router, "PUT", "/api/company/profile", token, invalid, http.StatusBadRequest)
	input.Supervisor.Email = "updated-" + input.Supervisor.Email
	registrationRequest(t, router, "PUT", "/api/company/profile", token, registrationProfilePayload(input), http.StatusOK)
	registrationRequest(t, router, "POST", "/api/auth/login", "", map[string]string{"email": input.Supervisor.Email, "password": input.Supervisor.Password}, http.StatusOK)
	var original model.User
	if err := db.First(&original, supervisor.ID).Error; err != nil {
		t.Fatal(err)
	}
	originalPassword := original.PasswordHash
	adminToken := registrationToken(t, registrationDemoUser(t, db, "admin"))
	registrationRequest(t, router, "POST", fmt.Sprintf("/api/admin/company-registrations/%d/reject", company.ID), adminToken, map[string]string{"reason": "مدارک را اصلاح کنید"}, http.StatusOK)
	profile := registrationDecode[struct{ Company model.Company }](t, registrationRequest(t, router, "GET", "/api/company/profile", token, nil, http.StatusOK))
	if profile.Company.RegistrationRejectionReason == nil || *profile.Company.RegistrationRejectionReason != "مدارک را اصلاح کنید" {
		t.Fatal("rejection reason not visible to company")
	}
	// Defensively validate required fields at resubmission, even for inconsistent legacy data.
	if err := db.Model(&company).Updates(map[string]any{"address": nil, "is_approved": true}).Error; err != nil {
		t.Fatal(err)
	}
	registrationRequest(t, router, "POST", "/api/company/registration/resubmit", token, nil, http.StatusBadRequest)
	registrationRequest(t, router, "PUT", "/api/company/profile", token, registrationProfilePayload(input), http.StatusOK)
	result := registrationDecode[struct {
		Company    model.Company
		Supervisor model.PublicUser
	}](t, registrationRequest(t, router, "POST", "/api/company/registration/resubmit", token, nil, http.StatusOK))
	if !result.Company.IsApproved || result.Company.RegistrationStatus != model.CompanyRegistrationStatusPending || result.Company.ID != company.ID || result.Supervisor.ID != supervisor.ID {
		t.Fatal("resubmission changed trust or identity")
	}
	if err := db.First(&original, supervisor.ID).Error; err != nil {
		t.Fatal(err)
	}
	if original.PasswordHash != originalPassword || original.Role != model.RoleCompanySupervisor || original.CompanyID == nil || *original.CompanyID != company.ID {
		t.Fatal("registration correction changed password/role/membership")
	}
}
