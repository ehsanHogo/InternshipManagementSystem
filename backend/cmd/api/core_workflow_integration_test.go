package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

// Exercise the production router, including role and registration middleware.
func TestCoreWorkflowStabilizationProductionRoutes(t *testing.T) {
	db := registrationTestDB(t)
	router := registrationRouter(t, db)
	suffix := time.Now().UnixNano()
	makeUser := func(t *testing.T, label string, role model.Role, companyID *uint) model.User {
		t.Helper()
		u := model.User{FullName: label, Email: fmt.Sprintf("m19-%s-%d@example.test", label, suffix), PasswordHash: "fixture", Role: role, CompanyID: companyID}
		if err := db.Create(&u).Error; err != nil {
			t.Fatal(err)
		}
		return u
	}
	company := model.Company{Name: fmt.Sprintf("m19-%d", suffix), NationalID: fmt.Sprint(suffix), EconomicCode: fmt.Sprintf("e-%d", suffix), RegistrationStatus: model.CompanyRegistrationStatusApproved, IsApproved: true}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	otherCompany := model.Company{Name: fmt.Sprintf("m19-other-%d", suffix), NationalID: fmt.Sprintf("other-%d", suffix), EconomicCode: fmt.Sprintf("other-e-%d", suffix), RegistrationStatus: model.CompanyRegistrationStatusApproved}
	if err := db.Create(&otherCompany).Error; err != nil {
		t.Fatal(err)
	}
	supervisor := makeUser(t, "supervisor", model.RoleCompanySupervisor, &company.ID)
	sameCompany := makeUser(t, "same-company", model.RoleCompanySupervisor, &company.ID)
	wrongSupervisor := makeUser(t, "wrong-company", model.RoleCompanySupervisor, &otherCompany.ID)
	professor := makeUser(t, "professor", model.RoleProfessor, nil)
	wrongProfessor := makeUser(t, "wrong-professor", model.RoleProfessor, nil)
	wrongStudent := makeUser(t, "wrong-student", model.RoleStudent, nil)
	university := registrationDemoUser(t, db, "university")
	admin := registrationDemoUser(t, db, "admin")
	request := func(t *testing.T, method, path string, user model.User, body any, want int) string {
		t.Helper()
		return registrationRequest(t, router, method, path, registrationToken(t, user), body, want).Body.String()
	}
	must := func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	counter := 0
	fixture := func(t *testing.T, status model.InternshipCaseStatus) (model.User, model.InternshipCase, model.WeeklyReport, model.FinalReport) {
		t.Helper()
		counter++
		student := makeUser(t, fmt.Sprintf("student-%d", counter), model.RoleStudent, nil)
		item := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, Status: status, CompanySupervisorID: &supervisor.ID}
		must(t, db.Create(&item).Error)
		opportunity := model.InternshipOpportunity{CompanyID: company.ID, CreatedBy: supervisor.ID, Title: "placement", Description: "description", WorkField: "software", Location: "Tehran", Status: model.OpportunityStatusOpen}
		must(t, db.Create(&opportunity).Error)
		file := model.File{OriginalName: "resume.pdf", StoredName: fmt.Sprintf("resume-%d.pdf", item.ID), Path: filepath.Join(t.TempDir(), "resume.pdf"), MimeType: "application/pdf", SizeBytes: 20, UploadedBy: student.ID, UploadedAt: time.Now()}
		must(t, os.WriteFile(file.Path, []byte("%PDF-1.4 fixture"), 0600))
		must(t, db.Create(&file).Error)
		application := model.OpportunityApplication{StudentID: student.ID, OpportunityID: opportunity.ID, ResumeFileID: file.ID, Status: model.ApplicationStatusAccepted, AppliedAt: time.Now()}
		must(t, db.Create(&application).Error)
		pref := model.InternshipPreference{InternshipCaseID: item.ID, OpportunityApplicationID: application.ID, Priority: 1}
		must(t, db.Create(&pref).Error)
		must(t, db.Model(&item).Update("selected_preference_id", pref.ID).Error)
		week := model.WeeklyReport{InternshipCaseID: item.ID, WeekNumber: 1, StartDate: time.Now(), EndDate: time.Now(), ActivityDescription: "history marker", CompanyReviewStatus: model.WeeklyReviewPending, ProfessorReviewStatus: model.WeeklyReviewPending}
		must(t, db.Create(&week).Error)
		file.ID = 0
		file.StoredName = fmt.Sprintf("final-%d.pdf", item.ID)
		must(t, db.Create(&file).Error)
		report := model.FinalReport{InternshipCaseID: item.ID, CurrentFileID: file.ID, Status: model.FinalReportSubmitted, SubmittedAt: time.Now()}
		must(t, db.Create(&report).Error)
		return student, item, week, report
	}

	t.Run("professor selected company list detail history and ownership", func(t *testing.T) {
		for _, status := range []model.InternshipCaseStatus{model.InternshipCaseStatusActive, model.InternshipCaseStatusPassed, model.InternshipCaseStatusFailed} {
			_, item, _, _ := fixture(t, status)
			list := registrationDecode[[]struct {
				CaseID  uint
				Company string
			}](t, registrationRequest(t, router, "GET", "/api/professor/internship-cases", registrationToken(t, professor), nil, 200))
			found := false
			for _, view := range list {
				if view.CaseID == item.ID {
					found = true
					if view.Company != company.Name {
						t.Fatalf("list company=%q", view.Company)
					}
				}
			}
			if !found {
				t.Fatal("case missing from professor list")
			}
			path := fmt.Sprintf("/api/professor/internship-cases/%d", item.ID)
			detail := registrationDecode[struct{ Internship struct{ Company string } }](t, registrationRequest(t, router, "GET", path, registrationToken(t, professor), nil, 200))
			if detail.Internship.Company != company.Name {
				t.Fatalf("detail company=%q", detail.Internship.Company)
			}
			request(t, "GET", path, wrongProfessor, nil, 403)
			wrongList := request(t, "GET", "/api/professor/internship-cases", wrongProfessor, nil, 200)
			if strings.Contains(wrongList, company.Name) {
				t.Fatal("unassigned professor inferred selected company")
			}
			must(t, db.Model(&item).Update("selected_preference_id", nil).Error)
			detail = registrationDecode[struct{ Internship struct{ Company string } }](t, registrationRequest(t, router, "GET", path, registrationToken(t, professor), nil, 200))
			if detail.Internship.Company != "" {
				t.Fatal("missing placement must have empty company")
			}
		}
	})

	t.Run("current reporting never falls back to terminal history", func(t *testing.T) {
		for _, oldStatus := range []model.InternshipCaseStatus{model.InternshipCaseStatusPassed, model.InternshipCaseStatusFailed, model.InternshipCaseStatusCancelled} {
			student, old, week, report := fixture(t, oldStatus)
			historyPath := fmt.Sprintf("/api/student/internship-cases/history/%d", old.ID)
			assertHistory := func() {
				t.Helper()
				view := registrationDecode[struct {
					WeeklyReports []model.WeeklyReport
					FinalReport   *model.FinalReport
				}](t, registrationRequest(t, router, "GET", historyPath, registrationToken(t, student), nil, 200))
				if len(view.WeeklyReports) != 1 || view.WeeklyReports[0].ID != week.ID || view.FinalReport == nil || view.FinalReport.ID != report.ID {
					t.Fatal("explicit history lost reports")
				}
			}
			assertHistory()
			request(t, "GET", "/api/student/internship-case/weekly-reports", student, nil, 400)
			request(t, "GET", fmt.Sprintf("/api/student/internship-case/weekly-reports/%d", week.ID), student, nil, 400)
			request(t, "GET", "/api/student/internship-case/final-report", student, nil, 404)
			if oldStatus == model.InternshipCaseStatusPassed {
				continue
			} // PASSED cannot retry.
			current := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusDraft}
			must(t, db.Create(&current).Error)
			for _, status := range []model.InternshipCaseStatus{model.InternshipCaseStatusDraft, model.InternshipCaseStatusRevisionRequested} {
				must(t, db.Model(&current).Update("status", status).Error)
				request(t, "GET", "/api/student/internship-case/weekly-reports", student, nil, 400)
				request(t, "GET", fmt.Sprintf("/api/student/internship-case/weekly-reports/%d", week.ID), student, nil, 400)
				if body := request(t, "GET", "/api/student/internship-case/final-report", student, nil, 200); body != "null" {
					t.Fatalf("historical final report leaked: %s", body)
				}
				assertHistory()
			}
			must(t, db.Model(&current).Update("status", model.InternshipCaseStatusActive).Error)
			currentWeek := model.WeeklyReport{InternshipCaseID: current.ID, WeekNumber: 1, StartDate: time.Now(), EndDate: time.Now(), ActivityDescription: "current marker"}
			must(t, db.Create(&currentWeek).Error)
			reports := registrationDecode[[]model.WeeklyReport](t, registrationRequest(t, router, "GET", "/api/student/internship-case/weekly-reports", registrationToken(t, student), nil, 200))
			if len(reports) != 1 || reports[0].ID != currentWeek.ID {
				t.Fatal("current reporting selected historical case")
			}
			request(t, "GET", fmt.Sprintf("/api/student/internship-case/weekly-reports/%d", currentWeek.ID), student, nil, 200)
			request(t, "GET", fmt.Sprintf("/api/student/internship-case/weekly-reports/%d", week.ID), student, nil, 404)
			request(t, "PUT", fmt.Sprintf("/api/student/internship-case/weekly-reports/%d", week.ID), student, map[string]any{"weekNumber": 1, "startDate": "2026-01-01", "endDate": "2026-01-02", "activityDescription": "attempt"}, 404)
			assertHistory()
		}
	})

	t.Run("student evaluation is an outcome only", func(t *testing.T) {
		student, item, _, _ := fixture(t, model.InternshipCaseStatusActive)
		evaluation := model.CompanyEvaluation{InternshipCaseID: item.ID, CompanySupervisorID: supervisor.ID, AttendanceRating: model.EvaluationRatingGood, ParticipationRating: model.EvaluationRatingGood, LearningRating: model.EvaluationRatingGood, InterestRating: model.EvaluationRatingGood, PersistenceRating: model.EvaluationRatingGood, SuggestionRating: model.EvaluationRatingGood, ResourceUsageRating: model.EvaluationRatingGood, ReportQualityRating: model.EvaluationRatingGood, ProjectPerformanceRating: model.EvaluationRatingGood, SubmittedAt: time.Now()}
		must(t, db.Create(&evaluation).Error)
		if strings.Contains(request(t, "GET", "/api/student/internship-case", student, nil, 200), "companyEvaluation") {
			t.Fatal("active student evaluation leaked")
		}
		for _, role := range []struct {
			user model.User
			path string
		}{{supervisor, "company"}, {professor, "professor"}, {university, "university"}} {
			if !strings.Contains(request(t, "GET", fmt.Sprintf("/api/%s/internship-cases/%d", role.path, item.ID), role.user, nil, 200), "companyEvaluation") {
				t.Fatal("reviewer evaluation lost")
			}
		}
		for _, status := range []model.InternshipCaseStatus{model.InternshipCaseStatusPassed, model.InternshipCaseStatusFailed, model.InternshipCaseStatusCancelled} {
			must(t, db.Model(&item).Update("status", status).Error)
			body := request(t, "GET", fmt.Sprintf("/api/student/internship-cases/history/%d", item.ID), student, nil, 200)
			if strings.Contains(body, "companyEvaluation") != (status != model.InternshipCaseStatusCancelled) {
				t.Fatalf("incorrect student visibility for %s", status)
			}
		}
	})

	t.Run("file authorization equals case ownership and membership", func(t *testing.T) {
		student, item, _, report := fixture(t, model.InternshipCaseStatusActive)
		path := fmt.Sprintf("/api/files/%d/download", report.CurrentFileID)
		for _, user := range []model.User{student, professor, university, supervisor} {
			request(t, "GET", path, user, nil, 200)
		}
		for _, user := range []model.User{wrongStudent, wrongProfessor, admin, sameCompany, wrongSupervisor} {
			request(t, "GET", path, user, nil, 403)
		}
		var pref model.InternshipPreference
		must(t, db.Where("internship_case_id = ?", item.ID).First(&pref).Error)
		for _, status := range []model.InternshipCaseStatus{model.InternshipCaseStatusActive, model.InternshipCaseStatusPassed, model.InternshipCaseStatusFailed, model.InternshipCaseStatusCancelled} {
			must(t, db.Model(&item).Updates(map[string]any{"status": status, "activated_at": time.Now(), "cancellation_comment": service.TermClosureCancellationComment}).Error)
			// Cancellation operational professor access requires a real term.
			if status != model.InternshipCaseStatusCancelled {
				request(t, "GET", path, professor, nil, 200)
			}
			request(t, "GET", path, student, nil, 200)
			request(t, "GET", path, university, nil, 200)
			request(t, "GET", path, supervisor, nil, 200)
			request(t, "GET", path, sameCompany, nil, 403)
			must(t, db.Model(&model.User{}).Where("id = ?", supervisor.ID).Update("company_id", otherCompany.ID).Error)
			request(t, "GET", path, supervisor, nil, 403)
			must(t, db.Model(&model.User{}).Where("id = ?", supervisor.ID).Update("company_id", nil).Error)
			request(t, "GET", path, supervisor, nil, 403)
			must(t, db.Model(&model.User{}).Where("id = ?", supervisor.ID).Update("company_id", company.ID).Error)
			must(t, db.Model(&model.User{}).Where("id = ?", supervisor.ID).Update("role", model.RoleStudent).Error)
			// Preserve the old supervisor JWT to ensure current DB membership is checked.
			request(t, "GET", path, supervisor, nil, 403)
			must(t, db.Model(&model.User{}).Where("id = ?", supervisor.ID).Update("role", model.RoleCompanySupervisor).Error)
			must(t, db.Model(&item).Update("selected_preference_id", nil).Error)
			request(t, "GET", path, supervisor, nil, 403)
			must(t, db.Model(&item).Update("selected_preference_id", pref.ID).Error)
			must(t, db.Model(&model.OpportunityApplication{}).Where("id = ?", pref.OpportunityApplicationID).Update("status", model.ApplicationStatusRejected).Error)
			request(t, "GET", path, supervisor, nil, 403)
			must(t, db.Model(&model.OpportunityApplication{}).Where("id = ?", pref.OpportunityApplicationID).Update("status", model.ApplicationStatusAccepted).Error)
			for _, registration := range []model.CompanyRegistrationStatus{model.CompanyRegistrationStatusPending, model.CompanyRegistrationStatusRejected} {
				must(t, db.Model(&company).Update("registration_status", registration).Error)
				want := 200
				if status == model.InternshipCaseStatusActive {
					want = 403
				}
				request(t, "GET", path, supervisor, nil, want)
			}
			must(t, db.Model(&company).Update("registration_status", model.CompanyRegistrationStatusApproved).Error)
		}
	})

	t.Run("generic and student company responses exclude legal fields", func(t *testing.T) {
		for _, user := range []model.User{wrongStudent, professor, university, admin, supervisor} {
			body := request(t, "GET", "/api/companies", user, nil, 200)
			for _, forbidden := range []string{"nationalId", "economicCode", "registrationReviewedBy", "registrationRejectionReason"} {
				if strings.Contains(body, forbidden) {
					t.Fatalf("generic directory leaked %s", forbidden)
				}
			}
		}
		body := request(t, "GET", "/api/student/approved-companies", wrongStudent, nil, 200)
		if strings.Contains(body, "nationalId") || strings.Contains(body, "economicCode") {
			t.Fatal("student directory leaked identifiers")
		}
		for _, endpoint := range []struct {
			user model.User
			path string
		}{{admin, fmt.Sprintf("/api/admin/company-registrations/%d", company.ID)}, {university, fmt.Sprintf("/api/university/companies/%d", company.ID)}, {supervisor, "/api/company/profile"}} {
			body := request(t, "GET", endpoint.path, endpoint.user, nil, 200)
			if !strings.Contains(body, company.NationalID) || !strings.Contains(body, company.EconomicCode) {
				t.Fatal("required review/profile identifiers missing")
			}
		}
		request(t, "GET", fmt.Sprintf("/api/admin/company-registrations/%d", company.ID), wrongStudent, nil, 403)
		request(t, "GET", fmt.Sprintf("/api/university/companies/%d", company.ID), professor, nil, 403)
	})

	t.Run("draft without acceptance and credits above 300", func(t *testing.T) {
		student := makeUser(t, "draft-no-acceptance", model.RoleStudent, nil)
		assignment := model.ProfessorAssignment{StudentID: student.ID, ProfessorID: professor.ID, AssignedAt: time.Now()}
		must(t, db.Create(&assignment).Error)
		term, err := service.NewInternshipTermService(db).Create(university.ID, 1405, model.InternshipTermTypeSummer)
		must(t, err)
		view := registrationDecode[struct {
			ID     uint
			Status model.InternshipCaseStatus
			TermID *uint
		}](t, registrationRequest(t, router, "POST", "/api/student/internship-case", registrationToken(t, student), nil, http.StatusCreated))
		if view.Status != model.InternshipCaseStatusDraft || view.TermID == nil || *view.TermID != term.ID {
			t.Fatal("draft without acceptance failed")
		}
		request(t, "PUT", "/api/student/internship-case", student, map[string]any{"passedCredits": 301, "mobile": "09121111111"}, 200)
		request(t, "PUT", "/api/student/internship-case", student, map[string]any{"passedCredits": -1}, 400)
		request(t, "PUT", "/api/student/internship-case", student, map[string]any{"passedCredits": 1.5}, 400)
		request(t, "POST", "/api/student/internship-case/submit", student, nil, 400)
		request(t, "POST", "/api/student/internship-case/preferences", student, map[string]any{"priority": 1, "opportunityApplicationId": 999999}, 400)
	})
}
