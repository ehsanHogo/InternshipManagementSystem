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

func TestOfficialInternshipApplicationFlow(t *testing.T) {
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
		t.Fatalf("migrate official application schema: %v", err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin test transaction: %v", tx.Error)
	}
	defer tx.Rollback()

	suffix := time.Now().UnixNano()
	company := applicationTestCompany(t, tx, suffix, "official")
	supervisor := applicationTestUser(t, tx, suffix, "official-supervisor", model.RoleCompanySupervisor, &company.ID)
	professor := applicationTestUser(t, tx, suffix, "official-professor", model.RoleProfessor, nil)
	internships := NewInternshipService(tx)
	applications := NewOpportunityApplicationService(tx)
	fixtureNumber := 0

	createStudentCase := func(t *testing.T, label string, withDetails bool) (model.User, model.InternshipCase) {
		t.Helper()
		student := applicationTestUser(t, tx, suffix, label, model.RoleStudent, nil)
		assignment := model.ProfessorAssignment{StudentID: student.ID, ProfessorID: professor.ID, AssignedAt: time.Now()}
		if err := tx.Create(&assignment).Error; err != nil {
			t.Fatalf("create assignment %s: %v", label, err)
		}
		internshipCase := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusDraft}
		if withDetails {
			credits, mobile := 80, "09120000000"
			internshipCase.PassedCredits, internshipCase.Mobile = &credits, &mobile
		}
		if err := tx.Create(&internshipCase).Error; err != nil {
			t.Fatalf("create draft %s: %v", label, err)
		}
		return student, internshipCase
	}
	createApplication := func(t *testing.T, student model.User, status model.ApplicationStatus, opportunityStatus model.OpportunityStatus) model.OpportunityApplication {
		t.Helper()
		fixtureNumber++
		label := fmt.Sprintf("official-%d", fixtureNumber)
		opportunity := applicationTestOpportunity(t, tx, suffix, label, company.ID, supervisor.ID, model.OpportunityStatusOpen)
		application, err := applications.Apply(student.ID, opportunity.ID, applicationTestResume(suffix, label))
		if err != nil {
			t.Fatalf("apply for %s: %v", label, err)
		}
		if status != model.ApplicationStatusPending {
			application, err = applications.Review(supervisor.ID, application.ID, status, nil)
			if err != nil {
				t.Fatalf("review %s as %s: %v", label, status, err)
			}
		}
		if opportunityStatus == model.OpportunityStatusClosed {
			if err := tx.Model(&opportunity).Update("status", model.OpportunityStatusClosed).Error; err != nil {
				t.Fatalf("close %s: %v", label, err)
			}
		}
		return *application
	}

	t.Run("accepted applications are filtered and validated by status and ownership", func(t *testing.T) {
		student, _ := createStudentCase(t, "official-eligibility", true)
		accepted := createApplication(t, student, model.ApplicationStatusAccepted, model.OpportunityStatusOpen)
		pending := createApplication(t, student, model.ApplicationStatusPending, model.OpportunityStatusOpen)
		rejected := createApplication(t, student, model.ApplicationStatusRejected, model.OpportunityStatusOpen)
		otherStudent := applicationTestUser(t, tx, suffix, "official-other-owner", model.RoleStudent, nil)
		otherAccepted := createApplication(t, otherStudent, model.ApplicationStatusAccepted, model.OpportunityStatusOpen)

		eligible, err := applications.ListAcceptedStudent(student.ID)
		if err != nil || len(eligible) != 1 || eligible[0].ID != accepted.ID {
			t.Fatalf("accepted list = %+v, err = %v", eligible, err)
		}
		if _, err := internships.AddPreference(student.ID, PreferenceInput{Priority: 1, OpportunityApplicationID: accepted.ID}); err != nil {
			t.Fatalf("accepted preference: %v", err)
		}
		if _, err := internships.AddPreference(student.ID, PreferenceInput{Priority: 2, OpportunityApplicationID: pending.ID}); !errors.Is(err, ErrPreferenceNotAccepted) {
			t.Fatalf("pending preference error = %v", err)
		}
		if _, err := internships.AddPreference(student.ID, PreferenceInput{Priority: 2, OpportunityApplicationID: rejected.ID}); !errors.Is(err, ErrPreferenceNotAccepted) {
			t.Fatalf("rejected preference error = %v", err)
		}
		if _, err := internships.AddPreference(student.ID, PreferenceInput{Priority: 2, OpportunityApplicationID: otherAccepted.ID}); !errors.Is(err, ErrPreferenceNotOwned) {
			t.Fatalf("other owner preference error = %v", err)
		}
	})

	t.Run("one to three contiguous unique preferences are enforced", func(t *testing.T) {
		student, _ := createStudentCase(t, "official-limits", true)
		accepted := make([]model.OpportunityApplication, 4)
		for index := range accepted {
			accepted[index] = createApplication(t, student, model.ApplicationStatusAccepted, model.OpportunityStatusOpen)
		}
		for count := 1; count <= 3; count++ {
			ids := make([]uint, count)
			for index := range ids {
				ids[index] = accepted[index].ID
			}
			updated, err := internships.ReplacePreferences(student.ID, ids)
			if err != nil || len(updated.Preferences) != count {
				t.Fatalf("replace with %d preferences: case=%+v err=%v", count, updated, err)
			}
			for index, preference := range updated.Preferences {
				if preference.Priority != index+1 {
					t.Fatalf("priority %d = %d", index, preference.Priority)
				}
			}
		}
		if _, err := internships.ReplacePreferences(student.ID, []uint{accepted[0].ID, accepted[1].ID, accepted[2].ID, accepted[3].ID}); !errors.Is(err, ErrPreferenceLimit) {
			t.Fatalf("fourth preference error = %v", err)
		}
		if _, err := internships.ReplacePreferences(student.ID, []uint{accepted[0].ID, accepted[0].ID}); !errors.Is(err, ErrPreferenceAlreadyExists) {
			t.Fatalf("duplicate application error = %v", err)
		}

		single, err := internships.ReplacePreferences(student.ID, []uint{accepted[0].ID})
		if err != nil {
			t.Fatalf("prepare database uniqueness check: %v", err)
		}
		if err := tx.SavePoint("official_duplicate_application").Error; err != nil {
			t.Fatalf("create duplicate application savepoint: %v", err)
		}
		duplicate := model.InternshipPreference{InternshipCaseID: single.ID, OpportunityApplicationID: accepted[0].ID, Priority: 2}
		duplicateErr := tx.Create(&duplicate).Error
		if err := tx.RollbackTo("official_duplicate_application").Error; err != nil {
			t.Fatalf("rollback duplicate application: %v", err)
		}
		if duplicateErr == nil {
			t.Fatal("expected database case-application uniqueness error")
		}

		priorityStudent, _ := createStudentCase(t, "official-duplicate-priority", true)
		first := createApplication(t, priorityStudent, model.ApplicationStatusAccepted, model.OpportunityStatusOpen)
		second := createApplication(t, priorityStudent, model.ApplicationStatusAccepted, model.OpportunityStatusOpen)
		if _, err := internships.AddPreference(priorityStudent.ID, PreferenceInput{Priority: 1, OpportunityApplicationID: first.ID}); err != nil {
			t.Fatalf("first priority: %v", err)
		}
		if _, err := internships.AddPreference(priorityStudent.ID, PreferenceInput{Priority: 1, OpportunityApplicationID: second.ID}); !errors.Is(err, ErrDuplicatePriority) {
			t.Fatalf("duplicate priority error = %v", err)
		}
		if _, err := internships.AddPreference(priorityStudent.ID, PreferenceInput{Priority: 2, OpportunityApplicationID: first.ID}); !errors.Is(err, ErrPreferenceAlreadyExists) {
			t.Fatalf("duplicate selected application error = %v", err)
		}
	})

	t.Run("draft information and preferences are editable", func(t *testing.T) {
		student, _ := createStudentCase(t, "official-draft-edit", false)
		credits, mobile := 75, "09123334444"
		updatedCase, err := internships.UpdateCase(student.ID, &credits, &mobile)
		if err != nil || updatedCase.PassedCredits == nil || *updatedCase.PassedCredits != credits || updatedCase.Mobile == nil || *updatedCase.Mobile != mobile {
			t.Fatalf("update draft information: case=%+v err=%v", updatedCase, err)
		}
		first := createApplication(t, student, model.ApplicationStatusAccepted, model.OpportunityStatusOpen)
		second := createApplication(t, student, model.ApplicationStatusAccepted, model.OpportunityStatusOpen)
		if _, err := internships.AddPreference(student.ID, PreferenceInput{Priority: 1, OpportunityApplicationID: first.ID}); err != nil {
			t.Fatalf("add first draft preference: %v", err)
		}
		secondPreference, err := internships.AddPreference(student.ID, PreferenceInput{Priority: 2, OpportunityApplicationID: second.ID})
		if err != nil {
			t.Fatalf("add second draft preference: %v", err)
		}
		if _, err := internships.UpdatePreference(student.ID, secondPreference.ID, PreferenceInput{Priority: 3, OpportunityApplicationID: second.ID}); err != nil {
			t.Fatalf("change draft priority: %v", err)
		}
		reordered, err := internships.ReplacePreferences(student.ID, []uint{second.ID, first.ID})
		if err != nil || len(reordered.Preferences) != 2 || reordered.Preferences[0].OpportunityApplicationID != second.ID {
			t.Fatalf("reorder draft preferences: case=%+v err=%v", reordered, err)
		}
		if err := internships.DeletePreference(student.ID, reordered.Preferences[1].ID); err != nil {
			t.Fatalf("remove draft preference: %v", err)
		}
		current, err := internships.GetCurrentCase(student.ID)
		if err != nil || len(current.Preferences) != 1 || current.Preferences[0].Priority != 1 {
			t.Fatalf("draft after removal: case=%+v err=%v", current, err)
		}
	})

	for count := 1; count <= 3; count++ {
		count := count
		t.Run(fmt.Sprintf("submission succeeds with priorities one through %d", count), func(t *testing.T) {
			student, _ := createStudentCase(t, fmt.Sprintf("official-submit-%d", count), true)
			ids := make([]uint, count)
			for index := range ids {
				ids[index] = createApplication(t, student, model.ApplicationStatusAccepted, model.OpportunityStatusOpen).ID
			}
			if _, err := internships.ReplacePreferences(student.ID, ids); err != nil {
				t.Fatalf("select preferences: %v", err)
			}
			submitted, err := internships.SubmitCase(student.ID)
			if err != nil || submitted.Status != model.InternshipCaseStatusPendingUniversityReview ||
				submitted.SubmittedAt == nil || submitted.SelectedPreferenceID != nil || len(submitted.Preferences) != count {
				t.Fatalf("submitted case = %+v, err = %v", submitted, err)
			}
			for index, preference := range submitted.Preferences {
				if preference.Priority != index+1 || preference.OpportunityApplication.Opportunity.Company.ID != company.ID {
					t.Fatalf("persisted preference %d = %+v", index, preference)
				}
			}
		})
	}

	for index, priorities := range [][]int{{2}, {1, 3}, {2, 3}} {
		priorities := priorities
		t.Run(fmt.Sprintf("submission rejects noncontiguous priorities %v", priorities), func(t *testing.T) {
			student, internshipCase := createStudentCase(t, fmt.Sprintf("official-gap-%d", index), true)
			for _, priority := range priorities {
				application := createApplication(t, student, model.ApplicationStatusAccepted, model.OpportunityStatusOpen)
				preference := model.InternshipPreference{InternshipCaseID: internshipCase.ID, OpportunityApplicationID: application.ID, Priority: priority}
				if err := tx.Create(&preference).Error; err != nil {
					t.Fatalf("insert gap priority %d: %v", priority, err)
				}
			}
			if _, err := internships.SubmitCase(student.ID); !errors.Is(err, ErrInvalidApplication) {
				t.Fatalf("gap submit error = %v", err)
			}
			var persisted model.InternshipCase
			if err := tx.First(&persisted, internshipCase.ID).Error; err != nil || persisted.Status != model.InternshipCaseStatusDraft {
				t.Fatalf("gap case changed: %+v err=%v", persisted, err)
			}
		})
	}

	t.Run("empty and tampered preferences do not submit", func(t *testing.T) {
		emptyStudent, emptyCase := createStudentCase(t, "official-empty", true)
		if _, err := internships.SubmitCase(emptyStudent.ID); !errors.Is(err, ErrInvalidApplication) {
			t.Fatalf("empty submit error = %v", err)
		}
		var emptyPersisted model.InternshipCase
		_ = tx.First(&emptyPersisted, emptyCase.ID).Error
		if emptyPersisted.Status != model.InternshipCaseStatusDraft {
			t.Fatalf("empty case status = %s", emptyPersisted.Status)
		}

		tamperedStudent, tamperedCase := createStudentCase(t, "official-tampered", true)
		tamperedApplication := createApplication(t, tamperedStudent, model.ApplicationStatusAccepted, model.OpportunityStatusOpen)
		preference := model.InternshipPreference{InternshipCaseID: tamperedCase.ID, OpportunityApplicationID: tamperedApplication.ID, Priority: 1}
		if err := tx.Create(&preference).Error; err != nil {
			t.Fatalf("create tampered preference: %v", err)
		}
		if err := tx.Model(&tamperedApplication).Update("status", model.ApplicationStatusRejected).Error; err != nil {
			t.Fatalf("tamper application status: %v", err)
		}
		if _, err := internships.SubmitCase(tamperedStudent.ID); !errors.Is(err, ErrInvalidApplication) {
			t.Fatalf("tampered submit error = %v", err)
		}
		var tamperedPersisted model.InternshipCase
		_ = tx.First(&tamperedPersisted, tamperedCase.ID).Error
		if tamperedPersisted.Status != model.InternshipCaseStatusDraft {
			t.Fatalf("tampered case status = %s", tamperedPersisted.Status)
		}
	})

	t.Run("preference owned by another student cannot submit", func(t *testing.T) {
		student, internshipCase := createStudentCase(t, "official-corrupt-owner", true)
		otherStudent := applicationTestUser(t, tx, suffix, "official-corrupt-owner-other", model.RoleStudent, nil)
		otherApplication := createApplication(t, otherStudent, model.ApplicationStatusAccepted, model.OpportunityStatusOpen)
		preference := model.InternshipPreference{InternshipCaseID: internshipCase.ID, OpportunityApplicationID: otherApplication.ID, Priority: 1}
		if err := tx.Create(&preference).Error; err != nil {
			t.Fatalf("create corrupt owner preference: %v", err)
		}
		if _, err := internships.SubmitCase(student.ID); !errors.Is(err, ErrInvalidApplication) {
			t.Fatalf("corrupt owner submit error = %v", err)
		}
		var persisted model.InternshipCase
		_ = tx.First(&persisted, internshipCase.ID).Error
		if persisted.Status != model.InternshipCaseStatusDraft {
			t.Fatalf("corrupt owner case status = %s", persisted.Status)
		}
	})

	t.Run("closed opportunity remains valid after application acceptance", func(t *testing.T) {
		student, _ := createStudentCase(t, "official-closed", true)
		accepted := createApplication(t, student, model.ApplicationStatusAccepted, model.OpportunityStatusClosed)
		if _, err := internships.ReplacePreferences(student.ID, []uint{accepted.ID}); err != nil {
			t.Fatalf("select closed accepted application: %v", err)
		}
		if _, err := internships.SubmitCase(student.ID); err != nil {
			t.Fatalf("submit closed accepted application: %v", err)
		}
	})

	t.Run("cancelled history is preserved and accepted application can be reused", func(t *testing.T) {
		student := applicationTestUser(t, tx, suffix, "official-cancelled-reuse", model.RoleStudent, nil)
		assignment := model.ProfessorAssignment{StudentID: student.ID, ProfessorID: professor.ID, AssignedAt: time.Now()}
		if err := tx.Create(&assignment).Error; err != nil {
			t.Fatalf("create cancelled assignment: %v", err)
		}
		accepted := createApplication(t, student, model.ApplicationStatusAccepted, model.OpportunityStatusOpen)
		cancelled := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusCancelled}
		if err := tx.Create(&cancelled).Error; err != nil {
			t.Fatalf("create cancelled case: %v", err)
		}
		oldPreference := model.InternshipPreference{InternshipCaseID: cancelled.ID, OpportunityApplicationID: accepted.ID, Priority: 1}
		if err := tx.Create(&oldPreference).Error; err != nil {
			t.Fatalf("create cancelled preference: %v", err)
		}
		current, created, err := internships.CreateOrGetCase(student.ID)
		if err != nil || !created || current.ID == cancelled.ID || current.Status != model.InternshipCaseStatusDraft {
			t.Fatalf("new draft after cancellation: %+v created=%v err=%v", current, created, err)
		}
		if _, err := internships.ReplacePreferences(student.ID, []uint{accepted.ID}); err != nil {
			t.Fatalf("reuse accepted application: %v", err)
		}
		var oldCount int64
		if err := tx.Model(&model.InternshipPreference{}).Where("internship_case_id = ?", cancelled.ID).Count(&oldCount).Error; err != nil || oldCount != 1 {
			t.Fatalf("cancelled preferences changed: count=%d err=%v", oldCount, err)
		}
		var cancelledPersisted model.InternshipCase
		_ = tx.First(&cancelledPersisted, cancelled.ID).Error
		if cancelledPersisted.Status != model.InternshipCaseStatusCancelled {
			t.Fatalf("cancelled case status = %s", cancelledPersisted.Status)
		}
	})

	t.Run("completed history blocks new official case", func(t *testing.T) {
		student := applicationTestUser(t, tx, suffix, "official-completed", model.RoleStudent, nil)
		assignment := model.ProfessorAssignment{StudentID: student.ID, ProfessorID: professor.ID, AssignedAt: time.Now()}
		if err := tx.Create(&assignment).Error; err != nil {
			t.Fatalf("create completed assignment: %v", err)
		}
		completed := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusCompleted}
		if err := tx.Create(&completed).Error; err != nil {
			t.Fatalf("create completed case: %v", err)
		}
		if _, created, err := internships.CreateOrGetCase(student.ID); !errors.Is(err, ErrInternshipCompleted) || created {
			t.Fatalf("completed creation: created=%v err=%v", created, err)
		}
	})

	t.Run("submitted case is immutable and locks new recruitment applications", func(t *testing.T) {
		student, _ := createStudentCase(t, "official-lock", true)
		accepted := createApplication(t, student, model.ApplicationStatusAccepted, model.OpportunityStatusOpen)
		selected, err := internships.ReplacePreferences(student.ID, []uint{accepted.ID})
		if err != nil {
			t.Fatalf("select before lock: %v", err)
		}
		submitted, err := internships.SubmitCase(student.ID)
		if err != nil {
			t.Fatalf("submit before lock checks: %v", err)
		}
		credits, mobile := 90, "09121111111"
		if _, err := internships.UpdateCase(student.ID, &credits, &mobile); !errors.Is(err, ErrCaseNotEditable) {
			t.Fatalf("update submitted case error = %v", err)
		}
		if _, err := internships.ReplacePreferences(student.ID, nil); !errors.Is(err, ErrCaseNotEditable) {
			t.Fatalf("replace submitted preferences error = %v", err)
		}
		if _, err := internships.AddPreference(student.ID, PreferenceInput{Priority: 2, OpportunityApplicationID: accepted.ID}); !errors.Is(err, ErrCaseNotEditable) {
			t.Fatalf("add submitted preference error = %v", err)
		}
		if err := internships.DeletePreference(student.ID, selected.Preferences[0].ID); !errors.Is(err, ErrCaseNotEditable) {
			t.Fatalf("delete submitted preference error = %v", err)
		}
		if _, err := internships.UpdatePreference(student.ID, selected.Preferences[0].ID, PreferenceInput{Priority: 1, OpportunityApplicationID: accepted.ID}); !errors.Is(err, ErrCaseNotEditable) {
			t.Fatalf("update submitted preference error = %v", err)
		}
		newOpportunity := applicationTestOpportunity(t, tx, suffix, "official-after-submit", company.ID, supervisor.ID, model.OpportunityStatusOpen)
		resume := applicationTestResume(suffix, "official-after-submit")
		if _, err := applications.Apply(student.ID, newOpportunity.ID, resume); !errors.Is(err, ErrInternshipCaseAlreadyInProgress) {
			t.Fatalf("recruitment lock error = %v", err)
		}
		assertNoApplicationOrFile(t, tx, student.ID, newOpportunity.ID, resume)
		if submitted.SelectedPreferenceID != nil {
			t.Fatalf("student submission selected preference %d", *submitted.SelectedPreferenceID)
		}
	})
}
