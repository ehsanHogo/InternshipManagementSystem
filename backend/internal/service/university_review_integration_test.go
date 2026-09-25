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

func TestUniversityReviewWorkflow(t *testing.T) {
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
	); err != nil {
		t.Fatalf("migrate university review schema: %v", err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin test transaction: %v", tx.Error)
	}
	defer tx.Rollback()

	suffix := time.Now().UnixNano()
	company := applicationTestCompany(t, tx, suffix, "r")
	otherCompany := applicationTestCompany(t, tx, suffix, "o")
	supervisor := applicationTestUser(t, tx, suffix, "review-supervisor", model.RoleCompanySupervisor, &company.ID)
	otherSupervisor := applicationTestUser(t, tx, suffix, "review-other-supervisor", model.RoleCompanySupervisor, &otherCompany.ID)
	professor := applicationTestUser(t, tx, suffix, "review-professor", model.RoleProfessor, nil)
	workflow := NewInternshipService(tx)
	applications := NewOpportunityApplicationService(tx)
	fixture := 0

	type submittedFixture struct {
		student      model.User
		internship   model.InternshipCase
		preferences  []model.InternshipPreference
		applications []model.OpportunityApplication
	}

	createApplication := func(t *testing.T, student model.User, creatorID, companyID uint, status model.ApplicationStatus) model.OpportunityApplication {
		t.Helper()
		fixture++
		label := fmt.Sprintf("review-%d", fixture)
		opportunity := applicationTestOpportunity(t, tx, suffix, label, companyID, creatorID, model.OpportunityStatusOpen)
		application, err := applications.Apply(student.ID, opportunity.ID, applicationTestResume(suffix, label))
		if err != nil {
			t.Fatalf("apply for fixture: %v", err)
		}
		if status != model.ApplicationStatusPending {
			application, err = applications.Review(supervisor.ID, application.ID, status, nil)
			if err != nil {
				t.Fatalf("review fixture application: %v", err)
			}
		}
		return *application
	}

	createSubmitted := func(t *testing.T, label string, count int) submittedFixture {
		t.Helper()
		student := applicationTestUser(t, tx, suffix, label, model.RoleStudent, nil)
		assignment := model.ProfessorAssignment{StudentID: student.ID, ProfessorID: professor.ID, AssignedAt: time.Now()}
		if err := tx.Create(&assignment).Error; err != nil {
			t.Fatalf("create assignment: %v", err)
		}
		credits, mobile, submittedAt := 90, "09120000000", time.Now()
		internshipCase := model.InternshipCase{
			StudentID: student.ID, ProfessorID: professor.ID,
			Status:        model.InternshipCaseStatusDraft,
			PassedCredits: &credits, Mobile: &mobile,
		}
		if err := tx.Create(&internshipCase).Error; err != nil {
			t.Fatalf("create submitted case: %v", err)
		}
		result := submittedFixture{student: student, internship: internshipCase}
		for priority := 1; priority <= count; priority++ {
			application := createApplication(t, student, supervisor.ID, company.ID, model.ApplicationStatusAccepted)
			preference := model.InternshipPreference{
				InternshipCaseID: internshipCase.ID, OpportunityApplicationID: application.ID, Priority: priority,
			}
			if err := tx.Create(&preference).Error; err != nil {
				t.Fatalf("create preference: %v", err)
			}
			result.applications = append(result.applications, application)
			result.preferences = append(result.preferences, preference)
		}
		if err := tx.Model(&internshipCase).Updates(map[string]any{
			"status":       model.InternshipCaseStatusPendingUniversityReview,
			"submitted_at": submittedAt,
		}).Error; err != nil {
			t.Fatalf("submit fixture case: %v", err)
		}
		internshipCase.Status = model.InternshipCaseStatusPendingUniversityReview
		internshipCase.SubmittedAt = &submittedAt
		return result
	}

	assertPendingUnchanged := func(t *testing.T, caseID uint) {
		t.Helper()
		var persisted model.InternshipCase
		if err := tx.First(&persisted, caseID).Error; err != nil {
			t.Fatalf("reload unchanged case: %v", err)
		}
		if persisted.Status != model.InternshipCaseStatusPendingUniversityReview ||
			persisted.SelectedPreferenceID != nil || persisted.LetterNumber != nil ||
			persisted.LetterDate != nil || persisted.CompanySupervisorID != nil {
			t.Fatalf("case changed after rejected review: %+v", persisted)
		}
	}

	for _, priority := range []int{1, 2, 3} {
		priority := priority
		t.Run(fmt.Sprintf("university may select priority %d atomically", priority), func(t *testing.T) {
			fixture := createSubmitted(t, fmt.Sprintf("review-priority-%d", priority), 3)
			pending, err := workflow.ListPendingUniversityReviewCases()
			if err != nil {
				t.Fatalf("list pending university cases: %v", err)
			}
			listed := false
			for _, pendingCase := range pending {
				if pendingCase.ID == fixture.internship.ID {
					listed = true
					break
				}
			}
			if !listed {
				t.Fatalf("submitted case %d missing from pending review list", fixture.internship.ID)
			}
			beforeCompanyID := *supervisor.CompanyID
			letterDate := time.Date(2026, time.September, 25, 0, 0, 0, 0, time.UTC)
			approved, err := workflow.ApproveUniversityPlacement(fixture.internship.ID, UniversityPlacementApprovalInput{
				PreferenceID: fixture.preferences[priority-1].ID,
				LetterNumber: " 1405/12345 ",
				LetterDate:   letterDate,
			})
			if err != nil {
				t.Fatalf("approve priority %d: %v", priority, err)
			}
			if approved.Status != model.InternshipCaseStatusPendingCompanyDetails ||
				approved.SelectedPreferenceID == nil || *approved.SelectedPreferenceID != fixture.preferences[priority-1].ID ||
				approved.CompanySupervisorID == nil || *approved.CompanySupervisorID != supervisor.ID ||
				approved.LetterNumber == nil || *approved.LetterNumber != "1405/12345" ||
				approved.LetterDate == nil || !approved.LetterDate.Equal(letterDate) {
				t.Fatalf("approved case mismatch: %+v", approved)
			}
			studentView, err := workflow.GetCurrentCase(fixture.student.ID)
			if err != nil || studentView.Status != model.InternshipCaseStatusPendingCompanyDetails ||
				studentView.SelectedPreference == nil || studentView.SelectedPreference.ID != fixture.preferences[priority-1].ID ||
				studentView.LetterNumber == nil || *studentView.LetterNumber != "1405/12345" ||
				studentView.LetterDate == nil || !studentView.LetterDate.Equal(letterDate) {
				t.Fatalf("student approved-state view = %+v err=%v", studentView, err)
			}
			var persistedSupervisor model.User
			if err := tx.First(&persistedSupervisor, supervisor.ID).Error; err != nil ||
				persistedSupervisor.CompanyID == nil || *persistedSupervisor.CompanyID != beforeCompanyID {
				t.Fatalf("supervisor company membership changed: user=%+v err=%v", persistedSupervisor, err)
			}
			if _, err := workflow.UpdateCase(fixture.student.ID, nil, nil); !errors.Is(err, ErrCaseNotEditable) {
				t.Fatalf("approved case student edit error = %v", err)
			}
			newOpportunity := applicationTestOpportunity(t, tx, suffix, fmt.Sprintf("locked-%d", priority), company.ID, supervisor.ID, model.OpportunityStatusOpen)
			if _, err := workflow.ReplacePreferences(fixture.student.ID, []uint{fixture.applications[0].ID}); !errors.Is(err, ErrCaseNotEditable) {
				t.Fatalf("approved case preference edit error = %v", err)
			}
			if _, err := workflow.ApproveUniversityPlacement(fixture.internship.ID, UniversityPlacementApprovalInput{
				PreferenceID: fixture.preferences[0].ID,
				LetterNumber: "second",
				LetterDate:   time.Now(),
			}); !errors.Is(err, ErrCaseNotPendingUniversityReview) {
				t.Fatalf("repeated approval error = %v", err)
			}
			if _, err := applications.Apply(fixture.student.ID, newOpportunity.ID, applicationTestResume(suffix, fmt.Sprintf("locked-%d", priority))); !errors.Is(err, ErrInternshipCaseAlreadyInProgress) {
				t.Fatalf("approved case recruitment lock error = %v", err)
			}
		})
	}

	t.Run("cross-case preference is rejected without mutation", func(t *testing.T) {
		first := createSubmitted(t, "review-cross-first", 1)
		second := createSubmitted(t, "review-cross-second", 1)
		_, err := workflow.ApproveUniversityPlacement(first.internship.ID, UniversityPlacementApprovalInput{
			PreferenceID: second.preferences[0].ID, LetterNumber: "1", LetterDate: time.Now(),
		})
		if !errors.Is(err, ErrPreferenceNotInCase) {
			t.Fatalf("cross-case error = %v", err)
		}
		assertPendingUnchanged(t, first.internship.ID)
	})

	t.Run("non-accepted application is rejected without mutation", func(t *testing.T) {
		item := createSubmitted(t, "review-corrupt-status", 1)
		if err := tx.Model(&model.OpportunityApplication{}).Where("id = ?", item.applications[0].ID).
			Update("status", model.ApplicationStatusRejected).Error; err != nil {
			t.Fatalf("corrupt application status: %v", err)
		}
		_, err := workflow.ApproveUniversityPlacement(item.internship.ID, UniversityPlacementApprovalInput{
			PreferenceID: item.preferences[0].ID, LetterNumber: "1", LetterDate: time.Now(),
		})
		if !errors.Is(err, ErrPreferenceNotAccepted) {
			t.Fatalf("non-accepted error = %v", err)
		}
		assertPendingUnchanged(t, item.internship.ID)
	})

	t.Run("letter metadata is required before transaction", func(t *testing.T) {
		item := createSubmitted(t, "review-letter-validation", 1)
		for _, input := range []UniversityPlacementApprovalInput{
			{PreferenceID: item.preferences[0].ID, LetterNumber: "   ", LetterDate: time.Now()},
			{PreferenceID: item.preferences[0].ID, LetterNumber: "1"},
		} {
			if _, err := workflow.ApproveUniversityPlacement(item.internship.ID, input); err == nil {
				t.Fatal("invalid letter input was accepted")
			}
			assertPendingUnchanged(t, item.internship.ID)
		}
	})

	t.Run("invalid opportunity creator relationship is rejected", func(t *testing.T) {
		item := createSubmitted(t, "review-invalid-supervisor", 1)
		application := item.applications[0]
		if err := tx.Model(&model.InternshipOpportunity{}).Where("id = ?", application.OpportunityID).
			Update("created_by", otherSupervisor.ID).Error; err != nil {
			t.Fatalf("corrupt opportunity creator: %v", err)
		}
		_, err := workflow.ApproveUniversityPlacement(item.internship.ID, UniversityPlacementApprovalInput{
			PreferenceID: item.preferences[0].ID, LetterNumber: "1", LetterDate: time.Now(),
		})
		if !errors.Is(err, ErrCompanySupervisorResolution) {
			t.Fatalf("invalid supervisor relationship error = %v", err)
		}
		assertPendingUnchanged(t, item.internship.ID)
	})

	t.Run("cancellation preserves history and reopens recruitment and case creation", func(t *testing.T) {
		item := createSubmitted(t, "review-cancel", 2)
		blockedOpportunity := applicationTestOpportunity(t, tx, suffix, "review-before-cancel", company.ID, supervisor.ID, model.OpportunityStatusOpen)
		if _, err := applications.Apply(item.student.ID, blockedOpportunity.ID, applicationTestResume(suffix, "review-before-cancel")); !errors.Is(err, ErrInternshipCaseAlreadyInProgress) {
			t.Fatalf("pre-cancellation recruitment error = %v", err)
		}
		if _, err := workflow.CancelUniversityReview(item.internship.ID, "   "); !errors.Is(err, ErrCancellationCommentRequired) {
			t.Fatalf("blank cancellation error = %v", err)
		}
		assertPendingUnchanged(t, item.internship.ID)

		cancelled, err := workflow.CancelUniversityReview(item.internship.ID, "  هیچ اولویتی مناسب نیست.  ")
		if err != nil || cancelled.Status != model.InternshipCaseStatusCancelled ||
			cancelled.CancellationComment == nil || *cancelled.CancellationComment != "هیچ اولویتی مناسب نیست." ||
			cancelled.CancelledAt == nil || cancelled.SelectedPreferenceID != nil {
			t.Fatalf("cancelled case = %+v err=%v", cancelled, err)
		}
		if len(cancelled.Preferences) != 2 || cancelled.Preferences[0].Priority != 1 || cancelled.Preferences[1].Priority != 2 {
			t.Fatalf("cancelled preferences not preserved: %+v", cancelled.Preferences)
		}
		for index, application := range item.applications {
			var persisted model.OpportunityApplication
			if err := tx.First(&persisted, application.ID).Error; err != nil || persisted.Status != model.ApplicationStatusAccepted {
				t.Fatalf("application %d changed: %+v err=%v", index, persisted, err)
			}
		}
		if _, err := workflow.CancelUniversityReview(item.internship.ID, "again"); !errors.Is(err, ErrCaseNotPendingUniversityReview) {
			t.Fatalf("repeated cancellation error = %v", err)
		}

		openOpportunity := applicationTestOpportunity(t, tx, suffix, "review-after-cancel", company.ID, supervisor.ID, model.OpportunityStatusOpen)
		if _, err := applications.Apply(item.student.ID, openOpportunity.ID, applicationTestResume(suffix, "review-after-cancel")); err != nil {
			t.Fatalf("recruitment did not reopen: %v", err)
		}
		current, err := workflow.GetCurrentCase(item.student.ID)
		if err != nil || current.ID != item.internship.ID || current.Status != model.InternshipCaseStatusCancelled {
			t.Fatalf("student cannot view cancelled case: case=%+v err=%v", current, err)
		}
		newCase, created, err := workflow.CreateOrGetCase(item.student.ID)
		if err != nil || !created || newCase.ID == item.internship.ID || newCase.Status != model.InternshipCaseStatusDraft {
			t.Fatalf("new draft after cancellation: case=%+v created=%v err=%v", newCase, created, err)
		}
		if _, err := workflow.ReplacePreferences(item.student.ID, []uint{item.applications[0].ID}); err != nil {
			t.Fatalf("accepted application reuse after cancellation: %v", err)
		}
		var oldPreferenceCount int64
		if err := tx.Model(&model.InternshipPreference{}).Where("internship_case_id = ?", item.internship.ID).Count(&oldPreferenceCount).Error; err != nil || oldPreferenceCount != 2 {
			t.Fatalf("old preference history count=%d err=%v", oldPreferenceCount, err)
		}
	})

	t.Run("cancellation is allowed only during university review", func(t *testing.T) {
		statuses := []model.InternshipCaseStatus{
			model.InternshipCaseStatusDraft,
			model.InternshipCaseStatusPendingCompanyDetails,
			model.InternshipCaseStatusPendingFinalApproval,
			model.InternshipCaseStatusReadyToStart,
			model.InternshipCaseStatusActive,
			model.InternshipCaseStatusCompleted,
			model.InternshipCaseStatusCancelled,
		}
		for index, status := range statuses {
			student := applicationTestUser(t, tx, suffix, fmt.Sprintf("review-invalid-cancel-%d", index), model.RoleStudent, nil)
			internshipCase := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, Status: status}
			if err := tx.Create(&internshipCase).Error; err != nil {
				t.Fatalf("create %s case: %v", status, err)
			}
			if _, err := workflow.CancelUniversityReview(internshipCase.ID, "reason"); !errors.Is(err, ErrCaseNotPendingUniversityReview) {
				t.Fatalf("cancel from %s error = %v", status, err)
			}
		}
	})
}
