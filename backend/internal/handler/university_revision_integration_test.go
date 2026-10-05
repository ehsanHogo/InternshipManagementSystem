package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
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

func TestUniversityRevisionAPIAndRecruitmentEligibility(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Company{}, &model.User{}, &model.InternshipTerm{}, &model.ProfessorAssignment{}, &model.InternshipOpportunity{}, &model.File{}, &model.OpportunityApplication{}, &model.InternshipCase{}, &model.InternshipPreference{}, &model.FinalReport{}, &model.WeeklyReport{}, &model.CompanyEvaluation{}); err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	suffix := time.Now().UnixNano()
	university := createOpportunityTestUser(t, tx, suffix, "revision-api-university", model.RoleUniversitySupervisor, nil)
	student := createOpportunityTestUser(t, tx, suffix, "revision-api-student", model.RoleStudent, nil)
	professor := createOpportunityTestUser(t, tx, suffix, "revision-api-professor", model.RoleProfessor, nil)
	company := createOpportunityTestCompany(t, tx, suffix, "revision-api", true)
	supervisor := createOpportunityTestUser(t, tx, suffix, "revision-api-company", model.RoleCompanySupervisor, &company.ID)
	admin := createOpportunityTestUser(t, tx, suffix, "revision-api-admin", model.RoleAdmin, nil)
	term, err := service.NewInternshipTermService(tx).Create(university.ID, 1405, model.InternshipTermTypeSummer)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.ProfessorAssignment{StudentID: student.ID, ProfessorID: professor.ID, AssignedAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	workflow := service.NewInternshipService(tx)
	item, _, err := workflow.CreateOrGetCase(student.ID)
	if err != nil {
		t.Fatal(err)
	}
	credits, mobile := 90, "09123456789"
	if _, err := workflow.UpdateCase(student.ID, &credits, &mobile); err != nil {
		t.Fatal(err)
	}
	opportunity := model.InternshipOpportunity{CompanyID: company.ID, CreatedBy: supervisor.ID, Title: "فرصت جدید", Description: "شرح", WorkField: "software", Location: "Tehran", Status: model.OpportunityStatusOpen}
	if err := tx.Create(&opportunity).Error; err != nil {
		t.Fatal(err)
	}
	// Use another opportunity for the initial accepted preference; the new one remains unapplied.
	oldOpportunity := opportunity
	oldOpportunity.ID = 0
	oldOpportunity.Title = "فرصت اولیه"
	if err := tx.Create(&oldOpportunity).Error; err != nil {
		t.Fatal(err)
	}
	resume := model.File{OriginalName: "resume.pdf", StoredName: fmt.Sprintf("revision-api-%d.pdf", suffix), Path: "/tmp/resume.pdf", MimeType: "application/pdf", SizeBytes: 100}
	application, err := service.NewOpportunityApplicationService(tx).Apply(student.ID, oldOpportunity.ID, &resume)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.NewOpportunityApplicationService(tx).Review(supervisor.ID, application.ID, model.ApplicationStatusAccepted, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := workflow.ReplacePreferences(student.ID, []uint{application.ID}); err != nil {
		t.Fatal(err)
	}
	item, err = workflow.SubmitCase(student.ID)
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	const secret = "m16-revision-api-test"
	api := router.Group("/api", appmiddleware.RequireAuth(secret))
	universityGroup := api.Group("/university", appmiddleware.RequireRole(model.RoleUniversitySupervisor))
	caseHandler := NewInternshipHandler(workflow)
	caseHandler.RegisterUniversityCaseRoutes(universityGroup)
	universityGroup.GET("/internship-cases", caseHandler.ListUniversityCases)
	universityGroup.GET("/internship-cases/:id", caseHandler.GetUniversityCase)
	studentGroup := api.Group("/student", appmiddleware.RequireRole(model.RoleStudent))
	studentGroup.GET("/internship-case", caseHandler.GetCurrentCase)
	studentGroup.POST("/internship-case", caseHandler.CreateOrGetCase)
	studentGroup.POST("/internship-case/submit", caseHandler.SubmitCase)
	studentGroup.PUT("/internship-case", caseHandler.UpdateCase)
	studentGroup.PUT("/internship-case/preferences", caseHandler.ReplacePreferences)
	opportunities := NewOpportunityHandler(service.NewOpportunityService(tx), service.NewOpportunityApplicationService(tx))
	studentGroup.GET("/opportunities", opportunities.ListStudent)
	studentGroup.GET("/opportunities/:id", opportunities.GetStudent)
	token := func(actor model.User) string {
		t.Helper()
		if actor.ID == 0 {
			return ""
		}
		value, err := auth.CreateToken(actor, secret, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	path := fmt.Sprintf("/api/university/internship-cases/%d", item.ID)
	for _, actor := range []model.User{student, professor, supervisor, admin, {}} {
		status := http.StatusForbidden
		if actor.ID == 0 {
			status = http.StatusUnauthorized
		}
		result := performJSONRequest(t, router, "POST", path+"/request-revision", map[string]any{"comment": "اصلاح"}, token(actor))
		if result.Code != status {
			t.Fatalf("role %s: %d %s", actor.Role, result.Code, result.Body)
		}
	}
	for _, comment := range []string{"", " \n\t"} {
		assertAPIError(t, performJSONRequest(t, router, "POST", path+"/request-revision", map[string]any{"comment": comment}, token(university)), 400, "UNIVERSITY_REVISION_COMMENT_REQUIRED")
	}
	assertAPIError(t, performJSONRequest(t, router, "POST", path+"/request-revision", map[string]any{"comment": "اصلاح", "termId": 0}, token(university)), 400, "INVALID_UNIVERSITY_REVISION")
	comment := "لطفاً اولویت‌های جدید انتخاب کنید."
	result := performJSONRequest(t, router, "POST", path+"/request-revision", map[string]any{"comment": comment}, token(university))
	if result.Code != 200 {
		t.Fatalf("revision: %d %s", result.Code, result.Body)
	}
	for _, readPath := range []string{path, "/api/student/internship-case"} {
		actor := university
		if readPath == "/api/student/internship-case" {
			actor = student
		}
		result := performJSONRequest(t, router, "GET", readPath, nil, token(actor))
		var view internshipCaseResponse
		if err := json.Unmarshal(result.Body.Bytes(), &view); err != nil {
			t.Fatal(err)
		}
		if result.Code != 200 || view.Status != model.InternshipCaseStatusRevisionRequested || view.UniversityRevisionComment == nil || *view.UniversityRevisionComment != comment || view.UniversityRevisionRequestedBy == nil || *view.UniversityRevisionRequestedBy != university.ID || view.UniversityRevisionRequestedAt == nil || view.TermID == nil || *view.TermID != term.ID || view.Professor.ID != professor.ID {
			t.Fatalf("revision view: %d %+v", result.Code, view)
		}
	}
	result = performJSONRequest(t, router, "GET", "/api/university/internship-cases?status=REVISION_REQUESTED", nil, token(university))
	var list []internshipCaseResponse
	if err := json.Unmarshal(result.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if result.Code != 200 || len(list) != 1 || list[0].ID != item.ID {
		t.Fatalf("revision filter: %d %+v", result.Code, list)
	}
	result = performJSONRequest(t, router, "POST", "/api/student/internship-case", nil, token(student))
	var same internshipCaseResponse
	if err := json.Unmarshal(result.Body.Bytes(), &same); err != nil {
		t.Fatal(err)
	}
	if result.Code != 200 || same.ID != item.ID {
		t.Fatalf("same case: %d %+v", result.Code, same)
	}
	assertAPIError(t, performJSONRequest(t, router, "PUT", "/api/student/internship-case", map[string]any{"passedCredits": 95, "mobile": "09000000000"}, token(student)), 409, "INTERNSHIP_CASE_NOT_EDITABLE")
	assertAPIError(t, performJSONRequest(t, router, "POST", path+"/approve-placement", map[string]any{"preferenceId": item.Preferences[0].ID, "letterNumber": "M16", "letterDate": "2026-10-05"}, token(university)), 409, "INTERNSHIP_CASE_NOT_PENDING_UNIVERSITY_REVIEW")
	result = performJSONRequest(t, router, "PUT", "/api/student/internship-case/preferences", map[string]any{"opportunityApplicationIds": []uint{application.ID}}, token(student))
	if result.Code != 200 {
		t.Fatalf("edit: %d %s", result.Code, result.Body)
	}
	result = performJSONRequest(t, router, "POST", "/api/student/internship-case/submit", nil, token(student))
	if result.Code != 200 {
		t.Fatalf("resubmit: %d %s", result.Code, result.Body)
	}
	assertAPIError(t, performJSONRequest(t, router, "GET", "/api/university/internship-cases?status=READY_TO_START", nil, token(university)), 400, "INVALID_INTERNSHIP_CASE_STATUS")
	for _, status := range []model.InternshipCaseStatus{model.InternshipCaseStatusDraft, model.InternshipCaseStatusRevisionRequested, model.InternshipCaseStatusPendingUniversityReview, model.InternshipCaseStatusPendingCompanyDetails, model.InternshipCaseStatusPendingFinalApproval, model.InternshipCaseStatusActive, model.InternshipCaseStatusPassed, model.InternshipCaseStatusFailed, model.InternshipCaseStatusCancelled} {
		t.Run("eligibility output "+string(status), func(t *testing.T) {
			if err := tx.Model(&model.InternshipCase{}).Where("id = ?", item.ID).Update("status", status).Error; err != nil {
				t.Fatal(err)
			}
			expected := status == model.InternshipCaseStatusDraft || status == model.InternshipCaseStatusRevisionRequested || status == model.InternshipCaseStatusFailed || status == model.InternshipCaseStatusCancelled
			result := performJSONRequest(t, router, "GET", fmt.Sprintf("/api/student/opportunities/%d", opportunity.ID), nil, token(student))
			var view studentOpportunityResponse
			if err := json.Unmarshal(result.Body.Bytes(), &view); err != nil {
				t.Fatal(err)
			}
			if result.Code != 200 || view.CanApply != expected {
				t.Fatalf("detail eligibility for %s: %d %+v", status, result.Code, view)
			}
			result = performJSONRequest(t, router, "GET", "/api/student/opportunities", nil, token(student))
			var catalog []studentOpportunityResponse
			if err := json.Unmarshal(result.Body.Bytes(), &catalog); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, entry := range catalog {
				if entry.ID == opportunity.ID {
					found = true
					if entry.CanApply != expected || entry.ApplyRestrictionCode != view.ApplyRestrictionCode {
						t.Fatalf("catalog eligibility disagrees: %+v %+v", entry, view)
					}
				}
			}
			if result.Code != 200 || !found {
				t.Fatalf("catalog: %d %+v", result.Code, catalog)
			}
		})
	}
	for _, route := range router.Routes() {
		if route.Path == path+"/activate" || route.Path == "/api/university/internship-cases/:id/activate" {
			t.Fatal("manual activation route registered")
		}
	}
	if result := performJSONRequest(t, router, "POST", path+"/activate", nil, token(university)); result.Code != 404 {
		t.Fatalf("activation route: %d", result.Code)
	}
}
