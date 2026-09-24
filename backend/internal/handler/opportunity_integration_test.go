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

func TestOpportunityLifecycleAuthorizationAndStudentCatalog(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect to integration database: %v", err)
	}
	if err := db.AutoMigrate(&model.Company{}, &model.User{}, &model.InternshipOpportunity{}); err != nil {
		t.Fatalf("migrate integration database: %v", err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin test transaction: %v", tx.Error)
	}
	defer tx.Rollback()

	suffix := time.Now().UnixNano()
	companyA := createOpportunityTestCompany(t, tx, suffix, "a", false)
	companyB := createOpportunityTestCompany(t, tx, suffix, "b", true)
	supervisorA := createOpportunityTestUser(t, tx, suffix, "company-a", model.RoleCompanySupervisor, &companyA.ID)
	supervisorB := createOpportunityTestUser(t, tx, suffix, "company-b", model.RoleCompanySupervisor, &companyB.ID)
	student := createOpportunityTestUser(t, tx, suffix, "student", model.RoleStudent, nil)
	professor := createOpportunityTestUser(t, tx, suffix, "professor", model.RoleProfessor, nil)
	university := createOpportunityTestUser(t, tx, suffix, "university", model.RoleUniversitySupervisor, nil)
	orphanSupervisor := createOpportunityTestUser(t, tx, suffix, "company-orphan", model.RoleCompanySupervisor, nil)

	const jwtSecret = "opportunity-integration-secret"
	router := opportunityTestRouter(tx, jwtSecret)
	tokens := map[model.Role]string{}
	for _, user := range []model.User{supervisorA, supervisorB, student, professor, university} {
		token, tokenErr := auth.CreateToken(user, jwtSecret, time.Hour)
		if tokenErr != nil {
			t.Fatalf("create %s token: %v", user.Role, tokenErr)
		}
		tokens[user.Role] = token
	}
	tokenA, _ := auth.CreateToken(supervisorA, jwtSecret, time.Hour)
	tokenB, _ := auth.CreateToken(supervisorB, jwtSecret, time.Hour)
	orphanToken, _ := auth.CreateToken(orphanSupervisor, jwtSecret, time.Hour)

	createPayload := map[string]any{
		"title": "  توسعه‌دهنده نرم‌افزار  ", "description": "  همکاری در توسعه محصول  ",
		"workField": "  مهندسی نرم‌افزار  ", "location": "  تهران  ",
		"companyId": companyB.ID, "createdBy": supervisorB.ID, "status": model.OpportunityStatusClosed,
	}
	createdResponse := performJSONRequest(t, router, http.MethodPost, "/api/company/opportunities", createPayload, tokenA)
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createdResponse.Code, createdResponse.Body.String())
	}
	var created companyOpportunityResponse
	if err := json.Unmarshal(createdResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created opportunity: %v", err)
	}
	var persisted model.InternshipOpportunity
	if err := tx.First(&persisted, created.ID).Error; err != nil {
		t.Fatalf("load created opportunity: %v", err)
	}
	if persisted.CompanyID != companyA.ID || persisted.CreatedBy != supervisorA.ID || persisted.Status != model.OpportunityStatusOpen {
		t.Fatalf("server-controlled ownership/status invalid: %+v", persisted)
	}
	if persisted.Title != "توسعه‌دهنده نرم‌افزار" || companyA.IsApproved {
		t.Fatalf("unapproved company create or trimming failed: company=%+v opportunity=%+v", companyA, persisted)
	}
	assertAPIError(t,
		performJSONRequest(t, router, http.MethodPost, "/api/company/opportunities", createPayload, orphanToken),
		http.StatusBadRequest, "COMPANY_REQUIRED",
	)

	t.Run("blank required fields are rejected", func(t *testing.T) {
		fields := []string{"title", "description", "workField", "location"}
		for _, field := range fields {
			payload := map[string]any{"title": "عنوان", "description": "توضیحات", "workField": "فنی", "location": "تهران"}
			payload[field] = "   "
			response := performJSONRequest(t, router, http.MethodPost, "/api/company/opportunities", payload, tokenA)
			assertAPIError(t, response, http.StatusBadRequest, "INVALID_OPPORTUNITY")
		}
	})

	otherPayload := map[string]string{"title": "فرصت شرکت ب", "description": "توضیحات ب", "workField": "داده", "location": "شیراز"}
	otherResponse := performJSONRequest(t, router, http.MethodPost, "/api/company/opportunities", otherPayload, tokenB)
	if otherResponse.Code != http.StatusCreated {
		t.Fatalf("create company B opportunity status = %d, body = %s", otherResponse.Code, otherResponse.Body.String())
	}
	var other companyOpportunityResponse
	if err := json.Unmarshal(otherResponse.Body.Bytes(), &other); err != nil {
		t.Fatalf("decode company B opportunity: %v", err)
	}

	t.Run("cross-company access is hidden", func(t *testing.T) {
		paths := []struct {
			method, path string
			body         any
		}{
			{http.MethodGet, fmt.Sprintf("/api/company/opportunities/%d", other.ID), nil},
			{http.MethodPut, fmt.Sprintf("/api/company/opportunities/%d", other.ID), createPayload},
			{http.MethodPost, fmt.Sprintf("/api/company/opportunities/%d/close", other.ID), map[string]any{}},
		}
		for _, request := range paths {
			response := performJSONRequest(t, router, request.method, request.path, request.body, tokenA)
			assertAPIError(t, response, http.StatusNotFound, "OPPORTUNITY_NOT_FOUND")
		}
	})

	editPayload := map[string]any{
		"title": "  عنوان ویرایش‌شده  ", "description": "شرح ویرایش‌شده", "workField": "محصول", "location": "اصفهان",
		"companyId": companyB.ID, "createdBy": supervisorB.ID, "status": model.OpportunityStatusClosed,
	}
	editResponse := performJSONRequest(t, router, http.MethodPut, fmt.Sprintf("/api/company/opportunities/%d", created.ID), editPayload, tokenA)
	if editResponse.Code != http.StatusOK {
		t.Fatalf("edit status = %d, body = %s", editResponse.Code, editResponse.Body.String())
	}
	if err := tx.First(&persisted, created.ID).Error; err != nil {
		t.Fatalf("reload edited opportunity: %v", err)
	}
	if persisted.Title != "عنوان ویرایش‌شده" || persisted.Description != "شرح ویرایش‌شده" || persisted.WorkField != "محصول" || persisted.Location != "اصفهان" {
		t.Fatalf("editable fields were not updated: %+v", persisted)
	}
	if persisted.CompanyID != companyA.ID || persisted.CreatedBy != supervisorA.ID || persisted.Status != model.OpportunityStatusOpen {
		t.Fatalf("protected fields changed through edit: %+v", persisted)
	}

	studentList := performJSONRequest(t, router, http.MethodGet, "/api/student/opportunities", nil, tokens[model.RoleStudent])
	if studentList.Code != http.StatusOK {
		t.Fatalf("student list status = %d, body = %s", studentList.Code, studentList.Body.String())
	}
	var catalog []studentOpportunityResponse
	if err := json.Unmarshal(studentList.Body.Bytes(), &catalog); err != nil {
		t.Fatalf("decode student catalog: %v", err)
	}
	assertCatalogContains(t, catalog, created.ID, companyA.ID, false)
	assertCatalogContains(t, catalog, other.ID, companyB.ID, true)
	studentDetail := performJSONRequest(t, router, http.MethodGet, fmt.Sprintf("/api/student/opportunities/%d", created.ID), nil, tokens[model.RoleStudent])
	if studentDetail.Code != http.StatusOK {
		t.Fatalf("student detail status = %d, body = %s", studentDetail.Code, studentDetail.Body.String())
	}

	closeResponse := performJSONRequest(t, router, http.MethodPost, fmt.Sprintf("/api/company/opportunities/%d/close", created.ID), map[string]any{}, tokenA)
	if closeResponse.Code != http.StatusOK {
		t.Fatalf("close status = %d, body = %s", closeResponse.Code, closeResponse.Body.String())
	}
	if err := tx.First(&persisted, created.ID).Error; err != nil || persisted.Status != model.OpportunityStatusClosed {
		t.Fatalf("closed record not retained correctly: opportunity=%+v err=%v", persisted, err)
	}
	assertAPIError(t,
		performJSONRequest(t, router, http.MethodPost, fmt.Sprintf("/api/company/opportunities/%d/close", created.ID), map[string]any{}, tokenA),
		http.StatusConflict, "OPPORTUNITY_ALREADY_CLOSED",
	)
	studentList = performJSONRequest(t, router, http.MethodGet, "/api/student/opportunities", nil, tokens[model.RoleStudent])
	if err := json.Unmarshal(studentList.Body.Bytes(), &catalog); err != nil {
		t.Fatalf("decode catalog after close: %v", err)
	}
	for _, item := range catalog {
		if item.ID == created.ID {
			t.Fatalf("closed opportunity %d remained in student catalog", created.ID)
		}
	}
	assertAPIError(t,
		performJSONRequest(t, router, http.MethodGet, fmt.Sprintf("/api/student/opportunities/%d", created.ID), nil, tokens[model.RoleStudent]),
		http.StatusNotFound, "OPPORTUNITY_NOT_FOUND",
	)
	companyHistory := performJSONRequest(t, router, http.MethodGet, "/api/company/opportunities", nil, tokenA)
	if companyHistory.Code != http.StatusOK {
		t.Fatalf("company history status = %d, body = %s", companyHistory.Code, companyHistory.Body.String())
	}
	var history []companyOpportunityResponse
	if err := json.Unmarshal(companyHistory.Body.Bytes(), &history); err != nil {
		t.Fatalf("decode company history: %v", err)
	}
	if len(history) != 1 || history[0].ID != created.ID || history[0].Status != model.OpportunityStatusClosed {
		t.Fatalf("company history did not retain only its closed opportunity: %+v", history)
	}

	t.Run("non-company roles cannot mutate opportunities", func(t *testing.T) {
		for _, role := range []model.Role{model.RoleStudent, model.RoleProfessor, model.RoleUniversitySupervisor} {
			requests := []struct {
				method, path string
				body         any
			}{
				{http.MethodPost, "/api/company/opportunities", createPayload},
				{http.MethodPut, fmt.Sprintf("/api/company/opportunities/%d", other.ID), createPayload},
				{http.MethodPost, fmt.Sprintf("/api/company/opportunities/%d/close", other.ID), map[string]any{}},
			}
			for _, request := range requests {
				response := performJSONRequest(t, router, request.method, request.path, request.body, tokens[role])
				if response.Code != http.StatusForbidden {
					t.Fatalf("%s %s as %s status = %d, body = %s", request.method, request.path, role, response.Code, response.Body.String())
				}
			}
		}
	})
}

func opportunityTestRouter(db *gorm.DB, jwtSecret string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	opportunities := NewOpportunityHandler(service.NewOpportunityService(db))
	router := gin.New()
	authenticated := router.Group("/api")
	authenticated.Use(appmiddleware.RequireAuth(jwtSecret))
	company := authenticated.Group("/company")
	company.Use(appmiddleware.RequireRole(model.RoleCompanySupervisor))
	company.POST("/opportunities", opportunities.Create)
	company.GET("/opportunities", opportunities.ListCompany)
	company.GET("/opportunities/:id", opportunities.GetCompany)
	company.PUT("/opportunities/:id", opportunities.Update)
	company.POST("/opportunities/:id/close", opportunities.Close)
	student := authenticated.Group("/student")
	student.Use(appmiddleware.RequireRole(model.RoleStudent))
	student.GET("/opportunities", opportunities.ListStudent)
	student.GET("/opportunities/:id", opportunities.GetStudent)
	return router
}

func createOpportunityTestCompany(t *testing.T, db *gorm.DB, suffix int64, label string, approved bool) model.Company {
	t.Helper()
	company := model.Company{
		Name: fmt.Sprintf("opportunity-company-%d-%s", suffix, label), NationalID: fmt.Sprintf("opp-national-%d-%s", suffix, label),
		EconomicCode: fmt.Sprintf("opp-economic-%d-%s", suffix, label), IsApproved: approved,
	}
	if err := db.Create(&company).Error; err != nil {
		t.Fatalf("create opportunity test company: %v", err)
	}
	return company
}

func createOpportunityTestUser(t *testing.T, db *gorm.DB, suffix int64, label string, role model.Role, companyID *uint) model.User {
	t.Helper()
	user := model.User{
		FullName: "کاربر آزمون", Email: fmt.Sprintf("opportunity-%d-%s@example.test", suffix, label),
		PasswordHash: "unused-in-route-test", Role: role, CompanyID: companyID,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create opportunity test user: %v", err)
	}
	return user
}

func assertCatalogContains(t *testing.T, catalog []studentOpportunityResponse, opportunityID, companyID uint, approved bool) {
	t.Helper()
	for _, item := range catalog {
		if item.ID == opportunityID {
			if item.Company.ID != companyID || item.Company.IsApproved != approved {
				t.Fatalf("catalog company mismatch for opportunity %d: %+v", opportunityID, item.Company)
			}
			return
		}
	}
	t.Fatalf("opportunity %d is missing from student catalog", opportunityID)
}
