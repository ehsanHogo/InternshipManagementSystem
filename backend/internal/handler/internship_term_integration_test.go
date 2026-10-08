package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
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

func createOpenTermFixture(t *testing.T, db *gorm.DB, suffix int64) {
	t.Helper()
	university := createOpportunityTestUser(t, db, suffix, "termfixtureuniversity", model.RoleUniversitySupervisor, nil)
	if _, err := service.NewInternshipTermService(db).Create(university.ID, 1405, model.InternshipTermTypeSummer); err != nil {
		t.Fatal(err)
	}
}

func TestInternshipTermAPI(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Company{}, &model.User{}, &model.Notification{}, &model.ProfessorAssignment{}, &model.InternshipTerm{}, &model.InternshipOpportunity{}, &model.File{}, &model.OpportunityApplication{}, &model.InternshipCase{}, &model.StudentInternshipRating{}, &model.InternshipPreference{}, &model.WeeklyReport{}, &model.CompanyEvaluation{}, &model.FinalReport{}); err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	suffix := time.Now().UnixNano()
	university := createOpportunityTestUser(t, tx, suffix, "termuniversity", model.RoleUniversitySupervisor, nil)
	professor := createOpportunityTestUser(t, tx, suffix, "termprofessor", model.RoleProfessor, nil)
	student := createOpportunityTestUser(t, tx, suffix, "termstudent", model.RoleStudent, nil)
	if err := tx.Create(&model.ProfessorAssignment{StudentID: student.ID, ProfessorID: professor.ID, AssignedAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	const secret = "term-api-integration"
	api := router.Group("/api", appmiddleware.RequireAuth(secret))
	termHandler := NewInternshipTermHandler(service.NewInternshipTermService(tx))
	caseHandler := NewInternshipHandler(service.NewInternshipService(tx))
	universityGroup := api.Group("/university", appmiddleware.RequireRole(model.RoleUniversitySupervisor))
	termHandler.RegisterUniversityRoutes(universityGroup)
	universityGroup.GET("/internship-cases", caseHandler.ListUniversityCases)
	students := api.Group("/student", appmiddleware.RequireRole(model.RoleStudent))
	students.GET("/internship-term", termHandler.Current)
	students.POST("/internship-case", caseHandler.CreateOrGetCase)
	students.PUT("/internship-case", caseHandler.UpdateCase)
	students.GET("/internship-cases/history/:id", caseHandler.GetStudentHistoricalCase)
	request := func(method, path string, actor model.User, body any, status int) *httptest.ResponseRecorder {
		t.Helper()
		var data []byte
		if body != nil {
			data, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		if actor.ID != 0 {
			token, err := auth.CreateToken(actor, secret, time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		if recorder.Code != status {
			t.Fatalf("%s %s: status %d want %d: %s", method, path, recorder.Code, status, recorder.Body.String())
		}
		return recorder
	}
	request("GET", "/api/university/internship-terms", model.User{}, nil, 401)
	for _, role := range []model.Role{model.RoleStudent, model.RoleProfessor, model.RoleCompanySupervisor, model.RoleAdmin} {
		actor := createOpportunityTestUser(t, tx, suffix, "termdenied"+string(role), role, nil)
		for _, endpoint := range []struct{ method, path string }{{"GET", "/api/university/internship-terms"}, {"POST", "/api/university/internship-terms"}, {"GET", "/api/university/internship-terms/1"}, {"POST", "/api/university/internship-terms/1/close"}} {
			request(endpoint.method, endpoint.path, actor, nil, 403)
		}
	}
	if response := request("GET", "/api/student/internship-term", student, nil, 200); response.Body.String() != "null" {
		t.Fatalf("no open term: %s", response.Body.String())
	}
	assertAPIError(t, request("POST", "/api/student/internship-case", student, nil, 409), 409, "NO_OPEN_INTERNSHIP_TERM")
	request("POST", "/api/university/internship-terms", university, gin.H{"academicYear": 1405, "termType": "INVALID"}, 400)
	var term model.InternshipTerm
	if err := json.Unmarshal(request("POST", "/api/university/internship-terms", university, gin.H{"academicYear": 1405, "termType": "SUMMER"}, 201).Body.Bytes(), &term); err != nil {
		t.Fatal(err)
	}
	request("POST", "/api/university/internship-terms", university, gin.H{"academicYear": 1405, "termType": "SUMMER"}, 409)
	request("POST", "/api/university/internship-terms", university, gin.H{"academicYear": 1406, "termType": "FIRST"}, 409)
	request("GET", "/api/university/internship-terms", university, nil, 200)
	request("GET", "/api/university/internship-terms/0", university, nil, 400)
	request("GET", "/api/university/internship-terms/99999999", university, nil, 404)
	// Student-supplied IDs are rejected and cannot influence assignment.
	request("POST", "/api/student/internship-case", student, gin.H{"termId": term.ID + 100}, 400)
	request("POST", "/api/student/internship-case", student, gin.H{"termId": nil}, 400)
	var item internshipCaseResponse
	if err := json.Unmarshal(request("POST", "/api/student/internship-case", student, gin.H{}, 201).Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.TermID == nil || *item.TermID != term.ID || item.Term == nil || item.Term.AcademicYear != 1405 {
		t.Fatalf("term response: %+v", item)
	}
	request("PUT", "/api/student/internship-case", student, gin.H{"termId": term.ID + 100, "mobile": "09123456789"}, 400)
	var detail struct {
		Term  model.InternshipTerm     `json:"term"`
		Cases []internshipCaseResponse `json:"cases"`
	}
	path := fmt.Sprintf("/api/university/internship-terms/%d", term.ID)
	response := request("GET", path, university, nil, 200)
	if err := json.Unmarshal(response.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Cases) != 1 || detail.Cases[0].ID != item.ID || detail.Cases[0].Status != model.InternshipCaseStatusDraft {
		t.Fatalf("draft visibility: %+v", detail)
	}
	if bytes.Contains(response.Body.Bytes(), []byte("PasswordHash")) || bytes.Contains(response.Body.Bytes(), []byte("passwordHash")) {
		t.Fatal("term response leaked user password")
	}
	if response := request("GET", "/api/university/internship-cases", university, nil, 200); response.Body.String() != "[]" {
		t.Fatalf("normal university list exposed draft: %s", response.Body.String())
	}
	request("POST", path+"/close", university, nil, 200)
	request("POST", path+"/close", university, nil, 409)
	var historical internshipCaseResponse
	if err := json.Unmarshal(request("GET", fmt.Sprintf("/api/student/internship-cases/history/%d", item.ID), student, nil, 200).Body.Bytes(), &historical); err != nil {
		t.Fatal(err)
	}
	if historical.Status != model.InternshipCaseStatusCancelled || historical.CancellationComment == nil || historical.Term == nil || historical.Term.ClosedBy == nil || *historical.Term.ClosedBy != university.ID {
		t.Fatalf("cancel audit: %+v", historical)
	}
	assertAPIError(t, request("POST", "/api/student/internship-case", student, nil, 409), 409, "NO_OPEN_INTERNSHIP_TERM")
	request("POST", "/api/university/internship-terms", university, gin.H{"academicYear": 1406, "termType": "FIRST"}, 201)
	var retry internshipCaseResponse
	if err := json.Unmarshal(request("POST", "/api/student/internship-case", student, nil, 201).Body.Bytes(), &retry); err != nil {
		t.Fatal(err)
	}
	if retry.ID == item.ID || retry.TermID == nil || *retry.TermID == term.ID {
		t.Fatalf("new term retry: %+v", retry)
	}
}
