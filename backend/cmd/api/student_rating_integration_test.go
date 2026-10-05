package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"internship-management-system/backend/internal/database"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

type ratingQueryLogger struct {
	logger.Interface
	aggregates atomic.Int64
}

func (l *ratingQueryLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, _ := fc()
	if strings.Contains(strings.ToLower(sql), "avg(rating)") {
		l.aggregates.Add(1)
	}
	l.Interface.Trace(ctx, begin, fc, err)
}

// Committed fixtures in a private schema let simultaneous HTTP requests use
// independent PostgreSQL transactions and exercise real row locks/uniqueness.
func ratingTestDB(t *testing.T) (*gorm.DB, *ratingQueryLogger) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlRoot, err := root.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlRoot.Close() })
	schema := fmt.Sprintf("student_rating_%d", time.Now().UnixNano())
	if err := root.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
	})
	scoped := dsn + " search_path=" + schema
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		scoped = u.String()
	}
	logs := &ratingQueryLogger{Interface: logger.Default.LogMode(logger.Silent)}
	db, err := gorm.Open(postgres.Open(scoped), &gorm.Config{Logger: logs})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() { sqlDB.Close() })
	if err := database.MigrateAndSeed(db); err != nil {
		t.Fatal(err)
	}
	return db, logs
}

func TestStudentCompanyRatingProductionRoutes(t *testing.T) {
	db, logs := ratingTestDB(t)
	router := registrationRouter(t, db)
	workflow := service.NewInternshipService(db)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	create := func(value any) { t.Helper(); must(db.Create(value).Error) }
	student := registrationDemoUser(t, db, "student")
	professor := registrationDemoUser(t, db, "professor")
	university := registrationDemoUser(t, db, "university")
	admin := registrationDemoUser(t, db, "admin")
	company := model.Company{Name: "Rating company A", NationalID: "rating-a", EconomicCode: "rating-e-a", RegistrationStatus: model.CompanyRegistrationStatusApproved}
	otherCompany := model.Company{Name: "Rating company B", NationalID: "rating-b", EconomicCode: "rating-e-b", RegistrationStatus: model.CompanyRegistrationStatusApproved}
	create(&company)
	create(&otherCompany)
	supervisor := model.User{FullName: "Rating supervisor", Email: "rating-supervisor@example.test", PasswordHash: "fixture", Role: model.RoleCompanySupervisor, CompanyID: &company.ID}
	create(&supervisor)
	otherStudent := model.User{FullName: "Other student", Email: "rating-other@example.test", PasswordHash: "fixture", Role: model.RoleStudent}
	create(&otherStudent)
	otherProfessor := model.User{FullName: "Other professor", Email: "rating-prof@example.test", PasswordHash: "fixture", Role: model.RoleProfessor}
	create(&otherProfessor)
	now := time.Now()
	closed := model.InternshipTerm{AcademicYear: 1404, TermType: model.InternshipTermTypeSummer, Status: model.InternshipTermStatusClosed, OpenedAt: now, ClosedAt: &now, CreatedBy: university.ID}
	create(&closed)
	request := func(method, path string, user model.User, body any, want int) *httptest.ResponseRecorder {
		t.Helper()
		return registrationRequest(t, router, method, path, registrationToken(t, user), body, want)
	}
	newOpportunity := func(companyID uint) model.InternshipOpportunity {
		t.Helper()
		value := model.InternshipOpportunity{CompanyID: companyID, CreatedBy: supervisor.ID, Title: "Internship", Description: "Description", WorkField: "Software", Location: "Tehran", Status: model.OpportunityStatusOpen}
		create(&value)
		return value
	}
	a1, a2, b1 := newOpportunity(company.ID), newOpportunity(company.ID), newOpportunity(otherCompany.ID)
	fixture := func(owner model.User, status model.InternshipCaseStatus, op model.InternshipOpportunity) (model.InternshipCase, model.OpportunityApplication) {
		t.Helper()
		item := model.InternshipCase{StudentID: owner.ID, ProfessorID: professor.ID, TermID: &closed.ID, Status: status, CompanySupervisorID: &supervisor.ID}
		create(&item)
		file := model.File{OriginalName: "resume.pdf", StoredName: fmt.Sprintf("rating-%d.pdf", item.ID), Path: "/tmp/rating.pdf", MimeType: "application/pdf", SizeBytes: 20, UploadedBy: owner.ID, UploadedAt: now}
		create(&file)
		application := model.OpportunityApplication{StudentID: owner.ID, OpportunityID: op.ID, ResumeFileID: file.ID, Status: model.ApplicationStatusAccepted, AppliedAt: now}
		var existing model.OpportunityApplication
		if err := db.Where("student_id = ? AND opportunity_id = ?", owner.ID, op.ID).First(&existing).Error; err == nil {
			application = existing
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			create(&application)
		} else {
			t.Fatal(err)
		}
		pref := model.InternshipPreference{InternshipCaseID: item.ID, OpportunityApplicationID: application.ID, Priority: 1}
		create(&pref)
		must(db.Model(&item).UpdateColumn("selected_preference_id", pref.ID).Error)
		return item, application
	}
	path := func(item model.InternshipCase) string {
		return fmt.Sprintf("/api/student/internship-cases/%d/rating", item.ID)
	}
	history := func(item model.InternshipCase) map[string]any {
		t.Helper()
		return registrationDecode[map[string]any](t, request("GET", fmt.Sprintf("/api/student/internship-cases/history/%d", item.ID), student, nil, 200))
	}
	score := func(item model.InternshipCase, n int, want int) {
		t.Helper()
		request("POST", path(item), student, map[string]any{"rating": n}, want)
	}
	aggregate := func(op model.InternshipOpportunity, average *float64, count int64) {
		t.Helper()
		got := registrationDecode[service.CompanyRating](t, request("GET", fmt.Sprintf("/api/student/opportunities/%d", op.ID), otherStudent, nil, 200))
		if !reflect.DeepEqual(got, service.CompanyRating{CompanyAverageRating: average, CompanyRatingCount: count}) {
			t.Fatalf("aggregate: %+v want %v/%d", got, average, count)
		}
	}
	t.Run("no ratings and apply eligibility", func(t *testing.T) { aggregate(a1, nil, 0); aggregate(a2, nil, 0); aggregate(b1, nil, 0) })
	passed, application := fixture(student, model.InternshipCaseStatusPassed, a1)
	t.Run("additive migration preserves legacy passed cases without backfill", func(t *testing.T) {
		var before model.InternshipCase
		must(db.First(&before, passed.ID).Error)
		// This isolated schema now represents the pre-M20 database.
		must(db.Migrator().DropTable(&model.StudentInternshipRating{}))
		must(database.MigrateAndSeed(db))
		var after model.InternshipCase
		must(db.First(&after, passed.ID).Error)
		if !reflect.DeepEqual(before, after) {
			t.Fatal("rating migration changed legacy passed case")
		}
		var count int64
		must(db.Model(&model.StudentInternshipRating{}).Count(&count).Error)
		if count != 0 {
			t.Fatal("migration backfilled ratings")
		}
		view := history(passed)
		if view["canRateInternship"] != true || view["studentRating"] != nil {
			t.Fatal("legacy passed case is not eligible after migration")
		}
	})
	t.Run("only passed status eligible and all other statuses rejected", func(t *testing.T) {
		for _, status := range []model.InternshipCaseStatus{model.InternshipCaseStatusDraft, model.InternshipCaseStatusRevisionRequested, model.InternshipCaseStatusPendingUniversityReview, model.InternshipCaseStatusPendingCompanyDetails, model.InternshipCaseStatusPendingFinalApproval, model.InternshipCaseStatusActive, model.InternshipCaseStatusFailed, model.InternshipCaseStatusCancelled} {
			t.Run(string(status), func(t *testing.T) {
				item, _ := fixture(student, status, a1)
				score(item, 4, 409)
				if status.Terminal() && history(item)["canRateInternship"] != false {
					t.Fatal("ineligible terminal case got controls")
				}
			})
		}
		view := history(passed)
		if view["studentRating"] != nil || view["canRateInternship"] != true {
			t.Fatalf("unrated passed metadata: %v", view)
		}
	})
	t.Run("validation rejects non-integers and client placement selection", func(t *testing.T) {
		for _, body := range []any{map[string]any{"rating": 0}, map[string]any{"rating": -1}, map[string]any{"rating": 6}, map[string]any{"rating": 2.5}, map[string]any{"rating": "4"}, map[string]any{"rating": nil}, map[string]any{}, map[string]any{"rating": true}, map[string]any{"rating": 4, "companyId": otherCompany.ID}, map[string]any{"rating": 4, "opportunityId": b1.ID}, map[string]any{"rating": 4, "studentId": otherStudent.ID}, map[string]any{"rating": 4, "CompanyID": otherCompany.ID}} {
			request("POST", path(passed), student, body, 400)
		}
		token := registrationToken(t, student)
		for _, body := range []string{"{", `{"rating":4.0}`, `{"rating":4} {"rating":5}`, `[]`, `null`} {
			req := httptest.NewRequest("POST", path(passed), strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != 400 {
				t.Fatalf("malformed %s accepted: %d", body, rec.Code)
			}
		}
	})
	t.Run("ownership role and immutable endpoint authorization", func(t *testing.T) {
		request("POST", path(passed), otherStudent, map[string]int{"rating": 4}, 404)
		registrationRequest(t, router, "POST", path(passed), "", map[string]int{"rating": 4}, 401)
		for _, user := range []model.User{supervisor, professor, university, admin} {
			request("POST", path(passed), user, map[string]int{"rating": 4}, 403)
		}
		request("POST", "/api/student/internship-cases/not-an-id/rating", student, map[string]int{"rating": 4}, 400)
		request("POST", "/api/student/internship-cases/999999999/rating", student, map[string]int{"rating": 4}, 404)
		for _, method := range []string{"PUT", "PATCH", "DELETE"} {
			for _, user := range []model.User{student, professor, university, supervisor, admin} {
				request(method, path(passed), user, map[string]int{"rating": 1}, 404)
			}
		}
	})
	t.Run("rating five persisted for selected company without side effects", func(t *testing.T) {
		// An unselected preference to another company must not influence the rating.
		_, otherApplication := fixture(student, model.InternshipCaseStatusPassed, b1)
		create(&model.InternshipPreference{InternshipCaseID: passed.ID, OpportunityApplicationID: otherApplication.ID, Priority: 2})
		var beforeCase model.InternshipCase
		var beforeCompany model.Company
		var beforeTerm model.InternshipTerm
		must(db.First(&beforeCase, passed.ID).Error)
		must(db.First(&beforeCompany, company.ID).Error)
		must(db.First(&beforeTerm, closed.ID).Error)
		approvalBefore, err := service.NewCompanyApprovalService(db).GetCompany(company.ID)
		must(err)
		score(passed, 5, 201)
		var stored model.StudentInternshipRating
		must(db.Where("internship_case_id = ?", passed.ID).First(&stored).Error)
		if stored.CompanyID != company.ID || stored.StudentID != student.ID || stored.Rating != 5 || stored.CreatedAt.IsZero() {
			t.Fatalf("incorrect persisted rating %+v", stored)
		}
		var afterCase model.InternshipCase
		var afterCompany model.Company
		var afterTerm model.InternshipTerm
		must(db.First(&afterCase, passed.ID).Error)
		must(db.First(&afterCompany, company.ID).Error)
		must(db.First(&afterTerm, closed.ID).Error)
		if !reflect.DeepEqual(beforeCase, afterCase) || !reflect.DeepEqual(beforeCompany, afterCompany) || !reflect.DeepEqual(beforeTerm, afterTerm) {
			t.Fatal("rating changed case, company approval/registration, or closed term")
		}
		approvalAfter, err := service.NewCompanyApprovalService(db).GetCompany(company.ID)
		must(err)
		if approvalBefore.PassedInternshipCount != approvalAfter.PassedInternshipCount || approvalAfter.IsApproved {
			t.Fatal("rating altered manual approval eligibility")
		}
		view := history(passed)
		if view["studentRating"] != float64(5) || view["canRateInternship"] != false {
			t.Fatalf("rated metadata: %v", view)
		}
		score(passed, 1, 409)
		var count int64
		must(db.Model(&model.StudentInternshipRating{}).Where("internship_case_id = ?", passed.ID).Count(&count).Error)
		if count != 1 {
			t.Fatal("duplicate rating")
		}
		five := 5.0
		aggregate(a1, &five, 1)
		aggregate(a2, &five, 1)
		aggregate(b1, nil, 0)
	})
	t.Run("case uniqueness permits same student company on multiple historical cases", func(t *testing.T) {
		second, _ := fixture(student, model.InternshipCaseStatusPassed, a2)
		score(second, 3, 201)
		independent, _ := fixture(student, model.InternshipCaseStatusPassed, b1)
		score(independent, 1, 201)
		four, one := 4.0, 1.0
		aggregate(a1, &four, 2)
		aggregate(a2, &four, 2)
		aggregate(b1, &one, 1)
		logs.aggregates.Store(0)
		catalog := registrationDecode[[]struct {
			ID uint
			service.CompanyRating
			CanApply bool
		}](t, request("GET", "/api/student/opportunities", otherStudent, nil, 200))
		if logs.aggregates.Load() != 1 {
			t.Fatalf("catalog aggregate queries=%d, want 1", logs.aggregates.Load())
		}
		for _, view := range catalog {
			if view.ID == a1.ID || view.ID == a2.ID {
				if view.CompanyAverageRating == nil || *view.CompanyAverageRating != 4 || view.CompanyRatingCount != 2 || !view.CanApply {
					t.Fatalf("catalog company rating/apply regression: %+v", view)
				}
			}
		}
		detail := registrationDecode[map[string]any](t, request("GET", fmt.Sprintf("/api/student/opportunities/%d", a1.ID), student, nil, 200))
		if detail["applyRestrictionCode"] != "INTERNSHIP_ALREADY_COMPLETED" {
			t.Fatal("PASSED stopped blocking applications")
		}
		request("POST", "/api/student/internship-case", student, nil, 409)
	})
	t.Run("university and assigned professor read company aggregate", func(t *testing.T) {
		uni := registrationDecode[service.CompanyRating](t, request("GET", fmt.Sprintf("/api/university/companies/%d", company.ID), university, nil, 200))
		if uni.CompanyAverageRating == nil || *uni.CompanyAverageRating != 4 || uni.CompanyRatingCount != 2 {
			t.Fatal("university company aggregate missing")
		}
		review := registrationDecode[service.CompanyRating](t, request("GET", fmt.Sprintf("/api/university/internship-cases/%d", passed.ID), university, nil, 200))
		if !reflect.DeepEqual(uni, review) {
			t.Fatal("university review aggregate mismatch")
		}
		prof := registrationDecode[struct{ Internship service.CompanyRating }](t, request("GET", fmt.Sprintf("/api/professor/internship-cases/%d", passed.ID), professor, nil, 200))
		if !reflect.DeepEqual(uni, prof.Internship) {
			t.Fatal("professor aggregate mismatch")
		}
		request("GET", fmt.Sprintf("/api/professor/internship-cases/%d", passed.ID), otherProfessor, nil, 403)
	})
	t.Run("university sees aggregate before placement selection", func(t *testing.T) {
		item, _ := fixture(student, model.InternshipCaseStatusPendingUniversityReview, a1)
		must(db.Model(&item).UpdateColumn("selected_preference_id", nil).Error)
		view := registrationDecode[struct {
			Preferences []struct{ service.CompanyRating }
		}](t, request("GET", fmt.Sprintf("/api/university/internship-cases/%d", item.ID), university, nil, 200))
		if len(view.Preferences) != 1 || view.Preferences[0].CompanyAverageRating == nil || *view.Preferences[0].CompanyAverageRating != 4 || view.Preferences[0].CompanyRatingCount != 2 {
			t.Fatalf("review preference rating missing: %+v", view)
		}
	})
	t.Run("fractional mean retains API precision", func(t *testing.T) {
		third := model.Company{Name: "Rating company C", NationalID: "rating-c", EconomicCode: "rating-e-c", RegistrationStatus: model.CompanyRegistrationStatusApproved}
		create(&third)
		op := newOpportunity(third.ID)
		for _, n := range []int{5, 4, 1} {
			item, _ := fixture(student, model.InternshipCaseStatusPassed, op)
			score(item, n, 201)
		}
		got := registrationDecode[service.CompanyRating](t, request("GET", fmt.Sprintf("/api/student/opportunities/%d", op.ID), otherStudent, nil, 200))
		if got.CompanyAverageRating == nil || math.Abs(*got.CompanyAverageRating-10.0/3.0) > 1e-12 || got.CompanyRatingCount != 3 {
			t.Fatalf("fractional aggregate: %+v", got)
		}
	})
	t.Run("company and admin response privacy and cross-role guards", func(t *testing.T) {
		paths := []string{"/api/company/profile", "/api/company/opportunities", fmt.Sprintf("/api/company/opportunities/%d", a1.ID), "/api/company/internship-cases", "/api/company/internship-cases/history", fmt.Sprintf("/api/company/internship-cases/%d", passed.ID), fmt.Sprintf("/api/company/internship-cases/history/%d", passed.ID), fmt.Sprintf("/api/company/opportunity-applications/%d", application.ID), fmt.Sprintf("/api/company/opportunities/%d/applications", a1.ID), "/api/companies", "/api/auth/me"}
		for _, p := range paths {
			body := request("GET", p, supervisor, nil, 200).Body.String()
			for _, field := range []string{"studentRating", "averageRating", "ratingCount", "companyAverageRating", "companyRatingCount", "canRateInternship", "StudentRating", "AverageRating", "RatingCount"} {
				if strings.Contains(body, `"`+field+`"`) {
					t.Fatalf("company privacy leak in %s: %s", p, field)
				}
			}
		}
		for _, p := range []string{"/api/student/opportunities", fmt.Sprintf("/api/student/opportunities/%d", a1.ID), fmt.Sprintf("/api/student/internship-cases/history/%d", passed.ID), fmt.Sprintf("/api/university/companies/%d", company.ID), fmt.Sprintf("/api/university/internship-cases/%d", passed.ID), fmt.Sprintf("/api/professor/internship-cases/%d", passed.ID)} {
			request("GET", p, supervisor, nil, 403)
		}
		body := request("GET", "/api/admin/company-registrations", admin, nil, 200).Body.String()
		if strings.Contains(body, "companyAverageRating") || strings.Contains(body, "studentRating") || strings.Contains(body, "companyRatingCount") {
			t.Fatal("admin registration review acquired ratings")
		}
	})
	t.Run("missing or foreign selected preference cannot choose company", func(t *testing.T) {
		missing := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusPassed}
		create(&missing)
		score(missing, 4, 409)
		must(db.Model(&missing).UpdateColumn("selected_preference_id", passed.SelectedPreferenceID).Error)
		score(missing, 4, 409)
		unrelated, _ := fixture(otherStudent, model.InternshipCaseStatusPassed, a1)
		must(db.Model(&missing).UpdateColumn("selected_preference_id", unrelated.SelectedPreferenceID).Error)
		score(missing, 4, 409)
	})
	t.Run("real concurrent submissions create exactly one final rating", func(t *testing.T) {
		item, _ := fixture(student, model.InternshipCaseStatusPassed, a1)
		token := registrationToken(t, student)
		start := make(chan struct{})
		results := make(chan *httptest.ResponseRecorder, 2)
		for _, n := range []int{1, 5} {
			go func(n int) {
				<-start
				req := httptest.NewRequest("POST", path(item), strings.NewReader(fmt.Sprintf(`{"rating":%d}`, n)))
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				results <- rec
			}(n)
		}
		close(start)
		first, second := <-results, <-results
		if !((first.Code == 201 && second.Code == 409) || (first.Code == 409 && second.Code == 201)) {
			t.Fatalf("concurrent responses: %d %s; %d %s", first.Code, first.Body.String(), second.Code, second.Body.String())
		}
		var count int64
		must(db.Model(&model.StudentInternshipRating{}).Where("internship_case_id = ?", item.ID).Count(&count).Error)
		if count != 1 {
			t.Fatal("concurrent duplicate persisted")
		}
	})
	t.Run("database score uniqueness and foreign key constraints", func(t *testing.T) {
		for _, n := range []int{0, -1, 6} {
			item, _ := fixture(student, model.InternshipCaseStatusPassed, a1)
			err := db.Create(&model.StudentInternshipRating{InternshipCaseID: item.ID, StudentID: student.ID, CompanyID: company.ID, Rating: n}).Error
			if err == nil {
				t.Fatal("database accepted invalid rating")
			}
		}
		err := db.Create(&model.StudentInternshipRating{InternshipCaseID: passed.ID, StudentID: student.ID, CompanyID: company.ID, Rating: 4}).Error
		if err == nil {
			t.Fatal("database accepted duplicate case")
		}
		err = db.Create(&model.StudentInternshipRating{InternshipCaseID: 999999999, StudentID: student.ID, CompanyID: company.ID, Rating: 4}).Error
		if err == nil {
			t.Fatal("missing case FK accepted")
		}
		var indexed bool
		must(db.Raw("SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname = current_schema() AND tablename = 'student_internship_ratings' AND indexdef LIKE '%(company_id)%')").Scan(&indexed).Error)
		if !indexed {
			t.Fatal("company aggregate index missing")
		}
	})
	t.Run("M18 sequencing and professor completion require no student rating", func(t *testing.T) {
		owner := model.User{FullName: "Completion student", Email: "rating-completion@example.test", PasswordHash: "fixture", Role: model.RoleStudent}
		create(&owner)
		item, app := fixture(owner, model.InternshipCaseStatusActive, a1)
		finalizePath := fmt.Sprintf("/api/professor/internship-cases/%d/complete", item.ID)
		request("POST", finalizePath, professor, map[string]string{"result": "GOOD"}, 409)
		for week := 1; week <= 8; week++ {
			create(&model.WeeklyReport{InternshipCaseID: item.ID, WeekNumber: week, StartDate: now, EndDate: now, ActivityDescription: "activity", SubmittedAt: &now, CompanyReviewStatus: model.WeeklyReviewApproved, ProfessorReviewStatus: model.WeeklyReviewApproved})
		}
		request("POST", finalizePath, professor, map[string]string{"result": "GOOD"}, 409)
		create(&model.CompanyEvaluation{InternshipCaseID: item.ID, CompanySupervisorID: supervisor.ID, AttendanceRating: model.EvaluationRatingGood, ParticipationRating: model.EvaluationRatingGood, LearningRating: model.EvaluationRatingGood, InterestRating: model.EvaluationRatingGood, PersistenceRating: model.EvaluationRatingGood, SuggestionRating: model.EvaluationRatingGood, ResourceUsageRating: model.EvaluationRatingGood, ReportQualityRating: model.EvaluationRatingGood, ProjectPerformanceRating: model.EvaluationRatingGood, SubmittedAt: now})
		request("POST", finalizePath, professor, map[string]string{"result": "GOOD"}, 409)
		report := model.FinalReport{InternshipCaseID: item.ID, CurrentFileID: app.ResumeFileID, Status: model.FinalReportSubmitted, SubmittedAt: now}
		create(&report)
		request("POST", finalizePath, professor, map[string]string{"result": "GOOD"}, 409)
		must(db.Model(&report).UpdateColumn("status", model.FinalReportApproved).Error)
		request("POST", finalizePath, professor, map[string]string{"result": "GOOD"}, 200)
		completed, err := workflow.GetStudentHistoricalCase(owner.ID, item.ID)
		must(err)
		if completed.Status != model.InternshipCaseStatusPassed || completed.StudentRating != nil {
			t.Fatal("professor completion depends on rating")
		}
		request("POST", "/api/student/internship-case", owner, nil, 409)
		request("POST", path(item), owner, map[string]int{"rating": 2}, 201)
		request("POST", "/api/student/internship-case", owner, nil, 409)
	})
	t.Run("failed retry stays independent of rating and term closure preserves passed eligibility", func(t *testing.T) {
		owner := model.User{FullName: "Retry student", Email: "rating-retry@example.test", PasswordHash: "fixture", Role: model.RoleStudent}
		create(&owner)
		retry, _ := fixture(owner, model.InternshipCaseStatusFailed, a1)
		request("POST", path(retry), owner, map[string]int{"rating": 4}, 409)
		create(&model.ProfessorAssignment{StudentID: owner.ID, ProfessorID: professor.ID, AssignedAt: now})
		open := model.InternshipTerm{AcademicYear: 1405, TermType: model.InternshipTermTypeFirst, Status: model.InternshipTermStatusOpen, OpenedAt: now, CreatedBy: university.ID}
		create(&open)
		draft := registrationDecode[struct {
			ID     uint
			Status model.InternshipCaseStatus
		}](t, request("POST", "/api/student/internship-case", owner, nil, 201))
		if draft.ID == retry.ID || draft.Status != model.InternshipCaseStatusDraft {
			t.Fatal("FAILED retry behavior changed")
		}
		successful, _ := fixture(owner, model.InternshipCaseStatusPassed, a1)
		must(db.Model(&successful).UpdateColumn("term_id", open.ID).Error)
		_, err := service.NewInternshipTermService(db).Close(university.ID, open.ID)
		must(err)
		request("POST", path(successful), owner, map[string]int{"rating": 4}, 201)
		request("POST", fmt.Sprintf("/api/student/internship-cases/%d/rating", draft.ID), owner, map[string]int{"rating": 4}, 409)
	})
	t.Run("migration is idempotent and preserves existing cases and ratings", func(t *testing.T) {
		var beforeCases, afterCases []model.InternshipCase
		var beforeRatings, afterRatings []model.StudentInternshipRating
		var beforeCompanies, afterCompanies []model.Company
		must(db.Order("id").Find(&beforeCases).Error)
		must(db.Order("id").Find(&beforeRatings).Error)
		must(db.Order("id").Find(&beforeCompanies).Error)
		must(database.MigrateAndSeed(db))
		must(database.MigrateAndSeed(db))
		must(db.Order("id").Find(&afterCases).Error)
		must(db.Order("id").Find(&afterRatings).Error)
		must(db.Order("id").Find(&afterCompanies).Error)
		if !reflect.DeepEqual(beforeCases, afterCases) || !reflect.DeepEqual(beforeRatings, afterRatings) || !reflect.DeepEqual(beforeCompanies, afterCompanies) {
			t.Fatal("migration backfilled ratings or changed existing data")
		}
	})

}
