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

func TestWeeklyReportsV2API(t *testing.T) {
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
	companyA := createOpportunityTestCompany(t, tx, suffix, "wra", true)
	companyB := createOpportunityTestCompany(t, tx, suffix, "wrb", true)
	supervisorA := createOpportunityTestUser(t, tx, suffix, "wra", model.RoleCompanySupervisor, &companyA.ID)
	supervisorB := createOpportunityTestUser(t, tx, suffix, "wrb", model.RoleCompanySupervisor, &companyB.ID)
	professorA := createOpportunityTestUser(t, tx, suffix, "wrpa", model.RoleProfessor, nil)
	professorB := createOpportunityTestUser(t, tx, suffix, "wrpb", model.RoleProfessor, nil)
	university := createOpportunityTestUser(t, tx, suffix, "wru", model.RoleUniversitySupervisor, nil)
	admin := createOpportunityTestUser(t, tx, suffix, "wradmin", model.RoleAdmin, nil)
	const secret = "weekly-report-v2-integration"
	token := func(t *testing.T, user model.User) string {
		value, err := auth.CreateToken(user, secret, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	handler := NewInternshipHandler(service.NewInternshipService(tx))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api", appmiddleware.RequireAuth(secret))
	students := api.Group("/student", appmiddleware.RequireRole(model.RoleStudent))
	handler.RegisterStudentWeeklyReportRoutes(students)
	company := api.Group("/company", appmiddleware.RequireRole(model.RoleCompanySupervisor))
	handler.RegisterCompanyWeeklyReportRoutes(company)
	professors := api.Group("/professor", appmiddleware.RequireRole(model.RoleProfessor))
	handler.RegisterProfessorWeeklyReportRoutes(professors)
	company.POST("/internship-cases/:id/evaluation", handler.CreateCompanyEvaluation)
	company.GET("/internship-cases/:id", handler.GetCompanyCase)
	professors.GET("/internship-cases/:id", handler.GetProfessorCase)
	professors.POST("/internship-cases/:id/complete", handler.CompleteProfessorCase)
	request := func(t *testing.T, method, path, bearer string, body any, want int) *httptest.ResponseRecorder {
		t.Helper()
		var data []byte
		if body != nil {
			var err error
			data, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s = %d, want %d: %s", method, path, rec.Code, want, rec.Body.String())
		}
		return rec
	}
	decode := func(t *testing.T, rec *httptest.ResponseRecorder, want model.WeeklyReportStatus) model.WeeklyReport {
		t.Helper()
		var report model.WeeklyReport
		if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		var status struct {
			Status model.WeeklyReportStatus `json:"status"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
			t.Fatal(err)
		}
		if status.Status != want || report.Status() != want {
			t.Fatalf("status = %s / %s, want %s", status.Status, report.Status(), want)
		}
		return report
	}
	studentPath := func(id uint) string { return fmt.Sprintf("/api/student/internship-case/weekly-reports/%d", id) }
	reviewPath := func(role string, caseID, reportID uint) string {
		return fmt.Sprintf("/api/%s/internship-cases/%d/weekly-reports/%d", role, caseID, reportID)
	}
	body := func(week int) map[string]any {
		return map[string]any{"weekNumber": week, "startDate": "2030-01-01", "endDate": "2030-01-01", "activityDescription": " فعالیت هفته "}
	}
	var counter int
	fixture := func(t *testing.T, status model.InternshipCaseStatus) (model.User, model.InternshipCase) {
		t.Helper()
		counter++
		name := fmt.Sprintf("wrstudent%d", counter)
		student := createOpportunityTestUser(t, tx, suffix, name, model.RoleStudent, nil)
		item := model.InternshipCase{StudentID: student.ID, ProfessorID: professorA.ID, CompanySupervisorID: &supervisorA.ID, Status: status}
		if err := tx.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
		opportunity := model.InternshipOpportunity{CompanyID: companyA.ID, CreatedBy: supervisorA.ID, Title: name, Description: "weekly test", WorkField: "software", Location: "Tehran", Status: model.OpportunityStatusOpen}
		if err := tx.Create(&opportunity).Error; err != nil {
			t.Fatal(err)
		}
		file := model.File{OriginalName: "resume.pdf", StoredName: fmt.Sprintf("wr-%d-%d.pdf", suffix, counter), Path: "/tmp/resume.pdf", MimeType: "application/pdf", SizeBytes: 100, UploadedBy: student.ID, UploadedAt: time.Now()}
		if err := tx.Create(&file).Error; err != nil {
			t.Fatal(err)
		}
		application := model.OpportunityApplication{OpportunityID: opportunity.ID, StudentID: student.ID, ResumeFileID: file.ID, Status: model.ApplicationStatusAccepted, AppliedAt: time.Now()}
		if err := tx.Create(&application).Error; err != nil {
			t.Fatal(err)
		}
		pref := model.InternshipPreference{InternshipCaseID: item.ID, OpportunityApplicationID: application.ID, Priority: 1}
		if err := tx.Create(&pref).Error; err != nil {
			t.Fatal(err)
		}
		if err := tx.Model(&item).Update("selected_preference_id", pref.ID).Error; err != nil {
			t.Fatal(err)
		}
		return student, item
	}
	draft := func(t *testing.T, student model.User, week int) model.WeeklyReport {
		t.Helper()
		return decode(t, request(t, "POST", "/api/student/internship-case/weekly-reports", token(t, student), body(week), 201), model.WeeklyReportDraft)
	}
	submit := func(t *testing.T, student model.User, report model.WeeklyReport) model.WeeklyReport {
		t.Helper()
		return decode(t, request(t, "POST", studentPath(report.ID)+"/submit", token(t, student), nil, 200), model.WeeklyReportSubmitted)
	}
	review := func(t *testing.T, user model.User, role string, item model.InternshipCase, report model.WeeklyReport, action string, comment any, want int) *httptest.ResponseRecorder {
		t.Helper()
		return request(t, "POST", reviewPath(role, item.ID, report.ID)+"/"+action, token(t, user), comment, want)
	}

	t.Run("drafts validation week constraints and ownership", func(t *testing.T) {
		student, item := fixture(t, model.InternshipCaseStatusActive)
		// Week 3 before Week 2; equal and future dates are valid.
		report := draft(t, student, 3)
		if report.SubmittedAt != nil || report.CompanyReviewStatus != model.WeeklyReviewPending || report.ProfessorReviewStatus != model.WeeklyReviewPending || report.ActivityDescription != "فعالیت هفته" {
			t.Fatalf("unexpected draft: %+v", report)
		}
		request(t, "POST", "/api/student/internship-case/weekly-reports", token(t, student), body(3), 409)
		for _, week := range []int{0, 9} {
			request(t, "POST", "/api/student/internship-case/weekly-reports", token(t, student), body(week), 400)
		}
		invalid := body(1)
		invalid["endDate"] = "2029-12-31"
		request(t, "POST", "/api/student/internship-case/weekly-reports", token(t, student), invalid, 400)
		invalid = body(1)
		invalid["activityDescription"] = " \n\t "
		request(t, "POST", "/api/student/internship-case/weekly-reports", token(t, student), invalid, 400)
		valid := body(1)
		valid["endDate"] = "2031-12-31"
		valid["studentId"] = 999
		valid["companyId"] = companyB.ID
		valid["professorId"] = professorB.ID
		valid["companyReviewStatus"] = "APPROVED"
		another := decode(t, request(t, "POST", "/api/student/internship-case/weekly-reports", token(t, student), valid, 201), model.WeeklyReportDraft)
		if another.InternshipCaseID != item.ID || another.CompanyReviewStatus != model.WeeklyReviewPending {
			t.Fatal("client controlled ownership or review status")
		}
		request(t, "PUT", studentPath(report.ID), token(t, student), body(2), 400)
		decode(t, request(t, "PUT", studentPath(report.ID), token(t, student), body(3), 200), model.WeeklyReportDraft)
		other, _ := fixture(t, model.InternshipCaseStatusActive)
		request(t, "GET", studentPath(report.ID), token(t, other), nil, 404)
		request(t, "PUT", studentPath(report.ID), token(t, other), body(3), 404)
		request(t, "POST", studentPath(report.ID)+"/submit", token(t, other), nil, 404)
		decode(t, request(t, "GET", studentPath(report.ID), token(t, student), nil, 200), model.WeeklyReportDraft)
		// Drafts are private to the student, including nested case responses.
		for _, role := range []string{"company", "professor"} {
			user := supervisorA
			if role == "professor" {
				user = professorA
			}
			rec := request(t, "GET", fmt.Sprintf("/api/%s/internship-cases/%d/weekly-reports", role, item.ID), token(t, user), nil, 200)
			if rec.Body.String() != "[]" {
				t.Fatalf("draft exposed: %s", rec.Body.String())
			}
			request(t, "GET", reviewPath(role, item.ID, report.ID), token(t, user), nil, 404)
			review(t, user, role, item, report, "approve", nil, 409)
		}
		for _, role := range []string{"company", "professor"} {
			user := supervisorA
			if role == "professor" {
				user = professorA
			}
			rec := request(t, "GET", fmt.Sprintf("/api/%s/internship-cases/%d", role, item.ID), token(t, user), nil, 200)
			var detail struct {
				WeeklyReports []model.WeeklyReport `json:"weeklyReports"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
				t.Fatal(err)
			}
			if len(detail.WeeklyReports) != 0 {
				t.Fatal("nested case response exposed drafts")
			}
		}

		// Database constraints, independently of service validation.
		for i, week := range []int{0, 9, 3} {
			savepoint := fmt.Sprintf("wr_constraint_%d", i)
			tx.SavePoint(savepoint)
			row := report
			row.ID = 0
			row.WeekNumber = week
			err := tx.Create(&row).Error
			tx.RollbackTo(savepoint)
			if err == nil {
				t.Fatalf("database accepted week %d", week)
			}
		}
		tx.SavePoint("wr_date_constraint")
		row := report
		row.ID = 0
		row.WeekNumber = 8
		row.EndDate = row.StartDate.AddDate(0, 0, -1)
		err := tx.Create(&row).Error
		tx.RollbackTo("wr_date_constraint")
		if err == nil {
			t.Fatal("database accepted reversed dates")
		}
	})

	for _, firstRole := range []string{"company", "professor"} {
		t.Run("independent approvals "+firstRole, func(t *testing.T) {
			student, item := fixture(t, model.InternshipCaseStatusActive)
			report := draft(t, student, 1)
			report = submit(t, student, report)
			if report.SubmittedAt == nil {
				t.Fatal("missing submission timestamp")
			}
			request(t, "PUT", studentPath(report.ID), token(t, student), body(1), 409)
			request(t, "POST", studentPath(report.ID)+"/submit", token(t, student), nil, 409)
			first, second := supervisorA, professorA
			secondRole := "professor"
			if firstRole == "professor" {
				first, second = professorA, supervisorA
				secondRole = "company"
			}
			report = decode(t, review(t, first, firstRole, item, report, "approve", map[string]any{"comment": "  looks good  "}, 200), model.WeeklyReportSubmitted)
			if firstRole == "company" && (report.CompanyReviewStatus != model.WeeklyReviewApproved || report.ProfessorReviewStatus != model.WeeklyReviewPending || report.CompanyReviewComment == nil || *report.CompanyReviewComment != "looks good") {
				t.Fatal("company approval was not independent/trimmed")
			}
			if firstRole == "professor" && (report.ProfessorReviewStatus != model.WeeklyReviewApproved || report.CompanyReviewStatus != model.WeeklyReviewPending || report.ProfessorReviewComment == nil || *report.ProfessorReviewComment != "looks good") {
				t.Fatal("professor approval was not independent/trimmed")
			}
			request(t, "PUT", studentPath(report.ID), token(t, student), body(1), 409)
			review(t, first, firstRole, item, report, "approve", nil, 409)
			report = decode(t, review(t, second, secondRole, item, report, "approve", nil, 200), model.WeeklyReportApproved)
			if report.CompanyReviewedAt == nil || report.ProfessorReviewedAt == nil {
				t.Fatal("review timestamps missing")
			}
			request(t, "PUT", studentPath(report.ID), token(t, student), body(1), 409)
			request(t, "POST", studentPath(report.ID)+"/submit", token(t, student), nil, 409)
			for _, action := range []string{"approve", "request-revision"} {
				review(t, first, firstRole, item, report, action, map[string]string{"comment": "change"}, 409)
				review(t, second, secondRole, item, report, action, map[string]string{"comment": "change"}, 409)
			}
		})
	}

	for _, revisionRole := range []string{"company", "professor"} {
		t.Run("revision and resubmission "+revisionRole, func(t *testing.T) {
			student, item := fixture(t, model.InternshipCaseStatusActive)
			report := submit(t, student, draft(t, student, 1))
			revisionUser, otherUser := supervisorA, professorA
			otherRole := "professor"
			if revisionRole == "professor" {
				revisionUser, otherUser = professorA, supervisorA
				otherRole = "company"
			}
			before := report
			for _, comment := range []any{nil, map[string]string{"comment": " \n\t "}} {
				review(t, revisionUser, revisionRole, item, report, "request-revision", comment, 400)
			}
			report = decode(t, request(t, "GET", studentPath(report.ID), token(t, student), nil, 200), model.WeeklyReportSubmitted)
			if report.CompanyReviewedAt != nil || report.ProfessorReviewedAt != nil {
				t.Fatal("invalid revision mutated report")
			}
			report = decode(t, review(t, revisionUser, revisionRole, item, report, "request-revision", map[string]string{"comment": "  add details  "}, 200), model.WeeklyReportRevisionRequested)
			for _, action := range []string{"approve", "request-revision"} {
				review(t, otherUser, otherRole, item, report, action, map[string]string{"comment": "feedback"}, 409)
			}
			input := body(1)
			input["activityDescription"] = "corrected content"
			report = decode(t, request(t, "PUT", studentPath(report.ID), token(t, student), input, 200), model.WeeklyReportRevisionRequested)
			report = submit(t, student, report)
			report = decode(t, review(t, otherUser, otherRole, item, report, "approve", map[string]string{"comment": "  approved older version  "}, 200), model.WeeklyReportSubmitted)
			report = decode(t, review(t, revisionUser, revisionRole, item, report, "request-revision", map[string]string{"comment": "  improve again  "}, 200), model.WeeklyReportRevisionRequested)
			commentCompany, commentProfessor := report.CompanyReviewComment, report.ProfessorReviewComment
			timeCompany, timeProfessor := report.CompanyReviewedAt, report.ProfessorReviewedAt
			oldSubmission := *report.SubmittedAt
			request(t, "PUT", studentPath(report.ID), token(t, student), input, 200)
			report = submit(t, student, report)
			if report.CompanyReviewStatus != model.WeeklyReviewPending || report.ProfessorReviewStatus != model.WeeklyReviewPending || !report.SubmittedAt.After(oldSubmission) || !report.SubmittedAt.After(*before.SubmittedAt) {
				t.Fatal("resubmission did not reset both reviews/timestamp")
			}
			if report.CompanyReviewComment == nil || report.ProfessorReviewComment == nil || *report.CompanyReviewComment != *commentCompany || *report.ProfessorReviewComment != *commentProfessor || !report.CompanyReviewedAt.Equal(timeCompany.Truncate(time.Microsecond)) || !report.ProfessorReviewedAt.Equal(timeProfessor.Truncate(time.Microsecond)) {
				t.Fatal("latest feedback was not preserved")
			}
			report = decode(t, review(t, revisionUser, revisionRole, item, report, "approve", nil, 200), model.WeeklyReportSubmitted)
			if revisionRole == "company" && report.CompanyReviewComment != nil || revisionRole == "professor" && report.ProfessorReviewComment != nil {
				t.Fatal("blank approval left stale revision comment")
			}
			report = decode(t, review(t, otherUser, otherRole, item, report, "approve", map[string]string{"comment": "  "}, 200), model.WeeklyReportApproved)
			if report.CompanyReviewComment != nil || report.ProfessorReviewComment != nil {
				t.Fatal("blank approval did not clear feedback")
			}
		})
	}

	t.Run("case snapshot company relationship and role security", func(t *testing.T) {
		student, item := fixture(t, model.InternshipCaseStatusActive)
		report := submit(t, student, draft(t, student, 1))
		assignment := model.ProfessorAssignment{StudentID: student.ID, ProfessorID: professorB.ID, AssignedAt: time.Now()}
		if err := tx.Create(&assignment).Error; err != nil {
			t.Fatal(err)
		}
		for _, role := range []string{"company", "professor"} {
			owner, outsider := supervisorA, supervisorB
			if role == "professor" {
				owner, outsider = professorA, professorB
			}
			base := reviewPath(role, item.ID, report.ID)
			request(t, "GET", base, token(t, outsider), nil, 403)
			request(t, "GET", fmt.Sprintf("/api/%s/internship-cases/%d/weekly-reports", role, item.ID), token(t, outsider), nil, 403)
			for _, action := range []string{"approve", "request-revision"} {
				review(t, outsider, role, item, report, action, map[string]string{"comment": "change"}, 403)
				for _, wrong := range []model.User{student, university, admin, supervisorA, professorA} {
					if wrong.ID == owner.ID {
						continue
					}
					request(t, "POST", base+"/"+action, token(t, wrong), map[string]string{"comment": "change"}, 403)
				}
				request(t, "POST", base+"/"+action, "", nil, 401)
			}
			decode(t, request(t, "GET", base, token(t, owner), nil, 200), model.WeeklyReportSubmitted)
		}
		// The selected company still controls access if the supervisor moves company.
		if err := tx.Model(&model.User{}).Where("id = ?", supervisorA.ID).Update("company_id", companyB.ID).Error; err != nil {
			t.Fatal(err)
		}
		request(t, "GET", reviewPath("company", item.ID, report.ID), token(t, supervisorA), nil, 403)
		request(t, "GET", fmt.Sprintf("/api/company/internship-cases/%d/weekly-reports", item.ID), token(t, supervisorA), nil, 403)
		for _, action := range []string{"approve", "request-revision"} {
			review(t, supervisorA, "company", item, report, action, map[string]string{"comment": "change"}, 403)
		}
		if err := tx.Model(&model.User{}).Where("id = ?", supervisorA.ID).Update("company_id", companyA.ID).Error; err != nil {
			t.Fatal(err)
		}
		decode(t, review(t, professorA, "professor", item, report, "approve", nil, 200), model.WeeklyReportSubmitted)
		rec := request(t, "GET", fmt.Sprintf("/api/company/internship-cases/%d", item.ID), token(t, supervisorA), nil, 200)
		if bytes.Contains(rec.Body.Bytes(), []byte(`"priority"`)) {
			t.Fatal("company response exposed preference priority")
		}
	})

	for _, status := range []model.InternshipCaseStatus{model.InternshipCaseStatusDraft, model.InternshipCaseStatusPendingUniversityReview, model.InternshipCaseStatusPendingCompanyDetails, model.InternshipCaseStatusPendingFinalApproval, model.InternshipCaseStatusRevisionRequested, model.InternshipCaseStatusPassed, model.InternshipCaseStatusFailed, model.InternshipCaseStatusCancelled} {
		t.Run("inactive "+string(status), func(t *testing.T) {
			student, item := fixture(t, model.InternshipCaseStatusActive)
			report := draft(t, student, 1)
			if err := tx.Model(&item).Update("status", status).Error; err != nil {
				t.Fatal(err)
			}
			request(t, "POST", "/api/student/internship-case/weekly-reports", token(t, student), body(2), 400)
			request(t, "PUT", studentPath(report.ID), token(t, student), body(1), 400)
			request(t, "POST", studentPath(report.ID)+"/submit", token(t, student), nil, 400)
			if err := tx.Model(&item).Update("status", model.InternshipCaseStatusActive).Error; err != nil {
				t.Fatal(err)
			}
			report = submit(t, student, report)
			if err := tx.Model(&item).Update("status", status).Error; err != nil {
				t.Fatal(err)
			}
			for _, action := range []string{"approve", "request-revision"} {
				review(t, supervisorA, "company", item, report, action, map[string]string{"comment": "change"}, 400)
				review(t, professorA, "professor", item, report, action, map[string]string{"comment": "change"}, 400)
			}
			if status == model.InternshipCaseStatusPassed || status == model.InternshipCaseStatusFailed {
				decode(t, request(t, "GET", studentPath(report.ID), token(t, student), nil, 200), model.WeeklyReportSubmitted)
			}
		})
	}

	t.Run("company evaluation and professor final readiness", func(t *testing.T) {
		student, item := fixture(t, model.InternshipCaseStatusActive)
		evaluation := map[string]any{"attendanceRating": "GOOD", "participationRating": "GOOD", "learningRating": "GOOD", "interestRating": "GOOD", "persistenceRating": "GOOD", "suggestionRating": "GOOD", "resourceUsageRating": "GOOD", "reportQualityRating": "GOOD", "projectPerformanceRating": "GOOD", "leaveDays": 1, "absenceDays": 0}
		evalPath := fmt.Sprintf("/api/company/internship-cases/%d/evaluation", item.ID)
		completePath := fmt.Sprintf("/api/professor/internship-cases/%d/complete", item.ID)
		reports := []model.WeeklyReport{}
		for week := 1; week <= 8; week++ {
			report := submit(t, student, draft(t, student, week))
			reports = append(reports, report)
			if week < 8 {
				review(t, supervisorA, "company", item, report, "approve", nil, 200)
			}
		}
		request(t, "POST", evalPath, token(t, supervisorA), evaluation, 409)
		last := reports[7]
		review(t, supervisorA, "company", item, last, "request-revision", map[string]string{"comment": "fix"}, 200)
		request(t, "POST", evalPath, token(t, supervisorA), evaluation, 409)
		submit(t, student, last)
		review(t, supervisorA, "company", item, last, "approve", nil, 200)
		// All eight company approvals suffice while EVERY professor review is pending.
		request(t, "POST", evalPath, token(t, supervisorA), evaluation, 201)
		request(t, "POST", evalPath, token(t, supervisorA), evaluation, 409)
		completion := map[string]string{"result": "GOOD"}
		request(t, "POST", completePath, token(t, professorA), completion, 409)
		for i, report := range reports {
			review(t, professorA, "professor", item, report, "approve", nil, 200)
			if i < 7 {
				request(t, "POST", completePath, token(t, professorA), completion, 409)
			}
		}
		// Final completion requires a professor-approved final report.
		request(t, "POST", completePath, token(t, professorA), completion, 409)
		file := model.File{OriginalName: "final.pdf", StoredName: fmt.Sprintf("wr-final-%d.pdf", suffix), Path: "/tmp/final.pdf", MimeType: "application/pdf", SizeBytes: 100, UploadedBy: student.ID, UploadedAt: time.Now()}
		if err := tx.Create(&file).Error; err != nil {
			t.Fatal(err)
		}
		finalReport := model.FinalReport{InternshipCaseID: item.ID, CurrentFileID: file.ID, Status: model.FinalReportApproved, SubmittedAt: time.Now()}
		if err := tx.Create(&finalReport).Error; err != nil {
			t.Fatal(err)
		}
		// Also ensure a missing company approval blocks final completion.
		if err := tx.Model(&model.WeeklyReport{}).Where("id = ?", last.ID).Update("company_review_status", model.WeeklyReviewPending).Error; err != nil {
			t.Fatal(err)
		}
		request(t, "POST", completePath, token(t, professorA), completion, 409)
		review(t, supervisorA, "company", item, last, "approve", nil, 200)
		request(t, "POST", completePath, token(t, professorA), completion, 200)
		request(t, "POST", completePath, token(t, professorA), completion, 409)
	})
}
