package handler

import (
	"bytes"
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
	if err := db.AutoMigrate(&model.Company{}, &model.User{}, &model.InternshipOpportunity{}, &model.ProfessorAssignment{}, &model.File{}, &model.OpportunityApplication{}, &model.InternshipCase{}, &model.FinalReport{}, &model.InternshipPreference{}, &model.WeeklyReport{}, &model.CompanyEvaluation{}); err != nil {
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
	universities.GET("/internship-cases/pending-final-approval", handler.ListPendingFinalApprovalCases)
	universities.GET("/internship-cases/:id", handler.GetUniversityCase)
	universities.POST("/internship-cases/:id/approve-placement", handler.ApproveUniversityPlacement)
	universities.POST("/internship-cases/:id/final-approve", handler.ApprovePlacementDetails)
	universities.POST("/internship-cases/:id/activate", handler.ActivateUniversityCase)
	universities.POST("/internship-cases/:id/request-placement-correction", handler.RequestPlacementCorrection)
	professors := api.Group("/professor")
	professors.Use(appmiddleware.RequireRole(model.RoleProfessor))
	professors.GET("/internship-cases/:id", handler.GetProfessorCase)
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
		if bytes.Contains(result.Body.Bytes(), []byte(`"priority"`)) {
			t.Fatal("company JSON exposes preference priority")
		}
		var view internshipCaseResponse
		if err := json.Unmarshal(result.Body.Bytes(), &view); err != nil {
			t.Fatal(err)
		}
		if len(view.Preferences) != 1 || view.Preferences[0].Priority != 0 || view.SelectedPreference == nil || view.SelectedPreference.Priority != 0 || view.CompanySupervisor == nil || view.CompanySupervisor.ID != supervisorB.ID || view.LetterNumber == nil || view.Mobile == nil || view.PassedCredits != nil || view.Student.Email != student.Email {
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
	t.Run("university final review role security including admin and unauthenticated", func(t *testing.T) {
		for _, endpoint := range []struct{ method, path string }{
			{http.MethodGet, "/api/university/internship-cases/pending-final-approval"},
			{http.MethodGet, universityPath},
			{http.MethodPost, universityPath + "/final-approve"},
			{http.MethodPost, universityPath + "/activate"},
			{http.MethodPost, universityPath + "/request-placement-correction"},
		} {
			for _, user := range []model.User{student, professor, supervisorA, supervisorB, admin} {
				before := load()
				result := performJSONRequest(t, router, endpoint.method, endpoint.path, map[string]any{"comment": "اصلاح"}, token(user))
				if result.Code != http.StatusForbidden {
					t.Fatalf("%s %s as %s: %d %s", endpoint.method, endpoint.path, user.Role, result.Code, result.Body)
				}
				assertUnchanged(before)
			}
			before := load()
			result := performJSONRequest(t, router, endpoint.method, endpoint.path, map[string]any{"comment": "اصلاح"}, "")
			if result.Code != http.StatusUnauthorized {
				t.Fatalf("unauthenticated %s: %d", endpoint.path, result.Code)
			}
			assertUnchanged(before)
		}
	})
	t.Run("final approval integrity and correction validation use stable errors", func(t *testing.T) {
		before := load()
		if err := tx.Model(&model.InternshipCase{}).Where("id = ?", item.ID).Update("workplace_phone", nil).Error; err != nil {
			t.Fatal(err)
		}
		corrupt := load()
		assertAPIError(t, performJSONRequest(t, router, http.MethodPost, universityPath+"/final-approve", map[string]any{}, tokenUniversity), http.StatusConflict, "PLACEMENT_DETAILS_INCOMPLETE")
		assertUnchanged(corrupt)
		if err := tx.Model(&model.InternshipCase{}).Where("id = ?", item.ID).Update("workplace_phone", before.WorkplacePhone).Error; err != nil {
			t.Fatal(err)
		}
		var selected model.InternshipPreference
		if err := tx.First(&selected, *before.SelectedPreferenceID).Error; err != nil {
			t.Fatal(err)
		}
		if err := tx.Model(&model.OpportunityApplication{}).Where("id = ?", selected.OpportunityApplicationID).Update("status", model.ApplicationStatusRejected).Error; err != nil {
			t.Fatal(err)
		}
		corrupt = load()
		assertAPIError(t, performJSONRequest(t, router, http.MethodPost, universityPath+"/final-approve", map[string]any{}, tokenUniversity), http.StatusConflict, "PLACEMENT_RELATIONSHIP_INVALID")
		assertUnchanged(corrupt)
		if err := tx.Model(&model.OpportunityApplication{}).Where("id = ?", selected.OpportunityApplicationID).Update("status", model.ApplicationStatusAccepted).Error; err != nil {
			t.Fatal(err)
		}
		for _, body := range []map[string]any{{}, {"comment": ""}, {"comment": " \t\n "}} {
			before := load()
			assertAPIError(t, performJSONRequest(t, router, http.MethodPost, universityPath+"/request-placement-correction", body, tokenUniversity), http.StatusBadRequest, "COMPANY_DETAILS_REVISION_COMMENT_REQUIRED")
			assertUnchanged(before)
		}
		for _, field := range []string{"internshipSubject", "startDate", "workplaceAddress", "workplacePhone", "selectedPreferenceId", "companySupervisorId", "letterNumber", "letterDate", "status"} {
			before := load()
			assertAPIError(t, performJSONRequest(t, router, http.MethodPost, universityPath+"/request-placement-correction", map[string]any{"comment": "اصلاح", field: "changed"}, tokenUniversity), http.StatusBadRequest, "INVALID_PLACEMENT_CORRECTION")
			assertUnchanged(before)
		}
		for _, action := range []string{"/final-approve", "/request-placement-correction"} {
			assertAPIError(t, performJSONRequest(t, router, http.MethodPost, "/api/university/internship-cases/9223372036854775807"+action, map[string]any{"comment": "اصلاح"}, tokenUniversity), http.StatusNotFound, "INTERNSHIP_CASE_NOT_FOUND")
		}
	})
	t.Run("university correction company resubmission final approval API loop", func(t *testing.T) {
		assertList := func(path, token string, count int) {
			t.Helper()
			result := performJSONRequest(t, router, http.MethodGet, path, nil, token)
			var values []internshipCaseResponse
			if result.Code != http.StatusOK {
				t.Fatalf("list %s: %d %s", path, result.Code, result.Body)
			}
			if err := json.Unmarshal(result.Body.Bytes(), &values); err != nil {
				t.Fatal(err)
			}
			if len(values) != count {
				t.Fatalf("list %s count=%d want=%d", path, len(values), count)
			}
		}
		assertList("/api/university/internship-cases/pending-final-approval", tokenUniversity, 1)
		before := load()
		result := performJSONRequest(t, router, http.MethodPost, universityPath+"/request-placement-correction", map[string]any{"comment": "  آدرس را اصلاح کنید  "}, tokenUniversity)
		if result.Code != http.StatusOK {
			t.Fatalf("correct: %d %s", result.Code, result.Body)
		}
		comment := "آدرس را اصلاح کنید"
		after := load()
		before.Status, before.CompanyDetailsRevisionComment, before.UpdatedAt = model.InternshipCaseStatusPendingCompanyDetails, &comment, after.UpdatedAt
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("correction changed other case data: %+v -> %+v", before, after)
		}
		assertList("/api/university/internship-cases/pending-final-approval", tokenUniversity, 0)
		assertList("/api/company/internship-cases/pending-details", tokenB, 1)
		assertList("/api/company/internship-cases/pending-details", tokenA, 0)
		assertAPIError(t, performJSONRequest(t, router, http.MethodPost, universityPath+"/request-placement-correction", map[string]any{"comment": "دوباره"}, tokenUniversity), http.StatusConflict, "INTERNSHIP_CASE_NOT_PENDING_FINAL_APPROVAL")
		assertAPIError(t, performJSONRequest(t, router, http.MethodPost, universityPath+"/final-approve", map[string]any{}, tokenUniversity), http.StatusConflict, "INTERNSHIP_CASE_NOT_PENDING_FINAL_APPROVAL")
		assertUnchanged(after)
		result = performJSONRequest(t, router, http.MethodGet, path, nil, tokenB)
		var companyView internshipCaseResponse
		if result.Code != http.StatusOK {
			t.Fatalf("company correction detail: %d %s", result.Code, result.Body)
		}
		if err := json.Unmarshal(result.Body.Bytes(), &companyView); err != nil {
			t.Fatal(err)
		}
		if companyView.CompanyDetailsRevisionComment == nil || *companyView.CompanyDetailsRevisionComment != comment || companyView.WorkplaceAddress == nil || *companyView.WorkplaceAddress != "آدرس محل" || bytes.Contains(result.Body.Bytes(), []byte(`"priority"`)) {
			t.Fatalf("company correction detail: %s", result.Body)
		}
		assertAPIError(t, performJSONRequest(t, router, http.MethodPost, path+"/placement-details", payload, tokenA), http.StatusForbidden, "INTERNSHIP_CASE_NOT_ASSIGNED_TO_COMPANY")
		assertUnchanged(after)
		payload["workplaceAddress"] = "آدرس اصلاح‌شده"
		result = performJSONRequest(t, router, http.MethodPost, path+"/placement-details", payload, tokenB)
		if result.Code != http.StatusOK {
			t.Fatalf("resubmit: %d %s", result.Code, result.Body)
		}
		after = load()
		if after.Status != model.InternshipCaseStatusPendingFinalApproval || after.CompanyDetailsRevisionComment != nil || *after.WorkplaceAddress != "آدرس اصلاح‌شده" {
			t.Fatalf("resubmit: %+v", after)
		}
		assertList("/api/university/internship-cases/pending-final-approval", tokenUniversity, 1)
		result = performJSONRequest(t, router, http.MethodPost, universityPath+"/final-approve", map[string]any{"workplaceAddress": "must not overwrite", "letterNumber": "must not overwrite"}, tokenUniversity)
		if result.Code != http.StatusOK {
			t.Fatalf("approve: %d %s", result.Code, result.Body)
		}
		ready := load()
		after.Status, after.UpdatedAt = model.InternshipCaseStatusReadyToStart, ready.UpdatedAt
		if !reflect.DeepEqual(after, ready) {
			t.Fatalf("approval changed other case data: %+v -> %+v", after, ready)
		}
		assertList("/api/university/internship-cases/pending-final-approval", tokenUniversity, 0)
		assertList("/api/company/internship-cases/pending-details", tokenB, 0)
		for _, action := range []string{"/final-approve", "/request-placement-correction"} {
			assertAPIError(t, performJSONRequest(t, router, http.MethodPost, universityPath+action, map[string]any{"comment": "اصلاح"}, tokenUniversity), http.StatusConflict, "INTERNSHIP_CASE_NOT_PENDING_FINAL_APPROVAL")
		}
		assertAPIError(t, performJSONRequest(t, router, http.MethodPost, path+"/placement-details", payload, tokenB), http.StatusConflict, "INTERNSHIP_CASE_NOT_PENDING_COMPANY_DETAILS")
		assertUnchanged(ready)
		for _, get := range []struct {
			path, token string
			company     bool
		}{{path, tokenB, true}, {"/api/student/internship-case", tokenStudent, false}, {universityPath, tokenUniversity, false}} {
			result := performJSONRequest(t, router, http.MethodGet, get.path, nil, get.token)
			var view internshipCaseResponse
			if result.Code != http.StatusOK {
				t.Fatalf("ready detail: %d %s", result.Code, result.Body)
			}
			if err := json.Unmarshal(result.Body.Bytes(), &view); err != nil {
				t.Fatal(err)
			}
			if view.Status != model.InternshipCaseStatusReadyToStart || view.ActivatedAt != nil || view.WorkplaceAddress == nil || *view.WorkplaceAddress != "آدرس اصلاح‌شده" || view.SelectedPreference == nil {
				t.Fatalf("ready detail: %+v", view)
			}
			if get.company {
				if bytes.Contains(result.Body.Bytes(), []byte(`"priority"`)) {
					t.Fatal("company ready response exposes priority")
				}
			} else if view.SelectedPreference.Priority != 2 || len(view.Preferences) != 2 {
				t.Fatal("student/university preference ordering lost")
			}
		}
		assertList("/api/university/internship-cases?status=READY_TO_START", tokenUniversity, 1)
		for _, body := range []map[string]any{{}, {"status": "ACTIVE"}, {"activatedAt": "2099-01-01"}, {"studentId": student.ID}, {"companyId": companyA.ID}, {"supervisorId": supervisorA.ID}} {
			assertAPIError(t, performJSONRequest(t, router, http.MethodPost, universityPath+"/activate", body, tokenUniversity), http.StatusBadRequest, "INVALID_INTERNSHIP_CASE_ACTIVATION")
			assertUnchanged(ready)
		}
		if err := tx.Model(&item).Update("start_date", nil).Error; err != nil {
			t.Fatal(err)
		}
		corrupt := load()
		assertAPIError(t, performJSONRequest(t, router, http.MethodPost, universityPath+"/activate", nil, tokenUniversity), http.StatusConflict, "INTERNSHIP_CASE_ACTIVATION_INTEGRITY_FAILED")
		assertUnchanged(corrupt)
		if err := tx.Model(&item).UpdateColumns(map[string]any{"start_date": ready.StartDate, "updated_at": ready.UpdatedAt}).Error; err != nil {
			t.Fatal(err)
		}
		assertAPIError(t, performJSONRequest(t, router, http.MethodPost, "/api/university/internship-cases/9223372036854775807/activate", nil, tokenUniversity), http.StatusNotFound, "INTERNSHIP_CASE_NOT_FOUND")
		snapshot := func() []any {
			var companies []model.Company
			var users []model.User
			var opportunities []model.InternshipOpportunity
			var applications []model.OpportunityApplication
			var preferences []model.InternshipPreference
			var files []model.File
			for _, dest := range []any{&companies, &users, &opportunities, &applications, &preferences, &files} {
				if err := tx.Order("id").Find(dest).Error; err != nil {
					t.Fatal(err)
				}
			}
			return []any{companies, users, opportunities, applications, preferences, files}
		}
		external := snapshot()
		result = performJSONRequest(t, router, http.MethodPost, universityPath+"/activate", nil, tokenUniversity)
		if result.Code != http.StatusOK {
			t.Fatalf("activate: %d %s", result.Code, result.Body)
		}
		active := load()
		if active.Status != model.InternshipCaseStatusActive || active.ActivatedAt == nil {
			t.Fatalf("activation: %+v", active)
		}
		ready.Status, ready.ActivatedAt = model.InternshipCaseStatusActive, active.ActivatedAt
		assertUnchanged(ready)
		if !reflect.DeepEqual(external, snapshot()) {
			t.Fatal("activation changed recruitment, files, company approval or user affiliation")
		}
		assertAPIError(t, performJSONRequest(t, router, http.MethodPost, universityPath+"/activate", nil, tokenUniversity), http.StatusConflict, "INTERNSHIP_CASE_NOT_READY_TO_START")
		assertAPIError(t, performJSONRequest(t, router, http.MethodPost, path+"/placement-details", payload, tokenB), http.StatusConflict, "INTERNSHIP_CASE_NOT_PENDING_COMPANY_DETAILS")
		assertUnchanged(active)
		assertList("/api/university/internship-cases?status=READY_TO_START", tokenUniversity, 0)
		assertList("/api/university/internship-cases?status=ACTIVE", tokenUniversity, 1)
		for _, get := range []struct{ path, token string }{{path, tokenB}, {"/api/student/internship-case", tokenStudent}, {universityPath, tokenUniversity}} {
			result := performJSONRequest(t, router, http.MethodGet, get.path, nil, get.token)
			var view internshipCaseResponse
			if result.Code != http.StatusOK {
				t.Fatalf("active detail: %d %s", result.Code, result.Body)
			}
			if err := json.Unmarshal(result.Body.Bytes(), &view); err != nil {
				t.Fatal(err)
			}
			if view.Status != model.InternshipCaseStatusActive || view.ActivatedAt == nil || view.WorkplaceAddress == nil || *view.WorkplaceAddress != "آدرس اصلاح‌شده" {
				t.Fatalf("active detail: %+v", view)
			}
			if get.path == path && bytes.Contains(result.Body.Bytes(), []byte(`"priority"`)) {
				t.Fatal("company active response exposes priority")
			}
		}
		result = performJSONRequest(t, router, http.MethodGet, fmt.Sprintf("/api/professor/internship-cases/%d", item.ID), nil, token(professor))
		var professorView struct {
			Internship struct {
				Status model.InternshipCaseStatus `json:"status"`
			} `json:"internship"`
		}
		if result.Code != http.StatusOK {
			t.Fatalf("professor active detail: %d %s", result.Code, result.Body)
		}
		if err := json.Unmarshal(result.Body.Bytes(), &professorView); err != nil {
			t.Fatal(err)
		}
		if professorView.Internship.Status != model.InternshipCaseStatusActive {
			t.Fatalf("professor cannot see active status: %s", result.Body)
		}

	})
}
