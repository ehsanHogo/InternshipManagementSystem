package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
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

func TestCompanyPlacementAPI(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Company{}, &model.User{}, &model.InternshipOpportunity{}, &model.ProfessorAssignment{}, &model.File{}, &model.OpportunityApplication{}, &model.InternshipCase{}, &model.InternshipPreference{}, &model.WeeklyReport{}, &model.CompanyEvaluation{}); err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	suffix := time.Now().UnixNano()
	companyA := createOpportunityTestCompany(t, tx, suffix, "pa", true)
	companyB := createOpportunityTestCompany(t, tx, suffix, "pb", false)
	supervisorA := createOpportunityTestUser(t, tx, suffix, "pa", model.RoleCompanySupervisor, &companyA.ID)
	supervisorB := createOpportunityTestUser(t, tx, suffix, "pb", model.RoleCompanySupervisor, &companyB.ID)
	student := createOpportunityTestUser(t, tx, suffix, "placement-student", model.RoleStudent, nil)
	professor := createOpportunityTestUser(t, tx, suffix, "placement-professor", model.RoleProfessor, nil)
	university := createOpportunityTestUser(t, tx, suffix, "placement-university", model.RoleUniversitySupervisor, nil)
	admin := createOpportunityTestUser(t, tx, suffix, "placement-admin", model.RoleAdmin, nil)
	const secret = "company-placement-integration-secret"
	token := func(user model.User) string {
		value, err := auth.CreateToken(user, secret, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	tokenA, tokenB, tokenStudent, tokenUniversity := token(supervisorA), token(supervisorB), token(student), token(university)
	handler := NewInternshipHandler(service.NewInternshipService(tx))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api")
	api.Use(appmiddleware.RequireAuth(secret))
	company := api.Group("/company")
	company.Use(appmiddleware.RequireRole(model.RoleCompanySupervisor))
	company.GET("/internship-cases", handler.ListCompanyCases)
	company.GET("/internship-cases/pending-details", handler.ListPendingCompanyDetailsCases)
	company.GET("/internship-cases/:id", handler.GetCompanyCase)
	company.POST("/internship-cases/:id/placement-details", handler.SubmitPlacementDetails)
	students := api.Group("/student")
	students.Use(appmiddleware.RequireRole(model.RoleStudent))
	students.GET("/internship-case", handler.GetCurrentCase)
	students.POST("/internship-case/submit", handler.SubmitCase)
	universities := api.Group("/university")
	universities.Use(appmiddleware.RequireRole(model.RoleUniversitySupervisor))
	universities.GET("/internship-cases", handler.ListUniversityCases)
	universities.GET("/internship-cases/:id", handler.GetUniversityCase)
	universities.POST("/internship-cases/:id/approve-placement", handler.ApproveUniversityPlacement)
	credits, mobile := 90, "09123456789"
	item := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusDraft, PassedCredits: &credits, Mobile: &mobile}
	if err := tx.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	for i, user := range []model.User{supervisorA, supervisorB} {
		opportunity := model.InternshipOpportunity{CompanyID: *user.CompanyID, CreatedBy: user.ID, Title: fmt.Sprintf("placement %d", i), Description: "description", WorkField: "software", Location: "Tehran", Status: model.OpportunityStatusOpen}
		if err := tx.Create(&opportunity).Error; err != nil {
			t.Fatal(err)
		}
		file := model.File{OriginalName: "resume.pdf", StoredName: fmt.Sprintf("placement-%d-%d.pdf", suffix, i), Path: "/tmp/resume.pdf", MimeType: "application/pdf", SizeBytes: 100, UploadedBy: student.ID, UploadedAt: time.Now()}
		if err := tx.Create(&file).Error; err != nil {
			t.Fatal(err)
		}
		application := model.OpportunityApplication{OpportunityID: opportunity.ID, StudentID: student.ID, ResumeFileID: file.ID, Status: model.ApplicationStatusAccepted, AppliedAt: time.Now()}
		if err := tx.Create(&application).Error; err != nil {
			t.Fatal(err)
		}
		preference := model.InternshipPreference{InternshipCaseID: item.ID, OpportunityApplicationID: application.ID, Priority: i + 1}
		if err := tx.Create(&preference).Error; err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			item.SelectedPreferenceID = &preference.ID
		}
	}
	result := performJSONRequest(t, router, http.MethodPost, "/api/student/internship-case/submit", map[string]any{}, tokenStudent)
	if result.Code != http.StatusOK {
		t.Fatalf("student submit=%d %s", result.Code, result.Body)
	}
	universityPath := fmt.Sprintf("/api/university/internship-cases/%d", item.ID)
	result = performJSONRequest(t, router, http.MethodPost, universityPath+"/approve-placement", map[string]any{"preferenceId": *item.SelectedPreferenceID, "letterNumber": "M7-100", "letterDate": "2026-09-01"}, tokenUniversity)
	if result.Code != http.StatusOK {
		t.Fatalf("university placement=%d %s", result.Code, result.Body)
	}
	path := fmt.Sprintf("/api/company/internship-cases/%d", item.ID)
	payload := map[string]any{"internshipSubject": "  نرم‌افزار  ", "startDate": "2020-01-02", "workplaceAddress": "  آدرس محل  ", "workplacePhone": "  021-123  "}
	load := func() model.InternshipCase {
		var value model.InternshipCase
		if err := tx.First(&value, item.ID).Error; err != nil {
			t.Fatal(err)
		}
		return value
	}
	assertUnchanged := func(before model.InternshipCase) {
		if after := load(); !reflect.DeepEqual(before, after) {
			t.Fatalf("rejected request changed case: %+v -> %+v", before, after)
		}
	}

	t.Run("pending list selected vs non-selected companies", func(t *testing.T) {
		for _, test := range []struct {
			token string
			count int
		}{{tokenA, 0}, {tokenB, 1}} {
			result := performJSONRequest(t, router, http.MethodGet, "/api/company/internship-cases/pending-details", nil, test.token)
			var cases []internshipCaseResponse
			if result.Code != http.StatusOK {
				t.Fatalf("list: %d %s", result.Code, result.Body)
			}
			if err := json.Unmarshal(result.Body.Bytes(), &cases); err != nil {
				t.Fatal(err)
			}
			if len(cases) != test.count {
				t.Fatalf("pending count=%d want=%d", len(cases), test.count)
			}
		}
	})
	t.Run("detail returns selected preference and student contact only", func(t *testing.T) {
		result := performJSONRequest(t, router, http.MethodGet, path, nil, tokenB)
		if result.Code != http.StatusOK {
			t.Fatalf("detail=%d %s", result.Code, result.Body)
		}
		var view internshipCaseResponse
		if err := json.Unmarshal(result.Body.Bytes(), &view); err != nil {
			t.Fatal(err)
		}
		if len(view.Preferences) != 1 || view.Preferences[0].Priority != 2 || view.SelectedPreference == nil || view.CompanySupervisor == nil || view.CompanySupervisor.ID != supervisorB.ID || view.LetterNumber == nil || view.Mobile == nil || view.PassedCredits != nil || view.Student.Email != student.Email {
			t.Fatalf("incorrect or excessive company response: %+v", view)
		}
	})
	t.Run("cross-company detail and mutation forbidden", func(t *testing.T) {
		before := load()
		assertAPIError(t, performJSONRequest(t, router, http.MethodGet, path, nil, tokenA), http.StatusForbidden, "INTERNSHIP_CASE_NOT_ASSIGNED_TO_COMPANY")
		assertAPIError(t, performJSONRequest(t, router, http.MethodPost, path+"/placement-details", payload, tokenA), http.StatusForbidden, "INTERNSHIP_CASE_NOT_ASSIGNED_TO_COMPANY")
		assertUnchanged(before)
	})
	t.Run("role security including admin and unauthenticated", func(t *testing.T) {
		for _, user := range []model.User{student, professor, university, admin} {
			before := load()
			result := performJSONRequest(t, router, http.MethodPost, path+"/placement-details", payload, token(user))
			if result.Code != http.StatusForbidden {
				t.Fatalf("%s mutation=%d", user.Role, result.Code)
			}
			assertUnchanged(before)
		}
		result := performJSONRequest(t, router, http.MethodPost, path+"/placement-details", payload, "")
		if result.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated=%d", result.Code)
		}
	})
	t.Run("unknown controlled IDs and status are rejected", func(t *testing.T) {
		for _, key := range []string{"companyId", "company_id", "companySupervisorId", "company_supervisor_id", "studentId", "student_id", "selectedPreferenceId", "selected_preference_id", "status"} {
			before := load()
			values := map[string]any{}
			for k, v := range payload {
				values[k] = v
			}
			values[key] = companyA.ID
			assertAPIError(t, performJSONRequest(t, router, http.MethodPost, path+"/placement-details", values, tokenB), http.StatusBadRequest, "INVALID_PLACEMENT_DETAILS")
			assertUnchanged(before)
		}
	})
	t.Run("missing blank and invalid placement fields", func(t *testing.T) {
		for field, code := range map[string]string{"internshipSubject": "INTERNSHIP_SUBJECT_REQUIRED", "startDate": "START_DATE_REQUIRED", "workplaceAddress": "WORKPLACE_ADDRESS_REQUIRED", "workplacePhone": "WORKPLACE_PHONE_REQUIRED"} {
			for _, missing := range []bool{true, false} {
				before := load()
				values := map[string]any{}
				for k, v := range payload {
					values[k] = v
				}
				if missing {
					delete(values, field)
				} else {
					values[field] = " \t "
				}
				assertAPIError(t, performJSONRequest(t, router, http.MethodPost, path+"/placement-details", values, tokenB), http.StatusBadRequest, code)
				assertUnchanged(before)
			}
		}
		for _, date := range []string{"2026-02-30", "1405/07/18", "not-a-date", "0000-01-01"} {
			before := load()
			values := map[string]any{}
			for k, v := range payload {
				values[k] = v
			}
			values["startDate"] = date
			assertAPIError(t, performJSONRequest(t, router, http.MethodPost, path+"/placement-details", values, tokenB), http.StatusBadRequest, "INVALID_START_DATE")
			assertUnchanged(before)
		}
	})
	t.Run("unapproved selected company submits and student university company see read-only data", func(t *testing.T) {
		if err := tx.Model(&model.InternshipCase{}).Where("id = ?", item.ID).Update("company_details_revision_comment", "اصلاحات").Error; err != nil {
			t.Fatal(err)
		}
		result := performJSONRequest(t, router, http.MethodPost, path+"/placement-details", payload, tokenB)
		if result.Code != http.StatusOK {
			t.Fatalf("submit=%d %s", result.Code, result.Body)
		}
		persisted := load()
		if persisted.Status != model.InternshipCaseStatusPendingFinalApproval || persisted.CompanyDetailsRevisionComment != nil || persisted.InternshipSubject == nil || *persisted.InternshipSubject != "نرم‌افزار" {
			t.Fatalf("bad transition %+v", persisted)
		}
		for _, get := range []struct{ path, token string }{{path, tokenB}, {"/api/student/internship-case", tokenStudent}, {universityPath, tokenUniversity}} {
			result := performJSONRequest(t, router, http.MethodGet, get.path, nil, get.token)
			var view internshipCaseResponse
			if result.Code != http.StatusOK {
				t.Fatalf("read=%d %s", result.Code, result.Body)
			}
			if err := json.Unmarshal(result.Body.Bytes(), &view); err != nil {
				t.Fatal(err)
			}
			if view.Status != model.InternshipCaseStatusPendingFinalApproval || view.InternshipSubject == nil || view.StartDate == nil || view.WorkplaceAddress == nil || view.WorkplacePhone == nil {
				t.Fatalf("incomplete placement response %+v", view)
			}
		}
		for _, list := range []struct {
			path, token string
			count       int
		}{{"/api/company/internship-cases/pending-details", tokenB, 0}, {"/api/company/internship-cases", tokenB, 1}, {"/api/university/internship-cases?status=PENDING_FINAL_APPROVAL", tokenUniversity, 1}} {
			result := performJSONRequest(t, router, http.MethodGet, list.path, nil, list.token)
			var values []internshipCaseResponse
			if err := json.Unmarshal(result.Body.Bytes(), &values); err != nil {
				t.Fatal(err)
			}
			if result.Code != http.StatusOK || len(values) != list.count {
				t.Fatalf("list %s=%d count=%d", list.path, result.Code, len(values))
			}
		}
		before := load()
		assertAPIError(t, performJSONRequest(t, router, http.MethodPost, path+"/placement-details", payload, tokenB), http.StatusConflict, "INTERNSHIP_CASE_NOT_PENDING_COMPANY_DETAILS")
		assertUnchanged(before)
		var company model.Company
		if err := tx.First(&company, companyB.ID).Error; err != nil {
			t.Fatal(err)
		}
		if company.IsApproved {
			t.Fatal("company auto-approved")
		}
	})
}
