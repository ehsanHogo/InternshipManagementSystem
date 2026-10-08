package handler

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestCaseHistoryAPI(t *testing.T) {
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
	defer sqlDB.Close()
	if err := db.AutoMigrate(&model.Company{}, &model.User{}, &model.Notification{}, &model.InternshipOpportunity{}, &model.ProfessorAssignment{}, &model.File{}, &model.OpportunityApplication{}, &model.InternshipCase{}, &model.StudentInternshipRating{}, &model.InternshipPreference{}, &model.WeeklyReport{}, &model.CompanyEvaluation{}, &model.FinalReport{}); err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	suffix := time.Now().UnixNano()
	createOpenTermFixture(t, tx, suffix)
	company := createOpportunityTestCompany(t, tx, suffix, "ha", false)
	otherCompany := createOpportunityTestCompany(t, tx, suffix, "hb", false)
	unrelatedCompany := createOpportunityTestCompany(t, tx, suffix, "hc", false)
	supervisor := createOpportunityTestUser(t, tx, suffix, "historysupervisor", model.RoleCompanySupervisor, &company.ID)
	nonselected := createOpportunityTestUser(t, tx, suffix, "historynonselected", model.RoleCompanySupervisor, &otherCompany.ID)
	unrelated := createOpportunityTestUser(t, tx, suffix, "historyunrelated", model.RoleCompanySupervisor, &unrelatedCompany.ID)
	colleague := createOpportunityTestUser(t, tx, suffix, "historycolleague", model.RoleCompanySupervisor, &company.ID)
	professor := createOpportunityTestUser(t, tx, suffix, "historyprofessor", model.RoleProfessor, nil)
	otherProfessor := createOpportunityTestUser(t, tx, suffix, "historyotherprofessor", model.RoleProfessor, nil)
	university := createOpportunityTestUser(t, tx, suffix, "historyuniversity", model.RoleUniversitySupervisor, nil)
	student := createOpportunityTestUser(t, tx, suffix, "historystudent", model.RoleStudent, nil)
	otherStudent := createOpportunityTestUser(t, tx, suffix, "historyotherstudent", model.RoleStudent, nil)
	// Current assignment differs from the Case snapshot: it grants no history access.
	if err := tx.Create(&model.ProfessorAssignment{StudentID: student.ID, ProfessorID: otherProfessor.ID, AssignedAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	uploadDir := t.TempDir()
	workflow := service.NewInternshipService(tx)
	handler := NewInternshipHandler(workflow, uploadDir)
	const secret = "case-history-integration"
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api", appmiddleware.RequireAuth(secret))
	students := api.Group("/student", appmiddleware.RequireRole(model.RoleStudent))
	students.GET("/internship-case", handler.GetCurrentCase)
	students.POST("/internship-case", handler.CreateOrGetCase)
	students.PUT("/internship-case", handler.UpdateCase)
	students.POST("/internship-case/submit", handler.SubmitCase)
	students.PUT("/internship-case/preferences/:id", handler.UpdatePreference)
	students.GET("/internship-cases/history", handler.ListStudentHistoricalCases)
	students.GET("/internship-cases/history/:id", handler.GetStudentHistoricalCase)
	handler.RegisterStudentWeeklyReportRoutes(students)
	companies := api.Group("/company", appmiddleware.RequireRole(model.RoleCompanySupervisor))
	companies.GET("/internship-cases", handler.ListCompanyCases)
	companies.GET("/internship-cases/:id", handler.GetCompanyCase)
	companies.POST("/internship-cases/:id/placement-details", handler.SubmitPlacementDetails)
	handler.RegisterCompanyWeeklyReportRoutes(companies)
	professors := api.Group("/professor", appmiddleware.RequireRole(model.RoleProfessor))
	professors.GET("/internship-cases", handler.ListProfessorCases)
	professors.GET("/internship-cases/:id", handler.GetProfessorCase)
	professors.POST("/internship-cases/:id/complete", handler.CompleteProfessorCase)
	handler.RegisterProfessorWeeklyReportRoutes(professors)
	handler.RegisterProfessorFinalReportRoutes(professors)
	universities := api.Group("/university", appmiddleware.RequireRole(model.RoleUniversitySupervisor))
	universities.GET("/internship-cases", handler.ListUniversityCases)
	universities.GET("/internship-cases/:id", handler.GetUniversityCase)
	handler.RegisterUniversityCaseRoutes(universities)
	api.GET("/files/:id/download", handler.DownloadFile)
	request := func(method, path string, user model.User, body any, want int) *httptest.ResponseRecorder {
		t.Helper()
		var token string
		if user.ID != 0 {
			var err error
			token, err = auth.CreateToken(user, secret, time.Hour)
			if err != nil {
				t.Fatal(err)
			}
		}
		rec := performJSONRequest(t, router, method, path, body, token)
		if rec.Code != want {
			t.Fatalf("%s %s as %s = %d want %d: %s", method, path, user.Role, rec.Code, want, rec.Body.String())
		}
		return rec
	}
	create := func(value any) {
		t.Helper()
		if err := tx.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	file := func(owner model.User, label string) model.File {
		t.Helper()
		name := fmt.Sprintf("history-%s-%d.pdf", label, time.Now().UnixNano())
		path := filepath.Join(uploadDir, name)
		if err := os.WriteFile(path, []byte("%PDF-1.4\nhistorical PDF\n%%EOF"), 0600); err != nil {
			t.Fatal(err)
		}
		value := model.File{OriginalName: name, StoredName: name, Path: path, MimeType: "application/pdf", SizeBytes: 32, UploadedBy: owner.ID, UploadedAt: time.Now()}
		create(&value)
		return value
	}
	fixture := func(owner model.User, status model.InternshipCaseStatus) *model.InternshipCase {
		t.Helper()
		now := time.Now()
		item := model.InternshipCase{StudentID: owner.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusPendingUniversityReview, SubmittedAt: &now}
		create(&item)
		for index, principal := range []model.User{supervisor, nonselected} {
			opportunity := model.InternshipOpportunity{CompanyID: *principal.CompanyID, CreatedBy: principal.ID, Title: "historical placement", Description: "history", WorkField: "software", Location: "Tehran", Status: model.OpportunityStatusOpen}
			create(&opportunity)
			resume := file(owner, "resume")
			application := model.OpportunityApplication{StudentID: owner.ID, OpportunityID: opportunity.ID, ResumeFileID: resume.ID, Status: model.ApplicationStatusAccepted, AppliedAt: now}
			create(&application)
			preference := model.InternshipPreference{InternshipCaseID: item.ID, OpportunityApplicationID: application.ID, Priority: index + 1}
			create(&preference)
			if index == 0 && status != model.InternshipCaseStatusCancelled {
				if err := tx.Model(&item).Updates(map[string]any{"selected_preference_id": preference.ID, "company_supervisor_id": supervisor.ID, "status": model.InternshipCaseStatusActive, "activated_at": now, "internship_subject": "historical subject", "start_date": now, "workplace_address": "Tehran", "workplace_phone": "02112345678", "letter_number": "history-letter", "letter_date": now}).Error; err != nil {
					t.Fatal(err)
				}
			}
		}
		if status == model.InternshipCaseStatusCancelled {
			cancelled, err := workflow.CancelUniversityReview(item.ID, "historical cancellation reason")
			if err != nil {
				t.Fatal(err)
			}
			return cancelled
		}
		for week := 1; week <= 8; week++ {
			create(&model.WeeklyReport{InternshipCaseID: item.ID, WeekNumber: week, StartDate: now, EndDate: now, ActivityDescription: "historical activity", SubmittedAt: &now, CompanyReviewStatus: model.WeeklyReviewApproved, ProfessorReviewStatus: model.WeeklyReviewApproved})
		}
		create(&model.CompanyEvaluation{InternshipCaseID: item.ID, CompanySupervisorID: supervisor.ID, AttendanceRating: model.EvaluationRatingGood, ParticipationRating: model.EvaluationRatingGood, LearningRating: model.EvaluationRatingGood, InterestRating: model.EvaluationRatingGood, PersistenceRating: model.EvaluationRatingGood, SuggestionRating: model.EvaluationRatingGood, ResourceUsageRating: model.EvaluationRatingGood, ReportQualityRating: model.EvaluationRatingGood, ProjectPerformanceRating: model.EvaluationRatingGood, SubmittedAt: now})
		finalFile := file(owner, "final")
		create(&model.FinalReport{InternshipCaseID: item.ID, CurrentFileID: finalFile.ID, Status: model.FinalReportApproved, SubmittedAt: now, ReviewedAt: &now})
		result := model.ProfessorFinalResultGood
		if status == model.InternshipCaseStatusFailed {
			result = model.ProfessorFinalResultFailed
		}
		comment := "historical final professor comment"
		finalized, err := workflow.CompleteProfessorCase(professor.ID, item.ID, service.ProfessorCompletionInput{Result: result, Comment: &comment})
		if err != nil {
			t.Fatal(err)
		}
		return finalized
	}
	items := []*model.InternshipCase{
		fixture(student, model.InternshipCaseStatusCancelled),
		fixture(student, model.InternshipCaseStatusFailed),
		fixture(student, model.InternshipCaseStatusPassed),
	}

	t.Run("student current and owned terminal detail", func(t *testing.T) {
		request("GET", "/api/student/internship-case", student, nil, 404)
		var history []internshipCaseResponse
		if err := json.Unmarshal(request("GET", "/api/student/internship-cases/history", student, nil, 200).Body.Bytes(), &history); err != nil {
			t.Fatal(err)
		}
		if len(history) != 3 {
			t.Fatalf("history count = %d", len(history))
		}
		for _, item := range items {
			path := fmt.Sprintf("/api/student/internship-cases/history/%d", item.ID)
			var detail internshipCaseResponse
			if err := json.Unmarshal(request("GET", path, student, nil, 200).Body.Bytes(), &detail); err != nil {
				t.Fatal(err)
			}
			if detail.ID != item.ID || detail.Status != item.Status || len(detail.Preferences) != 2 {
				t.Fatalf("lost history: %+v", detail)
			}
			if item.Status == model.InternshipCaseStatusCancelled {
				if detail.SelectedPreference != nil || detail.FinalResult != nil || detail.FinalReport != nil || detail.CancelledAt == nil || detail.CancellationComment == nil {
					t.Fatalf("incorrect partial cancellation history: %+v", detail)
				}
			} else if len(detail.WeeklyReports) != 8 || detail.FinalReport == nil || detail.CompanyEvaluation == nil || detail.CompletedAt == nil || detail.ProfessorComment == nil || detail.CanSubmitCompanyEvaluation {
				t.Fatalf("lost finalized business data: %+v", detail)
			}
			request("GET", path+"?student_id="+fmt.Sprint(student.ID), otherStudent, nil, 404)
			request("GET", path, supervisor, nil, 403)
		}
		if rec := request("GET", "/api/student/internship-cases/history?student_id="+fmt.Sprint(student.ID), otherStudent, nil, 200); rec.Body.String() != "[]" {
			t.Fatal("another student's history leaked")
		}
		request("GET", "/api/student/internship-cases/history", model.User{}, nil, 401)
		request("GET", "/api/student/internship-cases/history/not-an-id", student, nil, 400)
		assertAPIError(t, request("POST", "/api/student/internship-case", student, nil, 409), 409, "INTERNSHIP_ALREADY_COMPLETED")
	})

	t.Run("selected company history and priority privacy", func(t *testing.T) {
		for _, item := range items[1:] {
			path := fmt.Sprintf("/api/company/internship-cases/%d", item.ID)
			rec := request("GET", path, supervisor, nil, 200)
			var detail map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
				t.Fatal(err)
			}
			selected := detail["selectedPreference"].(map[string]any)
			if _, exposed := selected["priority"]; exposed || len(detail["preferences"].([]any)) != 1 {
				t.Fatal("company history exposed preference priority or other placements")
			}
			for _, denied := range []model.User{nonselected, unrelated, colleague} {
				request("GET", path, denied, nil, 403)
			}
		}
		var history []internshipCaseResponse
		if err := json.Unmarshal(request("GET", "/api/company/internship-cases", supervisor, nil, 200).Body.Bytes(), &history); err != nil || len(history) != 2 {
			t.Fatalf("company history count = %d, err = %v", len(history), err)
		}
		request("GET", fmt.Sprintf("/api/company/internship-cases/%d", items[0].ID), supervisor, nil, 403)
		for _, denied := range []model.User{nonselected, unrelated, colleague} {
			if rec := request("GET", "/api/company/internship-cases", denied, nil, 200); rec.Body.String() != "[]" {
				t.Fatal("non-assigned company supervisor gained history access")
			}
		}
	})

	t.Run("professor snapshot history", func(t *testing.T) {
		for _, item := range items[1:] {
			path := fmt.Sprintf("/api/professor/internship-cases/%d", item.ID)
			var detail professorCaseDetailResponse
			if err := json.Unmarshal(request("GET", path, professor, nil, 200).Body.Bytes(), &detail); err != nil {
				t.Fatal(err)
			}
			if detail.CanProfessorComplete || len(detail.WeeklyReports) != 8 || detail.FinalReport == nil || detail.CompanyEvaluation == nil || detail.CompletedAt == nil {
				t.Fatalf("incorrect professor history: %+v", detail)
			}
			request("GET", path, otherProfessor, nil, 403)
		}
		var history []professorCaseListResponse
		if err := json.Unmarshal(request("GET", "/api/professor/internship-cases", professor, nil, 200).Body.Bytes(), &history); err != nil || len(history) != 2 {
			t.Fatalf("professor history count = %d, err = %v", len(history), err)
		}
		if rec := request("GET", "/api/professor/internship-cases", otherProfessor, nil, 200); rec.Body.String() != "[]" {
			t.Fatal("current assignment exposed historical case")
		}
	})

	t.Run("university terminal visibility and role guards", func(t *testing.T) {
		for _, item := range items {
			path := fmt.Sprintf("/api/university/internship-cases/%d", item.ID)
			request("GET", path, university, nil, 200)
			var history []internshipCaseResponse
			if err := json.Unmarshal(request("GET", "/api/university/internship-cases?status="+string(item.Status), university, nil, 200).Body.Bytes(), &history); err != nil || len(history) != 1 || history[0].ID != item.ID {
				t.Fatalf("university terminal filter: %+v, err = %v", history, err)
			}
			for _, denied := range []model.User{student, supervisor, professor} {
				request("GET", path, denied, nil, 403)
				request("GET", "/api/university/internship-cases", denied, nil, 403)
			}
		}
	})

	t.Run("terminal cases reject mutation and retain protected files", func(t *testing.T) {
		for _, item := range items {
			path := fmt.Sprintf("/api/university/internship-cases/%d", item.ID)
			request("POST", path+"/activate", university, nil, 404)
			request("POST", path+"/final-approve", university, nil, 409)
			request("POST", path+"/request-placement-correction", university, gin.H{"comment": "change"}, 409)
			request("POST", path+"/cancel", university, gin.H{"comment": "change"}, 409)
			request("PUT", "/api/student/internship-case", student, gin.H{"mobile": "09123456789"}, 409)
			request("POST", "/api/student/internship-case/submit", student, nil, 409)
			request("POST", fmt.Sprintf("/api/professor/internship-cases/%d/complete", item.ID), professor, gin.H{"result": "GOOD"}, 409)
			if item.Status == model.InternshipCaseStatusCancelled {
				continue
			}
			request("POST", fmt.Sprintf("/api/company/internship-cases/%d/placement-details", item.ID), supervisor, gin.H{"internshipSubject": "change", "startDate": "2026-10-05", "workplaceAddress": "change", "workplacePhone": "02112345678"}, 409)
			for _, principal := range []model.User{supervisor, professor} {
				rolePath := "company"
				if principal.Role == model.RoleProfessor {
					rolePath = "professor"
				}
				request("POST", fmt.Sprintf("/api/%s/internship-cases/%d/weekly-reports/%d/approve", rolePath, item.ID, item.WeeklyReports[0].ID), principal, nil, 400)
			}
			request("POST", fmt.Sprintf("/api/professor/internship-cases/%d/final-report/request-revision", item.ID), professor, gin.H{"comment": "change"}, 409)
			finalPath := fmt.Sprintf("/api/files/%d/download", item.FinalReport.CurrentFileID)
			for _, allowed := range []model.User{student, professor, supervisor, university} {
				request("GET", finalPath, allowed, nil, 200)
			}
			for _, denied := range []model.User{otherStudent, otherProfessor, nonselected, unrelated} {
				request("GET", finalPath, denied, nil, 403)
			}
			request("GET", finalPath, model.User{}, nil, 401)
		}
		resumePath := fmt.Sprintf("/api/files/%d/download", items[0].Preferences[0].OpportunityApplication.ResumeFileID)
		request("GET", resumePath, student, nil, 200)
		request("GET", resumePath, supervisor, nil, 200)
		request("GET", resumePath, professor, nil, 403)
		request("GET", resumePath, university, nil, 403)
	})

	t.Run("failed and cancelled history stay readable after a new draft", func(t *testing.T) {
		for _, status := range []model.InternshipCaseStatus{model.InternshipCaseStatusFailed, model.InternshipCaseStatusCancelled} {
			owner := createOpportunityTestUser(t, tx, suffix, "historyretry"+string(status), model.RoleStudent, nil)
			create(&model.ProfessorAssignment{StudentID: owner.ID, ProfessorID: otherProfessor.ID, AssignedAt: time.Now()})
			old := fixture(owner, status)
			request("PUT", "/api/student/internship-case", owner, gin.H{"mobile": "09123456789"}, 409)
			request("POST", "/api/student/internship-case/submit", owner, nil, 409)
			request("GET", "/api/student/internship-case", owner, nil, 404)
			var draft internshipCaseResponse
			if err := json.Unmarshal(request("POST", "/api/student/internship-case", owner, nil, 201).Body.Bytes(), &draft); err != nil {
				t.Fatal(err)
			}
			if draft.ID == old.ID || draft.Status != model.InternshipCaseStatusDraft || draft.Professor.ID != otherProfessor.ID {
				t.Fatalf("incorrect retry: %+v", draft)
			}
			// A historical record with a later timestamp must never become current.
			if err := tx.Model(old).Update("created_at", time.Now().Add(time.Hour)).Error; err != nil {
				t.Fatal(err)
			}
			var current internshipCaseResponse
			if err := json.Unmarshal(request("GET", "/api/student/internship-case", owner, nil, 200).Body.Bytes(), &current); err != nil || current.ID != draft.ID {
				t.Fatalf("historical case treated as current: %+v, err = %v", current, err)
			}
			request("GET", fmt.Sprintf("/api/student/internship-cases/history/%d", old.ID), owner, nil, 200)
			assertAPIError(t, request("PUT", fmt.Sprintf("/api/student/internship-case/preferences/%d", old.Preferences[0].ID), owner,
				gin.H{"priority": 1, "opportunityApplicationId": old.Preferences[0].OpportunityApplicationID}, 404), 404, "PREFERENCE_NOT_FOUND")
			request("GET", fmt.Sprintf("/api/student/internship-cases/history/%d", draft.ID), owner, nil, 404)
			if _, err := workflow.ReplacePreferences(owner.ID, []uint{old.Preferences[0].OpportunityApplicationID}); err != nil {
				t.Fatalf("historical accepted application could not be reused: %v", err)
			}
		}
	})
}
