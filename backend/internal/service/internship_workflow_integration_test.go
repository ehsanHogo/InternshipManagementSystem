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
		&model.User{}, &model.Company{}, &model.ProfessorAssignment{},
		&model.File{}, &model.InternshipCase{}, &model.InternshipPreference{},
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

	professor := createTestUser(t, tx, suffix, "professor", model.RoleProfessor)
	supervisor := createTestUser(t, tx, suffix, "supervisor", model.RoleCompanySupervisor)
	otherSupervisor := createTestUser(t, tx, suffix, "other-supervisor", model.RoleCompanySupervisor)
	existingCompany := model.Company{Name: fmt.Sprintf("existing-%d", suffix), IsApproved: true}
	if err := tx.Create(&existingCompany).Error; err != nil {
		t.Fatalf("create existing company: %v", err)
	}

	t.Run("student submission uses pending university approval", func(t *testing.T) {
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
		preference := model.InternshipPreference{
			InternshipCaseID: internshipCase.ID, Priority: 1, CompanyID: &existingCompany.ID,
			City: "تهران", WorkField: "نرم افزار",
		}
		if err := tx.Create(&preference).Error; err != nil {
			t.Fatalf("create draft preference: %v", err)
		}
		submitted, err := workflow.SubmitCase(student.ID)
		if err != nil {
			t.Fatalf("submit case: %v", err)
		}
		if submitted.Status != model.InternshipCaseStatusPendingUniversityApproval {
			t.Fatalf("submission status = %s, want %s", submitted.Status, model.InternshipCaseStatusPendingUniversityApproval)
		}
	})

	t.Run("proposed company is created only on company confirmation", func(t *testing.T) {
		student := createTestStudent(t, tx, suffix, "proposed")
		proposedName := fmt.Sprintf("proposed-%d", suffix)
		proposedPhone := "02100000000"
		internshipCase := createSubmittedCase(t, tx, student.ID, professor.ID)
		preference := model.InternshipPreference{
			InternshipCaseID: internshipCase.ID, Priority: 1, ProposedCompanyName: &proposedName,
			ProposedPhone: &proposedPhone, City: "تهران", WorkField: "فناوری",
		}
		if err := tx.Create(&preference).Error; err != nil {
			t.Fatalf("create proposed preference: %v", err)
		}

		sent, err := workflow.SendToCompany(internshipCase.ID, service.SendToCompanyInput{
			PreferenceID: preference.ID, CompanySupervisorID: supervisor.ID,
			LetterNumber: "1405/test", LetterDate: testDate(t, "2026-07-06"),
		})
		if err != nil {
			t.Fatalf("send proposed preference to company: %v", err)
		}
		if sent.Status != model.InternshipCaseStatusPendingCompanyApproval {
			t.Fatalf("sent status = %s", sent.Status)
		}
		assertCompanyCount(t, tx, proposedName, 0)
		if _, err := workflow.GetCompanyCase(otherSupervisor.ID, internshipCase.ID); !errors.Is(err, service.ErrCaseAccessDenied) {
			t.Fatalf("unassigned supervisor error = %v, want access denied", err)
		}

		confirmed, err := workflow.ConfirmCompanyCase(supervisor.ID, internshipCase.ID, service.CompanyConfirmationInput{
			InternshipSubject: "توسعه وب", StartDate: testDate(t, "2026-07-11"),
			WorkplaceAddress: "تهران", WorkplacePhone: "02112345678",
		})
		if err != nil {
			t.Fatalf("confirm proposed company placement: %v", err)
		}
		if confirmed.Status != model.InternshipCaseStatusCompanyApproved || confirmed.SelectedPreference == nil ||
			confirmed.SelectedPreference.CompanyID == nil || confirmed.CompanyConfirmedAt == nil {
			t.Fatalf("confirmation did not persist expected state: %+v", confirmed)
		}
		assertCompanyCount(t, tx, proposedName, 1)
		var refreshedSupervisor model.User
		if err := tx.First(&refreshedSupervisor, supervisor.ID).Error; err != nil {
			t.Fatalf("reload supervisor: %v", err)
		}
		if refreshedSupervisor.CompanyID == nil || *refreshedSupervisor.CompanyID != *confirmed.SelectedPreference.CompanyID {
			t.Fatalf("supervisor was not associated with proposed company")
		}
		if _, err := workflow.ConfirmCompanyCase(supervisor.ID, internshipCase.ID, service.CompanyConfirmationInput{
			InternshipSubject: "تکرار", StartDate: testDate(t, "2026-07-11"),
			WorkplaceAddress: "تهران", WorkplacePhone: "02112345678",
		}); !errors.Is(err, service.ErrInvalidTransition) {
			t.Fatalf("repeated confirmation error = %v, want invalid transition", err)
		}
		assertCompanyCount(t, tx, proposedName, 1)

		approved, err := workflow.ApproveUniversityCase(internshipCase.ID)
		if err != nil || approved.Status != model.InternshipCaseStatusUniversityApproved || approved.UniversityApprovedAt == nil {
			t.Fatalf("university approval failed: case=%+v err=%v", approved, err)
		}
		activated, err := workflow.ActivateUniversityCase(internshipCase.ID)
		if err != nil || activated.Status != model.InternshipCaseStatusActive || activated.ActivatedAt == nil {
			t.Fatalf("activation failed: case=%+v err=%v", activated, err)
		}
	})

	t.Run("existing company confirmation does not create a duplicate", func(t *testing.T) {
		student := createTestStudent(t, tx, suffix, "existing")
		internshipCase := createSubmittedCase(t, tx, student.ID, professor.ID)
		preference := model.InternshipPreference{
			InternshipCaseID: internshipCase.ID, Priority: 1, CompanyID: &existingCompany.ID,
			City: "تهران", WorkField: "فناوری",
		}
		if err := tx.Create(&preference).Error; err != nil {
			t.Fatalf("create existing-company preference: %v", err)
		}
		if _, err := workflow.SendToCompany(internshipCase.ID, service.SendToCompanyInput{
			PreferenceID: preference.ID, CompanySupervisorID: supervisor.ID,
			LetterNumber: "1405/existing", LetterDate: testDate(t, "2026-07-06"),
		}); err != nil {
			t.Fatalf("send existing company preference: %v", err)
		}
		assertCompanyCount(t, tx, existingCompany.Name, 1)
		if _, err := workflow.ConfirmCompanyCase(supervisor.ID, internshipCase.ID, service.CompanyConfirmationInput{
			InternshipSubject: "تحلیل داده", StartDate: testDate(t, "2026-07-11"),
			WorkplaceAddress: "تهران", WorkplacePhone: "02112345678",
		}); err != nil {
			t.Fatalf("confirm existing company placement: %v", err)
		}
		assertCompanyCount(t, tx, existingCompany.Name, 1)
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
		inactiveCase := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusUniversityApproved}
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
		Status: model.InternshipCaseStatusPendingUniversityApproval, SubmittedAt: &now,
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
