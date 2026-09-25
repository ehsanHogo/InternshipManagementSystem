package service

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"internship-management-system/backend/internal/model"
)

func TestOpportunityApplicationEligibilityReviewAndAuthorization(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect to integration database: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Company{}, &model.User{}, &model.InternshipOpportunity{}, &model.File{},
		&model.OpportunityApplication{}, &model.InternshipCase{},
	); err != nil {
		t.Fatalf("migrate integration database: %v", err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin test transaction: %v", tx.Error)
	}
	defer tx.Rollback()

	suffix := time.Now().UnixNano()
	companyA := applicationTestCompany(t, tx, suffix, "a")
	companyB := applicationTestCompany(t, tx, suffix, "b")
	supervisorA := applicationTestUser(t, tx, suffix, "supervisor-a", model.RoleCompanySupervisor, &companyA.ID)
	supervisorB := applicationTestUser(t, tx, suffix, "supervisor-b", model.RoleCompanySupervisor, &companyB.ID)
	professor := applicationTestUser(t, tx, suffix, "professor", model.RoleProfessor, nil)
	university := applicationTestUser(t, tx, suffix, "university", model.RoleUniversitySupervisor, nil)
	otherStudent := applicationTestUser(t, tx, suffix, "other-student", model.RoleStudent, nil)
	openOpportunity := applicationTestOpportunity(t, tx, suffix, "open", companyA.ID, supervisorA.ID, model.OpportunityStatusOpen)
	closedOpportunity := applicationTestOpportunity(t, tx, suffix, "closed", companyA.ID, supervisorA.ID, model.OpportunityStatusClosed)
	applications := NewOpportunityApplicationService(tx)
	files := NewInternshipService(tx)

	t.Run("no case succeeds with server controlled fields", func(t *testing.T) {
		student := applicationTestUser(t, tx, suffix, "no-case", model.RoleStudent, nil)
		application, err := applications.Apply(student.ID, openOpportunity.ID, applicationTestResume(suffix, "no-case"))
		if err != nil {
			t.Fatalf("apply without case: %v", err)
		}
		if application.StudentID != student.ID || application.OpportunityID != openOpportunity.ID ||
			application.Status != model.ApplicationStatusPending || application.ResumeFileID == 0 ||
			application.AppliedAt.IsZero() || application.ReviewedAt != nil {
			t.Fatalf("application fields are not server controlled: %+v", application)
		}
	})

	for _, status := range []model.InternshipCaseStatus{model.InternshipCaseStatusDraft, model.InternshipCaseStatusCancelled} {
		status := status
		t.Run(string(status)+" allows application", func(t *testing.T) {
			student := applicationTestUser(t, tx, suffix, "allowed-"+string(status), model.RoleStudent, nil)
			applicationTestCase(t, tx, student.ID, professor.ID, status)
			if _, err := applications.Apply(student.ID, openOpportunity.ID, applicationTestResume(suffix, "allowed-"+string(status))); err != nil {
				t.Fatalf("status %s should allow application: %v", status, err)
			}
		})
	}

	blockingStatuses := []model.InternshipCaseStatus{
		model.InternshipCaseStatusPendingUniversityReview,
		model.InternshipCaseStatusPendingCompanyDetails,
		model.InternshipCaseStatusPendingFinalApproval,
		model.InternshipCaseStatusReadyToStart,
		model.InternshipCaseStatusActive,
	}
	for _, status := range blockingStatuses {
		status := status
		t.Run(string(status)+" blocks application without persisted file", func(t *testing.T) {
			student := applicationTestUser(t, tx, suffix, "blocked-"+string(status), model.RoleStudent, nil)
			applicationTestCase(t, tx, student.ID, professor.ID, status)
			resume := applicationTestResume(suffix, "blocked-"+string(status))
			if _, err := applications.Apply(student.ID, openOpportunity.ID, resume); !errors.Is(err, ErrInternshipCaseAlreadyInProgress) {
				t.Fatalf("status %s error = %v", status, err)
			}
			assertNoApplicationOrFile(t, tx, student.ID, openOpportunity.ID, resume)
		})
	}

	t.Run("completed takes permanent priority", func(t *testing.T) {
		student := applicationTestUser(t, tx, suffix, "completed", model.RoleStudent, nil)
		applicationTestCase(t, tx, student.ID, professor.ID, model.InternshipCaseStatusCancelled)
		applicationTestCase(t, tx, student.ID, professor.ID, model.InternshipCaseStatusDraft)
		applicationTestCase(t, tx, student.ID, professor.ID, model.InternshipCaseStatusCompleted)
		resume := applicationTestResume(suffix, "completed")
		if _, err := applications.Apply(student.ID, openOpportunity.ID, resume); !errors.Is(err, ErrInternshipAlreadyCompleted) {
			t.Fatalf("completed case error = %v", err)
		}
		assertNoApplicationOrFile(t, tx, student.ID, openOpportunity.ID, resume)
	})

	t.Run("duplicate and closed opportunity are rejected cleanly", func(t *testing.T) {
		student := applicationTestUser(t, tx, suffix, "duplicates", model.RoleStudent, nil)
		duplicateOpportunity := applicationTestOpportunity(t, tx, suffix, "duplicate", companyA.ID, supervisorA.ID, model.OpportunityStatusOpen)
		if _, err := applications.Apply(student.ID, duplicateOpportunity.ID, applicationTestResume(suffix, "duplicate-first")); err != nil {
			t.Fatalf("first duplicate test application: %v", err)
		}
		secondResume := applicationTestResume(suffix, "duplicate-second")
		if _, err := applications.Apply(student.ID, duplicateOpportunity.ID, secondResume); !errors.Is(err, ErrApplicationAlreadyExists) {
			t.Fatalf("duplicate error = %v", err)
		}
		assertApplicationCount(t, tx, student.ID, duplicateOpportunity.ID, 1)
		if secondResume.ID != 0 {
			t.Fatalf("duplicate resume metadata was persisted: %+v", secondResume)
		}
		closedResume := applicationTestResume(suffix, "closed")
		if _, err := applications.Apply(student.ID, closedOpportunity.ID, closedResume); !errors.Is(err, ErrOpportunityNotOpen) {
			t.Fatalf("closed opportunity error = %v", err)
		}
		assertNoApplicationOrFile(t, tx, student.ID, closedOpportunity.ID, closedResume)
	})

	t.Run("company ownership review terminal decisions and resume access", func(t *testing.T) {
		student := applicationTestUser(t, tx, suffix, "review", model.RoleStudent, nil)
		reviewOpportunity := applicationTestOpportunity(t, tx, suffix, "review", companyA.ID, supervisorA.ID, model.OpportunityStatusOpen)
		application, err := applications.Apply(student.ID, reviewOpportunity.ID, applicationTestResume(suffix, "review"))
		if err != nil {
			t.Fatalf("create review application: %v", err)
		}
		if _, err := applications.GetStudent(otherStudent.ID, application.ID); !errors.Is(err, ErrOpportunityApplicationNotFound) {
			t.Fatalf("other student detail error = %v", err)
		}
		if _, err := applications.GetCompany(supervisorB.ID, application.ID); !errors.Is(err, ErrOpportunityApplicationNotFound) {
			t.Fatalf("cross-company detail error = %v", err)
		}
		if _, err := applications.Review(supervisorB.ID, application.ID, model.ApplicationStatusAccepted, nil); !errors.Is(err, ErrOpportunityApplicationNotFound) {
			t.Fatalf("cross-company review error = %v", err)
		}
		comment := "  رزومه مناسب است.  "
		accepted, err := applications.Review(supervisorA.ID, application.ID, model.ApplicationStatusAccepted, &comment)
		if err != nil {
			t.Fatalf("accept application: %v", err)
		}
		if accepted.Status != model.ApplicationStatusAccepted || accepted.ReviewedAt == nil ||
			accepted.CompanyComment == nil || *accepted.CompanyComment != "رزومه مناسب است." {
			t.Fatalf("accepted application is incomplete: %+v", accepted)
		}
		if _, err := applications.Review(supervisorA.ID, application.ID, model.ApplicationStatusRejected, nil); !errors.Is(err, ErrApplicationNotPending) {
			t.Fatalf("terminal decision change error = %v", err)
		}
		if _, err := files.GetAccessibleFile(student.ID, model.RoleStudent, accepted.ResumeFileID); err != nil {
			t.Fatalf("owning student resume access: %v", err)
		}
		if _, err := files.GetAccessibleFile(supervisorA.ID, model.RoleCompanySupervisor, accepted.ResumeFileID); err != nil {
			t.Fatalf("owning supervisor resume access: %v", err)
		}
		for label, principal := range map[string]struct {
			id   uint
			role model.Role
		}{
			"other student":    {otherStudent.ID, model.RoleStudent},
			"other supervisor": {supervisorB.ID, model.RoleCompanySupervisor},
			"professor":        {professor.ID, model.RoleProfessor},
			"university":       {university.ID, model.RoleUniversitySupervisor},
		} {
			if _, err := files.GetAccessibleFile(principal.id, principal.role, accepted.ResumeFileID); !errors.Is(err, ErrCaseAccessDenied) {
				t.Fatalf("%s resume access error = %v", label, err)
			}
		}
	})

	t.Run("rejection and closed opportunity existing application remain reviewable", func(t *testing.T) {
		studentRejected := applicationTestUser(t, tx, suffix, "rejected", model.RoleStudent, nil)
		rejectOpportunity := applicationTestOpportunity(t, tx, suffix, "reject", companyA.ID, supervisorA.ID, model.OpportunityStatusOpen)
		rejectedApplication, err := applications.Apply(studentRejected.ID, rejectOpportunity.ID, applicationTestResume(suffix, "reject"))
		if err != nil {
			t.Fatalf("create rejected application: %v", err)
		}
		rejected, err := applications.Review(supervisorA.ID, rejectedApplication.ID, model.ApplicationStatusRejected, nil)
		if err != nil || rejected.Status != model.ApplicationStatusRejected || rejected.ReviewedAt == nil {
			t.Fatalf("reject application = %+v, %v", rejected, err)
		}

		studentClosed := applicationTestUser(t, tx, suffix, "closed-existing", model.RoleStudent, nil)
		closeAfterApply := applicationTestOpportunity(t, tx, suffix, "close-after", companyA.ID, supervisorA.ID, model.OpportunityStatusOpen)
		existing, err := applications.Apply(studentClosed.ID, closeAfterApply.ID, applicationTestResume(suffix, "close-after"))
		if err != nil {
			t.Fatalf("apply before close: %v", err)
		}
		if err := tx.Model(&closeAfterApply).Update("status", model.OpportunityStatusClosed).Error; err != nil {
			t.Fatalf("close opportunity: %v", err)
		}
		if _, err := applications.GetCompany(supervisorA.ID, existing.ID); err != nil {
			t.Fatalf("view after close: %v", err)
		}
		if _, err := applications.Review(supervisorA.ID, existing.ID, model.ApplicationStatusAccepted, nil); err != nil {
			t.Fatalf("review after close: %v", err)
		}
	})
}

func applicationTestCompany(t *testing.T, db *gorm.DB, suffix int64, label string) model.Company {
	t.Helper()
	company := model.Company{Name: fmt.Sprintf("application-company-%d-%s", suffix, label), NationalID: fmt.Sprintf("application-national-%d-%s", suffix, label), EconomicCode: fmt.Sprintf("application-economic-%d-%s", suffix, label)}
	if err := db.Create(&company).Error; err != nil {
		t.Fatalf("create company: %v", err)
	}
	return company
}

func applicationTestUser(t *testing.T, db *gorm.DB, suffix int64, label string, role model.Role, companyID *uint) model.User {
	t.Helper()
	user := model.User{FullName: "کاربر آزمون", Email: fmt.Sprintf("application-%d-%s@example.test", suffix, label), PasswordHash: "unused", Role: role, CompanyID: companyID}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user %s: %v", label, err)
	}
	return user
}

func applicationTestOpportunity(t *testing.T, db *gorm.DB, suffix int64, label string, companyID, supervisorID uint, status model.OpportunityStatus) model.InternshipOpportunity {
	t.Helper()
	opportunity := model.InternshipOpportunity{CompanyID: companyID, CreatedBy: supervisorID, Title: fmt.Sprintf("فرصت %d %s", suffix, label), Description: "شرح", WorkField: "نرم‌افزار", Location: "تهران", Status: status}
	if err := db.Create(&opportunity).Error; err != nil {
		t.Fatalf("create opportunity %s: %v", label, err)
	}
	return opportunity
}

func applicationTestCase(t *testing.T, db *gorm.DB, studentID, professorID uint, status model.InternshipCaseStatus) {
	t.Helper()
	internshipCase := model.InternshipCase{StudentID: studentID, ProfessorID: professorID, Status: status}
	if err := db.Create(&internshipCase).Error; err != nil {
		t.Fatalf("create %s case: %v", status, err)
	}
}

func applicationTestResume(suffix int64, label string) *model.File {
	return &model.File{OriginalName: label + ".pdf", StoredName: fmt.Sprintf("%d-%s.pdf", suffix, label), Path: fmt.Sprintf("/tmp/%d-%s.pdf", suffix, label), MimeType: "application/pdf", SizeBytes: 128, UploadedAt: time.Now()}
}

func assertNoApplicationOrFile(t *testing.T, db *gorm.DB, studentID, opportunityID uint, resume *model.File) {
	t.Helper()
	assertApplicationCount(t, db, studentID, opportunityID, 0)
	if resume.ID != 0 {
		t.Fatalf("resume metadata was persisted: %+v", resume)
	}
	var count int64
	if err := db.Model(&model.File{}).Where("stored_name = ?", resume.StoredName).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("orphan resume count = %d, err = %v", count, err)
	}
}

func assertApplicationCount(t *testing.T, db *gorm.DB, studentID, opportunityID uint, expected int64) {
	t.Helper()
	var count int64
	if err := db.Model(&model.OpportunityApplication{}).Where("student_id = ? AND opportunity_id = ?", studentID, opportunityID).Count(&count).Error; err != nil || count != expected {
		t.Fatalf("application count = %d, expected = %d, err = %v", count, expected, err)
	}
}
