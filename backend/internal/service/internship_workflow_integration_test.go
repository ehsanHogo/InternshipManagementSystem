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
		&model.InternshipCase{}, &model.InternshipPreference{},
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
