package service_test

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

func TestObsoleteCompanyWorkflowIsDisabled(t *testing.T) {
	workflow := service.NewInternshipService(nil)
	checks := []struct {
		name string
		run  func() error
	}{
		{name: "manual supervisor selection", run: func() error { _, err := workflow.SendToCompany(1, service.SendToCompanyInput{}); return err }},
		{name: "company confirmation", run: func() error {
			_, err := workflow.ConfirmCompanyCase(1, 1, service.CompanyConfirmationInput{})
			return err
		}},
		{name: "legacy university approval", run: func() error { _, err := workflow.ApproveUniversityCase(1); return err }},
		{name: "manual activation", run: func() error { _, err := workflow.ActivateUniversityCase(1); return err }},
	}
	for _, check := range checks {
		if err := check.run(); !errors.Is(err, service.ErrObsoleteWorkflow) {
			t.Errorf("%s error = %v, want obsolete workflow", check.name, err)
		}
	}
}

func TestWorkflow(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect to integration database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Company{}, &model.User{}, &model.InternshipOpportunity{}, &model.ProfessorAssignment{},
		&model.File{}, &model.OpportunityApplication{}, &model.InternshipCase{}, &model.InternshipPreference{},
		&model.WeeklyReport{}, &model.CompanyEvaluation{},
	); err != nil {
		t.Fatalf("migrate integration database: %v", err)
	}

	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin test transaction: %v", tx.Error)
	}
	defer tx.Rollback()
	workflow := service.NewInternshipService(tx)
	suffix := time.Now().UnixNano()

	t.Run("company identifiers are required and unique", func(t *testing.T) {
		tests := []struct {
			name    string
			company model.Company
		}{
			{name: "national id required", company: model.Company{Name: fmt.Sprintf("missing-national-%d", suffix), EconomicCode: fmt.Sprintf("economic-a-%d", suffix)}},
			{name: "economic code required", company: model.Company{Name: fmt.Sprintf("missing-economic-%d", suffix), NationalID: fmt.Sprintf("national-a-%d", suffix)}},
		}
		for index, test := range tests {
			savepoint := fmt.Sprintf("company_required_%d", index)
			if err := tx.SavePoint(savepoint).Error; err != nil {
				t.Fatalf("create savepoint: %v", err)
			}
			err := tx.Create(&test.company).Error
			if rollbackErr := tx.RollbackTo(savepoint).Error; rollbackErr != nil {
				t.Fatalf("rollback failed company insert: %v", rollbackErr)
			}
			if err == nil {
				t.Fatalf("%s: expected database constraint error", test.name)
			}
		}

		for index, column := range []string{"national_id", "economic_code"} {
			savepoint := fmt.Sprintf("company_unique_%d", index)
			if err := tx.SavePoint(savepoint).Error; err != nil {
				t.Fatalf("create savepoint: %v", err)
			}
			base := model.Company{
				Name:         fmt.Sprintf("unique-base-%d-%d", suffix, index),
				NationalID:   fmt.Sprintf("national-%d-%d", suffix, index),
				EconomicCode: fmt.Sprintf("economic-%d-%d", suffix, index),
			}
			if err := tx.Create(&base).Error; err != nil {
				t.Fatalf("create company uniqueness fixture: %v", err)
			}
			duplicate := model.Company{
				Name:         fmt.Sprintf("unique-duplicate-%d-%d", suffix, index),
				NationalID:   fmt.Sprintf("other-national-%d-%d", suffix, index),
				EconomicCode: fmt.Sprintf("other-economic-%d-%d", suffix, index),
			}
			if column == "national_id" {
				duplicate.NationalID = base.NationalID
			} else {
				duplicate.EconomicCode = base.EconomicCode
			}
			err := tx.Create(&duplicate).Error
			if rollbackErr := tx.RollbackTo(savepoint).Error; rollbackErr != nil {
				t.Fatalf("rollback duplicate company: %v", rollbackErr)
			}
			if err == nil {
				t.Fatalf("expected %s uniqueness error", column)
			}
		}
	})

	professor := createTestUser(t, tx, suffix, "professor", model.RoleProfessor)
	supervisor := createTestUser(t, tx, suffix, "supervisor", model.RoleCompanySupervisor)
	otherSupervisor := createTestUser(t, tx, suffix, "other-supervisor", model.RoleCompanySupervisor)

	t.Run("preference priority is unique per case", func(t *testing.T) {
		student := createTestStudent(t, tx, suffix, "preference-unique")
		internshipCase := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusDraft}
		if err := tx.Create(&internshipCase).Error; err != nil {
			t.Fatalf("create preference case: %v", err)
		}
		if err := tx.SavePoint("preference_priority_unique").Error; err != nil {
			t.Fatalf("create preference savepoint: %v", err)
		}
		firstApplication := createWorkflowOpportunityApplication(t, tx, suffix, student.ID, professor.ID, "priority-first", model.ApplicationStatusAccepted, model.OpportunityStatusOpen)
		secondApplication := createWorkflowOpportunityApplication(t, tx, suffix, student.ID, professor.ID, "priority-second", model.ApplicationStatusAccepted, model.OpportunityStatusOpen)
		first := model.InternshipPreference{InternshipCaseID: internshipCase.ID, OpportunityApplicationID: firstApplication.ID, Priority: 1}
		if err := tx.Create(&first).Error; err != nil {
			t.Fatalf("create first preference: %v", err)
		}
		duplicate := model.InternshipPreference{InternshipCaseID: internshipCase.ID, OpportunityApplicationID: secondApplication.ID, Priority: 1}
		err := tx.Create(&duplicate).Error
		if rollbackErr := tx.RollbackTo("preference_priority_unique").Error; rollbackErr != nil {
			t.Fatalf("rollback duplicate preference: %v", rollbackErr)
		}
		if err == nil {
			t.Fatal("expected duplicate case/priority constraint error")
		}
	})
	t.Run("student with no case creates a draft", func(t *testing.T) {
		student := createTestStudent(t, tx, suffix, "new-case")
		createAssignment(t, tx, student.ID, professor.ID)
		internshipCase, created, err := workflow.CreateOrGetCase(student.ID)
		if err != nil || !created || internshipCase.Status != model.InternshipCaseStatusDraft {
			t.Fatalf("create draft: case=%+v created=%v err=%v", internshipCase, created, err)
		}
	})

	t.Run("non-terminal case is reused", func(t *testing.T) {
		student := createTestStudent(t, tx, suffix, "reuse-case")
		createAssignment(t, tx, student.ID, professor.ID)
		first, created, err := workflow.CreateOrGetCase(student.ID)
		if err != nil || !created {
			t.Fatalf("create first case: case=%+v created=%v err=%v", first, created, err)
		}
		second, created, err := workflow.CreateOrGetCase(student.ID)
		if err != nil || created || second.ID != first.ID {
			t.Fatalf("reuse case: first=%d second=%+v created=%v err=%v", first.ID, second, created, err)
		}
	})

	t.Run("cancelled case permits a new draft", func(t *testing.T) {
		student := createTestStudent(t, tx, suffix, "cancelled-case")
		createAssignment(t, tx, student.ID, professor.ID)
		cancelled := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusCancelled}
		if err := tx.Create(&cancelled).Error; err != nil {
			t.Fatalf("create cancelled case: %v", err)
		}
		internshipCase, created, err := workflow.CreateOrGetCase(student.ID)
		if err != nil || !created || internshipCase.ID == cancelled.ID || internshipCase.Status != model.InternshipCaseStatusDraft {
			t.Fatalf("create after cancellation: case=%+v created=%v err=%v", internshipCase, created, err)
		}
	})

	t.Run("completed case blocks a new case", func(t *testing.T) {
		student := createTestStudent(t, tx, suffix, "completed-case")
		createAssignment(t, tx, student.ID, professor.ID)
		completed := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusCompleted}
		if err := tx.Create(&completed).Error; err != nil {
			t.Fatalf("create completed case: %v", err)
		}
		if _, created, err := workflow.CreateOrGetCase(student.ID); !errors.Is(err, service.ErrInternshipCompleted) || created {
			t.Fatalf("completed case result: created=%v err=%v", created, err)
		}
	})

	t.Run("submission enters university review", func(t *testing.T) {
		student := createTestStudent(t, tx, suffix, "submission")
		createAssignment(t, tx, student.ID, professor.ID)
		credits, mobile := 80, "09120000000"
		internshipCase := model.InternshipCase{
			StudentID: student.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusDraft,
			PassedCredits: &credits, Mobile: &mobile,
		}
		if err := tx.Create(&internshipCase).Error; err != nil {
			t.Fatalf("create draft case: %v", err)
		}
		application := createWorkflowOpportunityApplication(t, tx, suffix, student.ID, professor.ID, "submission", model.ApplicationStatusAccepted, model.OpportunityStatusOpen)
		preference := model.InternshipPreference{
			InternshipCaseID: internshipCase.ID, OpportunityApplicationID: application.ID, Priority: 1,
		}
		if err := tx.Create(&preference).Error; err != nil {
			t.Fatalf("create draft preference: %v", err)
		}
		submitted, err := workflow.SubmitCase(student.ID)
		if err != nil || submitted.Status != model.InternshipCaseStatusPendingUniversityReview {
			t.Fatalf("submit case: case=%+v err=%v", submitted, err)
		}
	})

	t.Run("active internship reporting workflow", func(t *testing.T) {
		student := createTestStudent(t, tx, suffix, "reporting")
		activeCase := model.InternshipCase{
			StudentID: student.ID, ProfessorID: professor.ID, CompanySupervisorID: &supervisor.ID,
			Status: model.InternshipCaseStatusActive,
		}
		if err := tx.Create(&activeCase).Error; err != nil {
			t.Fatalf("create active case: %v", err)
		}
		evaluationInput := service.CompanyEvaluationInput{
			AttendanceRating: model.EvaluationRatingExcellent, ParticipationRating: model.EvaluationRatingGood,
			LearningRating: model.EvaluationRatingExcellent, InterestRating: model.EvaluationRatingGood,
			PersistenceRating: model.EvaluationRatingGood, SuggestionRating: model.EvaluationRatingAverage,
			ResourceUsageRating: model.EvaluationRatingGood, ReportQualityRating: model.EvaluationRatingExcellent,
			ProjectPerformanceRating: model.EvaluationRatingGood, LeaveDays: 1, AbsenceDays: 0,
		}
		if _, err := workflow.CreateCompanyEvaluation(supervisor.ID, activeCase.ID, evaluationInput); !errors.Is(err, service.ErrWeeklyReportsIncomplete) {
			t.Fatalf("evaluation with no reports error = %v", err)
		}

		input := service.WeeklyReportInput{
			WeekNumber: 1, StartDate: testDate(t, "2026-07-11"), EndDate: testDate(t, "2026-07-17"),
			ActivityDescription: "توسعه API",
		}
		report, err := workflow.CreateWeeklyReport(student.ID, input)
		if err != nil || report.IsConfirmed || report.SubmittedAt.IsZero() {
			t.Fatalf("create weekly report: report=%+v err=%v", report, err)
		}
		if _, err := workflow.CreateWeeklyReport(student.ID, input); !errors.Is(err, service.ErrDuplicateWeeklyReport) {
			t.Fatalf("duplicate weekly report error = %v", err)
		}
		invalid := input
		invalid.WeekNumber = 9
		if _, err := workflow.CreateWeeklyReport(student.ID, invalid); !errors.Is(err, service.ErrInvalidWeeklyReport) {
			t.Fatalf("invalid week error = %v", err)
		}
		input.ActivityDescription = "توسعه و آزمون API"
		if _, err := workflow.UpdateWeeklyReport(student.ID, report.ID, input); err != nil {
			t.Fatalf("update weekly report: %v", err)
		}
		if _, err := workflow.ListCompanyWeeklyReports(otherSupervisor.ID, activeCase.ID); !errors.Is(err, service.ErrCaseAccessDenied) {
			t.Fatalf("unassigned report access error = %v", err)
		}
		confirmed, err := workflow.ConfirmWeeklyReport(supervisor.ID, activeCase.ID, report.ID, nil)
		if err != nil || !confirmed.IsConfirmed || confirmed.ConfirmedAt == nil {
			t.Fatalf("confirm weekly report: report=%+v err=%v", confirmed, err)
		}
		if _, err := workflow.UpdateWeeklyReport(student.ID, report.ID, input); !errors.Is(err, service.ErrWeeklyReportConfirmed) {
			t.Fatalf("confirmed report update error = %v", err)
		}

		if _, err := workflow.CreateCompanyEvaluation(supervisor.ID, activeCase.ID, evaluationInput); !errors.Is(err, service.ErrWeeklyReportsIncomplete) {
			t.Fatalf("evaluation with fewer than 8 reports error = %v", err)
		}

		var eighthReport *model.WeeklyReport
		for week := 2; week <= 8; week++ {
			weeklyReport, err := workflow.CreateWeeklyReport(student.ID, service.WeeklyReportInput{
				WeekNumber: week, StartDate: testDate(t, "2026-07-18"), EndDate: testDate(t, "2026-07-24"),
				ActivityDescription: fmt.Sprintf("گزارش هفته %d", week),
			})
			if err != nil {
				t.Fatalf("create report for week %d: %v", week, err)
			}
			if week == 8 {
				eighthReport = weeklyReport
				continue
			}
			if _, err := workflow.ConfirmWeeklyReport(supervisor.ID, activeCase.ID, weeklyReport.ID, nil); err != nil {
				t.Fatalf("confirm report for week %d: %v", week, err)
			}
		}
		if _, err := workflow.CreateCompanyEvaluation(supervisor.ID, activeCase.ID, evaluationInput); !errors.Is(err, service.ErrWeeklyReportsIncomplete) {
			t.Fatalf("evaluation with one unconfirmed report error = %v", err)
		}
		if eighthReport == nil {
			t.Fatal("week 8 report was not created")
		}
		if _, err := workflow.ConfirmWeeklyReport(supervisor.ID, activeCase.ID, eighthReport.ID, nil); err != nil {
			t.Fatalf("confirm report for week 8: %v", err)
		}
		if _, err := workflow.CreateCompanyEvaluation(supervisor.ID, activeCase.ID, evaluationInput); err != nil {
			t.Fatalf("create company evaluation: %v", err)
		}
		if _, err := workflow.CreateCompanyEvaluation(supervisor.ID, activeCase.ID, evaluationInput); !errors.Is(err, service.ErrDuplicateEvaluation) {
			t.Fatalf("duplicate evaluation error = %v", err)
		}

		file := &model.File{
			OriginalName: "final.pdf", StoredName: fmt.Sprintf("final-%d.pdf", suffix), Path: "/tmp/final.pdf",
			MimeType: "application/pdf", SizeBytes: 100, UploadedBy: student.ID, UploadedAt: time.Now(),
		}
		if _, _, err := workflow.AttachFinalReport(student.ID, file); err != nil {
			t.Fatalf("attach final report: %v", err)
		}
		if _, err := workflow.GetAccessibleFile(student.ID, model.RoleStudent, file.ID); err != nil {
			t.Fatalf("student access final report: %v", err)
		}
		if _, err := workflow.GetAccessibleFile(otherSupervisor.ID, model.RoleCompanySupervisor, file.ID); !errors.Is(err, service.ErrCaseAccessDenied) {
			t.Fatalf("unassigned file access error = %v", err)
		}
	})

	t.Run("reports require active internship", func(t *testing.T) {
		student := createTestStudent(t, tx, suffix, "inactive-reporting")
		inactiveCase := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusReadyToStart}
		if err := tx.Create(&inactiveCase).Error; err != nil {
			t.Fatalf("create inactive case: %v", err)
		}
		_, err := workflow.CreateWeeklyReport(student.ID, service.WeeklyReportInput{
			WeekNumber: 1, StartDate: testDate(t, "2026-07-11"), EndDate: testDate(t, "2026-07-17"), ActivityDescription: "test",
		})
		if !errors.Is(err, service.ErrInvalidCaseStatus) {
			t.Fatalf("inactive report error = %v", err)
		}
	})

	t.Run("professor final review validates readiness ownership and qualitative results", func(t *testing.T) {
		otherProfessor := createTestUser(t, tx, suffix, "other-professor", model.RoleProfessor)

		fewerReports := createProfessorReviewCase(t, tx, suffix+10, professor.ID, supervisor.ID, 7, 7, false, false)
		if _, err := workflow.CompleteProfessorCase(professor.ID, fewerReports.ID, service.ProfessorCompletionInput{
			Result: model.ProfessorFinalResultExcellent,
		}); !errors.Is(err, service.ErrProfessorWeeklyReportsIncomplete) {
			t.Fatalf("completion with fewer reports error = %v", err)
		}

		unconfirmed := createProfessorReviewCase(t, tx, suffix+20, professor.ID, supervisor.ID, 8, 7, false, false)
		if _, err := workflow.CompleteProfessorCase(professor.ID, unconfirmed.ID, service.ProfessorCompletionInput{
			Result: model.ProfessorFinalResultGood,
		}); !errors.Is(err, service.ErrProfessorWeeklyReportsIncomplete) {
			t.Fatalf("completion with an unconfirmed report error = %v", err)
		}

		withoutEvaluation := createProfessorReviewCase(t, tx, suffix+30, professor.ID, supervisor.ID, 8, 8, false, true)
		if _, err := workflow.CompleteProfessorCase(professor.ID, withoutEvaluation.ID, service.ProfessorCompletionInput{
			Result: model.ProfessorFinalResultFailed,
		}); !errors.Is(err, service.ErrProfessorCompanyEvaluationRequired) {
			t.Fatalf("completion without evaluation error = %v", err)
		}

		withoutFinalReport := createProfessorReviewCase(t, tx, suffix+40, professor.ID, supervisor.ID, 8, 8, true, false)
		if _, err := workflow.CompleteProfessorCase(professor.ID, withoutFinalReport.ID, service.ProfessorCompletionInput{
			Result: model.ProfessorFinalResultExcellent,
		}); !errors.Is(err, service.ErrProfessorFinalReportRequired) {
			t.Fatalf("completion without final report error = %v", err)
		}

		for _, invalid := range []model.ProfessorFinalResult{"18", "20", "AVERAGE", "WEAK", ""} {
			if _, err := workflow.CompleteProfessorCase(professor.ID, withoutFinalReport.ID, service.ProfessorCompletionInput{
				Result: invalid,
			}); !errors.Is(err, service.ErrInvalidProfessorResult) {
				t.Fatalf("invalid result %q error = %v", invalid, err)
			}
		}

		for index, result := range []model.ProfessorFinalResult{
			model.ProfessorFinalResultExcellent,
			model.ProfessorFinalResultGood,
			model.ProfessorFinalResultFailed,
		} {
			readyCase := createProfessorReviewCase(t, tx, suffix+100+int64(index), professor.ID, supervisor.ID, 8, 8, true, true)
			comment := fmt.Sprintf("professor comment %d", index)
			if _, err := workflow.GetProfessorCase(otherProfessor.ID, readyCase.ID); !errors.Is(err, service.ErrCaseAccessDenied) {
				t.Fatalf("other professor access error = %v", err)
			}
			completed, err := workflow.CompleteProfessorCase(professor.ID, readyCase.ID, service.ProfessorCompletionInput{
				Result: result, Comment: &comment,
			})
			if err != nil {
				t.Fatalf("complete case with %s: %v", result, err)
			}
			if completed.Status != model.InternshipCaseStatusCompleted || completed.FinalResult == nil ||
				*completed.FinalResult != result || completed.ProfessorComment == nil || *completed.ProfessorComment != comment ||
				completed.CompletedAt == nil {
				t.Fatalf("completion did not persist expected fields: %+v", completed)
			}
			if len(completed.WeeklyReports) != 8 || completed.CompanyEvaluation == nil || completed.FinalReportFile == nil {
				t.Fatalf("completed professor detail is missing review data: %+v", completed)
			}
			if _, err := workflow.GetAccessibleFile(professor.ID, model.RoleProfessor, completed.FinalReportFile.ID); err != nil {
				t.Fatalf("professor final report access: %v", err)
			}
			studentView, err := workflow.GetCurrentCase(readyCase.StudentID)
			if err != nil || studentView.FinalResult == nil || *studentView.FinalResult != result {
				t.Fatalf("student completed result: case=%+v err=%v", studentView, err)
			}
			if _, err := workflow.CompleteProfessorCase(professor.ID, readyCase.ID, service.ProfessorCompletionInput{
				Result: result,
			}); !errors.Is(err, service.ErrProfessorCaseNotActive) {
				t.Fatalf("duplicate completion error = %v", err)
			}

			if index == 0 {
				reports := completed.WeeklyReports
				if len(reports) != 8 {
					t.Fatalf("load completed reports: count=%d", len(reports))
				}
				if _, err := workflow.UpdateWeeklyReport(readyCase.StudentID, reports[0].ID, service.WeeklyReportInput{
					WeekNumber: 1, StartDate: testDate(t, "2026-07-01"), EndDate: testDate(t, "2026-07-07"), ActivityDescription: "changed",
				}); !errors.Is(err, service.ErrInvalidCaseStatus) {
					t.Fatalf("student mutation after completion error = %v", err)
				}
				if _, err := workflow.ConfirmWeeklyReport(supervisor.ID, readyCase.ID, reports[0].ID, nil); !errors.Is(err, service.ErrInvalidCaseStatus) {
					t.Fatalf("company report mutation after completion error = %v", err)
				}
				replacement := &model.File{
					OriginalName: "replacement.pdf", StoredName: fmt.Sprintf("replacement-%d.pdf", suffix), Path: "/tmp/replacement.pdf",
					MimeType: "application/pdf", SizeBytes: 100, UploadedBy: readyCase.StudentID, UploadedAt: time.Now(),
				}
				if _, _, err := workflow.AttachFinalReport(readyCase.StudentID, replacement); !errors.Is(err, service.ErrInvalidCaseStatus) {
					t.Fatalf("student final report mutation after completion error = %v", err)
				}
				if _, err := workflow.CreateCompanyEvaluation(supervisor.ID, readyCase.ID, service.CompanyEvaluationInput{
					AttendanceRating: model.EvaluationRatingGood, ParticipationRating: model.EvaluationRatingGood,
					LearningRating: model.EvaluationRatingGood, InterestRating: model.EvaluationRatingGood,
					PersistenceRating: model.EvaluationRatingGood, SuggestionRating: model.EvaluationRatingGood,
					ResourceUsageRating: model.EvaluationRatingGood, ReportQualityRating: model.EvaluationRatingGood,
					ProjectPerformanceRating: model.EvaluationRatingGood,
				}); !errors.Is(err, service.ErrInvalidCaseStatus) {
					t.Fatalf("company evaluation mutation after completion error = %v", err)
				}
				studentReports, err := workflow.ListStudentWeeklyReports(readyCase.StudentID)
				if err != nil || len(studentReports) != 8 {
					t.Fatalf("student completed report visibility: count=%d err=%v", len(studentReports), err)
				}
				companyCase, err := workflow.GetCompanyCase(supervisor.ID, readyCase.ID)
				if err != nil || companyCase.Status != model.InternshipCaseStatusCompleted {
					t.Fatalf("company completed case visibility: case=%+v err=%v", companyCase, err)
				}
				companyReports, err := workflow.ListCompanyWeeklyReports(supervisor.ID, readyCase.ID)
				if err != nil || len(companyReports) != 8 {
					t.Fatalf("company completed reports visibility: count=%d err=%v", len(companyReports), err)
				}
				if _, err := workflow.GetCompanyEvaluation(supervisor.ID, readyCase.ID); err != nil {
					t.Fatalf("company completed evaluation visibility: %v", err)
				}
				completedStatus := model.InternshipCaseStatusCompleted
				universityCases, err := workflow.ListUniversityCases(&completedStatus)
				if err != nil || len(universityCases) == 0 {
					t.Fatalf("university completed case list: count=%d err=%v", len(universityCases), err)
				}
				universityCase, err := workflow.GetUniversityCase(readyCase.ID)
				if err != nil || universityCase.Status != model.InternshipCaseStatusCompleted ||
					len(universityCase.WeeklyReports) != 8 || universityCase.CompanyEvaluation == nil ||
					universityCase.FinalReportFile == nil || universityCase.FinalResult == nil {
					t.Fatalf("university completed case detail: case=%+v err=%v", universityCase, err)
				}
				if _, err := workflow.ApproveUniversityCase(readyCase.ID); !errors.Is(err, service.ErrObsoleteWorkflow) {
					t.Fatalf("university completed mutation error = %v", err)
				}
			}
		}

		cases, err := workflow.ListProfessorCases(professor.ID)
		if err != nil || len(cases) == 0 {
			t.Fatalf("list professor cases: count=%d err=%v", len(cases), err)
		}
		otherCases, err := workflow.ListProfessorCases(otherProfessor.ID)
		if err != nil || len(otherCases) != 0 {
			t.Fatalf("other professor list: count=%d err=%v", len(otherCases), err)
		}
	})
}

func createProfessorReviewCase(
	t *testing.T,
	db *gorm.DB,
	suffix int64,
	professorID, supervisorID uint,
	reportCount, confirmedCount int,
	withEvaluation, withFinalReport bool,
) model.InternshipCase {
	t.Helper()
	student := createTestStudent(t, db, suffix, "professor-review")
	internshipCase := model.InternshipCase{
		StudentID: student.ID, ProfessorID: professorID, CompanySupervisorID: &supervisorID,
		Status: model.InternshipCaseStatusActive,
	}
	if err := db.Create(&internshipCase).Error; err != nil {
		t.Fatalf("create professor review case: %v", err)
	}
	for week := 1; week <= reportCount; week++ {
		report := model.WeeklyReport{
			InternshipCaseID: internshipCase.ID, WeekNumber: week,
			StartDate: testDate(t, "2026-07-01"), EndDate: testDate(t, "2026-07-07"),
			ActivityDescription: fmt.Sprintf("week %d", week), SubmittedAt: time.Now(), IsConfirmed: week <= confirmedCount,
		}
		if report.IsConfirmed {
			now := time.Now()
			report.ConfirmedAt = &now
		}
		if err := db.Create(&report).Error; err != nil {
			t.Fatalf("create professor review report %d: %v", week, err)
		}
	}
	if withEvaluation {
		evaluation := model.CompanyEvaluation{
			InternshipCaseID: internshipCase.ID, CompanySupervisorID: supervisorID,
			AttendanceRating: model.EvaluationRatingGood, ParticipationRating: model.EvaluationRatingGood,
			LearningRating: model.EvaluationRatingGood, InterestRating: model.EvaluationRatingGood,
			PersistenceRating: model.EvaluationRatingGood, SuggestionRating: model.EvaluationRatingGood,
			ResourceUsageRating: model.EvaluationRatingGood, ReportQualityRating: model.EvaluationRatingGood,
			ProjectPerformanceRating: model.EvaluationRatingGood, SubmittedAt: time.Now(),
		}
		if err := db.Create(&evaluation).Error; err != nil {
			t.Fatalf("create professor review evaluation: %v", err)
		}
	}
	if withFinalReport {
		file := model.File{
			OriginalName: "final.pdf", StoredName: fmt.Sprintf("professor-final-%d-%d.pdf", suffix, internshipCase.ID),
			Path: "/tmp/final.pdf", MimeType: "application/pdf", SizeBytes: 100,
			UploadedBy: student.ID, UploadedAt: time.Now(),
		}
		if err := db.Create(&file).Error; err != nil {
			t.Fatalf("create professor review file: %v", err)
		}
		if err := db.Model(&internshipCase).Update("final_report_file_id", file.ID).Error; err != nil {
			t.Fatalf("attach professor review file: %v", err)
		}
		internshipCase.FinalReportFileID = &file.ID
	}
	return internshipCase
}

func createTestUser(t *testing.T, db *gorm.DB, suffix int64, label string, role model.Role) model.User {
	t.Helper()
	user := model.User{
		FullName: label, Email: fmt.Sprintf("%s-%d@example.test", label, suffix),
		PasswordHash: "test", Role: role,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create %s: %v", label, err)
	}
	return user
}

func createTestStudent(t *testing.T, db *gorm.DB, suffix int64, label string) model.User {
	t.Helper()
	studentNumber := fmt.Sprintf("%d-%s", suffix, label)
	major := "مهندسی کامپیوتر"
	student := createTestUser(t, db, suffix, label, model.RoleStudent)
	if err := db.Model(&student).Updates(map[string]any{"student_number": studentNumber, "major": major}).Error; err != nil {
		t.Fatalf("complete student profile: %v", err)
	}
	return student
}

func createAssignment(t *testing.T, db *gorm.DB, studentID, professorID uint) {
	t.Helper()
	assignment := model.ProfessorAssignment{StudentID: studentID, ProfessorID: professorID, AssignedAt: time.Now()}
	if err := db.Create(&assignment).Error; err != nil {
		t.Fatalf("create professor assignment: %v", err)
	}
}

func createSubmittedCase(t *testing.T, db *gorm.DB, studentID, professorID uint) model.InternshipCase {
	t.Helper()
	now := time.Now()
	internshipCase := model.InternshipCase{
		StudentID: studentID, ProfessorID: professorID,
		Status: model.InternshipCaseStatusPendingUniversityReview, SubmittedAt: &now,
	}
	if err := db.Create(&internshipCase).Error; err != nil {
		t.Fatalf("create submitted case: %v", err)
	}
	return internshipCase
}

func assertCompanyCount(t *testing.T, db *gorm.DB, name string, expected int64) {
	t.Helper()
	var count int64
	if err := db.Model(&model.Company{}).Where("name = ?", name).Count(&count).Error; err != nil {
		t.Fatalf("count company %q: %v", name, err)
	}
	if count != expected {
		t.Fatalf("company %q count = %d, want %d", name, count, expected)
	}
}

func testDate(t *testing.T, value string) time.Time {
	t.Helper()
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatalf("parse test date: %v", err)
	}
	return date
}

func createWorkflowOpportunityApplication(t *testing.T, db *gorm.DB, suffix int64, studentID, creatorID uint, label string, status model.ApplicationStatus, opportunityStatus model.OpportunityStatus) model.OpportunityApplication {
	t.Helper()
	company := model.Company{
		Name:         fmt.Sprintf("workflow-company-%d-%s", suffix, label),
		NationalID:   fmt.Sprintf("wn-%d-%s", suffix, label),
		EconomicCode: fmt.Sprintf("we-%d-%s", suffix, label),
	}
	if err := db.Create(&company).Error; err != nil {
		t.Fatalf("create workflow company %s: %v", label, err)
	}
	opportunity := model.InternshipOpportunity{
		CompanyID: company.ID, CreatedBy: creatorID, Title: "فرصت آزمون", Description: "شرح",
		WorkField: "نرم‌افزار", Location: "تهران", Status: opportunityStatus,
	}
	if err := db.Create(&opportunity).Error; err != nil {
		t.Fatalf("create workflow opportunity %s: %v", label, err)
	}
	file := model.File{
		OriginalName: label + ".pdf", StoredName: fmt.Sprintf("workflow-%d-%s.pdf", suffix, label),
		Path: fmt.Sprintf("/tmp/workflow-%d-%s.pdf", suffix, label), MimeType: "application/pdf",
		SizeBytes: 100, UploadedBy: studentID, UploadedAt: time.Now(),
	}
	if err := db.Create(&file).Error; err != nil {
		t.Fatalf("create workflow resume %s: %v", label, err)
	}
	application := model.OpportunityApplication{
		OpportunityID: opportunity.ID, StudentID: studentID, ResumeFileID: file.ID,
		Status: status, AppliedAt: time.Now(),
	}
	if status != model.ApplicationStatusPending {
		now := time.Now()
		application.ReviewedAt = &now
	}
	if err := db.Create(&application).Error; err != nil {
		t.Fatalf("create workflow application %s: %v", label, err)
	}
	return application
}
