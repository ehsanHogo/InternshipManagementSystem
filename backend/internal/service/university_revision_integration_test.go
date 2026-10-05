package service

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"internship-management-system/backend/internal/model"
)

func TestUniversityPreferenceRevision(t *testing.T) {
	db := termTestDatabase(t)
	suffix := time.Now().UnixNano()
	university := applicationTestUser(t, db, suffix, "revision-university", model.RoleUniversitySupervisor, nil)
	professor := applicationTestUser(t, db, suffix, "revision-professor", model.RoleProfessor, nil)
	student := applicationTestUser(t, db, suffix, "revision-student", model.RoleStudent, nil)
	other := applicationTestUser(t, db, suffix, "revision-other", model.RoleStudent, nil)
	company := applicationTestCompany(t, db, suffix, "revision")
	supervisor := applicationTestUser(t, db, suffix, "revision-company", model.RoleCompanySupervisor, &company.ID)
	term, err := NewInternshipTermService(db).Create(university.ID, 1405, model.InternshipTermTypeSummer)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ProfessorAssignment{StudentID: student.ID, ProfessorID: professor.ID, AssignedAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	workflow := NewInternshipService(db)
	applications := NewOpportunityApplicationService(db)
	item, created, err := workflow.CreateOrGetCase(student.ID)
	if err != nil || !created {
		t.Fatalf("create: %+v %v %v", item, created, err)
	}
	credits, mobile := 90, "09123456789"
	if _, err := workflow.UpdateCase(student.ID, &credits, &mobile); err != nil {
		t.Fatal(err)
	}
	accepted := func(label string, owner uint, status model.ApplicationStatus) model.OpportunityApplication {
		t.Helper()
		opportunity := applicationTestOpportunity(t, db, suffix, label, company.ID, supervisor.ID, model.OpportunityStatusOpen)
		application, err := applications.Apply(owner, opportunity.ID, applicationTestResume(suffix, label))
		if err != nil {
			t.Fatal(err)
		}
		if status != model.ApplicationStatusPending {
			application, err = applications.Review(supervisor.ID, application.ID, status, nil)
			if err != nil {
				t.Fatal(err)
			}
		}
		return *application
	}
	first := accepted("revision-first", student.ID, model.ApplicationStatusAccepted)
	second := accepted("revision-second", student.ID, model.ApplicationStatusAccepted)
	foreign := accepted("revision-foreign", other.ID, model.ApplicationStatusAccepted)
	pending := accepted("revision-pending", student.ID, model.ApplicationStatusPending)
	rejected := accepted("revision-rejected", student.ID, model.ApplicationStatusRejected)
	if _, err := workflow.ReplacePreferences(student.ID, []uint{first.ID, second.ID}); err != nil {
		t.Fatal(err)
	}
	item, err = workflow.SubmitCase(student.ID)
	if err != nil {
		t.Fatal(err)
	}
	load := func() model.InternshipCase {
		t.Helper()
		var row model.InternshipCase
		if err := db.First(&row, item.ID).Error; err != nil {
			t.Fatal(err)
		}
		return row
	}
	before := load()
	for _, comment := range []string{"", " \n\t"} {
		if _, err := workflow.RequestUniversityRevision(item.ID, university.ID, comment); !errors.Is(err, ErrUniversityRevisionCommentRequired) {
			t.Fatalf("blank comment: %v", err)
		}
	}
	for _, actor := range []model.User{student, professor, supervisor, applicationTestUser(t, db, suffix, "revision-admin", model.RoleAdmin, nil)} {
		if _, err := workflow.RequestUniversityRevision(item.ID, actor.ID, "اصلاح"); !errors.Is(err, ErrCaseAccessDenied) {
			t.Fatalf("role %s: %v", actor.Role, err)
		}
	}
	if !reflect.DeepEqual(before, load()) {
		t.Fatal("rejected revision changed case")
	}
	comment := "شرکت‌های انتخاب‌شده مناسب نیستند؛ اولویت‌ها را اصلاح کنید."
	item, err = workflow.RequestUniversityRevision(item.ID, university.ID, "  "+comment+"  ")
	if err != nil || item.Status != model.InternshipCaseStatusRevisionRequested || item.UniversityRevisionComment == nil || *item.UniversityRevisionComment != comment || item.UniversityRevisionRequestedAt == nil || item.UniversityRevisionRequestedBy == nil || *item.UniversityRevisionRequestedBy != university.ID {
		t.Fatalf("revision metadata: %+v %v", item, err)
	}
	after := load()
	expected := before
	expected.Status, expected.UpdatedAt = after.Status, after.UpdatedAt
	expected.UniversityRevisionComment, expected.UniversityRevisionRequestedAt, expected.UniversityRevisionRequestedBy = after.UniversityRevisionComment, after.UniversityRevisionRequestedAt, after.UniversityRevisionRequestedBy
	if !reflect.DeepEqual(expected, after) {
		t.Fatalf("revision altered unrelated fields: %+v -> %+v", before, after)
	}
	if item.TermID == nil || *item.TermID != term.ID || item.ProfessorID != professor.ID {
		t.Fatal("term or professor changed")
	}
	if !containsStatus(nonTerminalCaseStatuses(), model.InternshipCaseStatusRevisionRequested) || containsStatus(terminalCaseStatuses(), model.InternshipCaseStatusRevisionRequested) {
		t.Fatal("revision must be non-terminal")
	}
	if current, err := workflow.GetCurrentCase(student.ID); err != nil || current.ID != item.ID || current.UniversityRevisionComment == nil || *current.UniversityRevisionComment != comment {
		t.Fatalf("student cannot read revision: %+v %v", current, err)
	}
	if current, created, err := workflow.CreateOrGetCase(student.ID); err != nil || created || current.ID != item.ID {
		t.Fatalf("second case created: %+v %v %v", current, created, err)
	}
	if _, err := workflow.UpdateCase(student.ID, &credits, &mobile); !errors.Is(err, ErrCaseNotEditable) {
		t.Fatalf("non-preference edit during revision: %v", err)
	}
	approve := UniversityPlacementApprovalInput{PreferenceID: item.Preferences[0].ID, LetterNumber: "revision-letter", LetterDate: time.Now().UTC()}
	if _, err := workflow.ApproveUniversityPlacement(item.ID, approve); !errors.Is(err, ErrCaseNotPendingUniversityReview) {
		t.Fatalf("select before resubmission: %v", err)
	}
	if _, err := workflow.GetCompanyCase(supervisor.ID, item.ID); !errors.Is(err, ErrCaseNotAssignedToCompany) {
		t.Fatalf("company visibility: %v", err)
	}
	if _, err := workflow.GetProfessorCase(professor.ID, item.ID); !errors.Is(err, ErrCaseAccessDenied) {
		t.Fatalf("professor visibility: %v", err)
	}
	// A new recruitment application and company acceptance are possible during revision.
	third := accepted("revision-new", student.ID, model.ApplicationStatusAccepted)
	fourth := accepted("revision-extra", student.ID, model.ApplicationStatusAccepted)
	for _, test := range []struct {
		id   uint
		want error
	}{{foreign.ID, ErrPreferenceNotOwned}, {pending.ID, ErrPreferenceNotAccepted}, {rejected.ID, ErrPreferenceNotAccepted}} {
		if _, err := workflow.AddPreference(student.ID, PreferenceInput{Priority: 3, OpportunityApplicationID: test.id}); !errors.Is(err, test.want) {
			t.Fatalf("invalid addition %d: %v", test.id, err)
		}
		if _, err := workflow.ReplacePreferences(student.ID, []uint{test.id}); !errors.Is(err, test.want) {
			t.Fatalf("invalid replacement %d: %v", test.id, err)
		}
		if _, err := workflow.UpdatePreference(student.ID, item.Preferences[0].ID, PreferenceInput{Priority: 1, OpportunityApplicationID: test.id}); !errors.Is(err, test.want) {
			t.Fatalf("invalid update %d: %v", test.id, err)
		}
	}
	if _, err := workflow.ReplacePreferences(student.ID, []uint{first.ID, first.ID}); !errors.Is(err, ErrPreferenceAlreadyExists) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := workflow.ReplacePreferences(student.ID, []uint{first.ID, second.ID, third.ID, pending.ID}); !errors.Is(err, ErrPreferenceLimit) {
		t.Fatalf("limit: %v", err)
	}
	if _, err := workflow.AddPreference(student.ID, PreferenceInput{Priority: 4, OpportunityApplicationID: third.ID}); !errors.Is(err, ErrInvalidPreference) {
		t.Fatalf("priority: %v", err)
	}
	if err := workflow.DeletePreference(student.ID, item.Preferences[0].ID); err != nil {
		t.Fatal(err)
	}
	current, err := workflow.GetCurrentCase(student.ID)
	if err != nil || len(current.Preferences) != 1 || current.Preferences[0].Priority != 1 {
		t.Fatalf("delete compaction: %+v %v", current, err)
	}
	// Single preference update replaces an accepted application and changes priority.
	if _, err := workflow.UpdatePreference(student.ID, current.Preferences[0].ID, PreferenceInput{Priority: 2, OpportunityApplicationID: third.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := workflow.SubmitCase(student.ID); !errors.Is(err, ErrInvalidApplication) {
		t.Fatalf("noncontiguous resubmission: %v", err)
	}
	if _, err := workflow.ReplacePreferences(student.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := workflow.SubmitCase(student.ID); !errors.Is(err, ErrInvalidApplication) {
		t.Fatalf("empty resubmission: %v", err)
	}
	for i, application := range []model.OpportunityApplication{first, second, third} {
		if _, err := workflow.AddPreference(student.ID, PreferenceInput{Priority: i + 1, OpportunityApplicationID: application.ID}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := workflow.AddPreference(student.ID, PreferenceInput{Priority: 3, OpportunityApplicationID: fourth.ID}); !errors.Is(err, ErrPreferenceLimit) {
		t.Fatalf("fourth accepted preference: %v", err)
	}
	if _, err := workflow.AddPreference(student.ID, PreferenceInput{Priority: 3, OpportunityApplicationID: pending.ID}); !errors.Is(err, ErrPreferenceNotAccepted) {
		t.Fatalf("accepted validation: %v", err)
	}
	current, err = workflow.ReplacePreferences(student.ID, []uint{third.ID, second.ID, first.ID})
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range []uint{third.ID, second.ID, first.ID} {
		if current.Preferences[i].Priority != i+1 || current.Preferences[i].OpportunityApplicationID != id {
			t.Fatal("reorder lost contiguous priorities")
		}
	}
	// Submission revalidates an application that is no longer accepted.
	if err := db.Model(&model.OpportunityApplication{}).Where("id = ?", third.ID).Update("status", model.ApplicationStatusRejected).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := workflow.SubmitCase(student.ID); !errors.Is(err, ErrInvalidApplication) {
		t.Fatalf("stale accepted application: %v", err)
	}
	if err := db.Model(&model.OpportunityApplication{}).Where("id = ?", third.ID).Update("status", model.ApplicationStatusAccepted).Error; err != nil {
		t.Fatal(err)
	}
	current, err = workflow.SubmitCase(student.ID)
	if err != nil || current.ID != item.ID || current.Status != model.InternshipCaseStatusPendingUniversityReview || current.TermID == nil || *current.TermID != term.ID || current.ProfessorID != professor.ID || !current.CreatedAt.Equal(before.CreatedAt) {
		t.Fatalf("resubmission: %+v %v", current, err)
	}
	if current.UniversityRevisionComment == nil || *current.UniversityRevisionComment != comment {
		t.Fatal("latest revision audit must survive resubmission")
	}
	current, err = workflow.RequestUniversityRevision(item.ID, university.ID, "دلیل جدید")
	if err != nil || *current.UniversityRevisionComment != "دلیل جدید" {
		t.Fatalf("latest metadata update: %+v %v", current, err)
	}
	current, err = workflow.SubmitCase(student.ID)
	if err != nil {
		t.Fatal(err)
	}
	approve.PreferenceID = current.Preferences[0].ID
	current, err = workflow.ApproveUniversityPlacement(item.ID, approve)
	if err != nil || current.Status != model.InternshipCaseStatusPendingCompanyDetails || *current.TermID != term.ID {
		t.Fatalf("selection after revision: %+v %v", current, err)
	}
	for _, status := range []model.InternshipCaseStatus{model.InternshipCaseStatusDraft, model.InternshipCaseStatusRevisionRequested, model.InternshipCaseStatusPendingCompanyDetails, model.InternshipCaseStatusPendingFinalApproval, model.InternshipCaseStatusActive, model.InternshipCaseStatusPassed, model.InternshipCaseStatusFailed, model.InternshipCaseStatusCancelled} {
		t.Run(fmt.Sprintf("reject revision from %s", status), func(t *testing.T) {
			if err := db.Model(&model.InternshipCase{}).Where("id = ?", item.ID).Update("status", status).Error; err != nil {
				t.Fatal(err)
			}
			before := load()
			if _, err := workflow.RequestUniversityRevision(item.ID, university.ID, "اصلاح"); !errors.Is(err, ErrCaseNotPendingUniversityReview) {
				t.Fatalf("invalid status: %v", err)
			}
			if !reflect.DeepEqual(before, load()) {
				t.Fatal("invalid state changed case")
			}
		})
	}
}
