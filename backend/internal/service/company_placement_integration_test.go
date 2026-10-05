package service

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"internship-management-system/backend/internal/model"
)

func TestCompanyPlacementDetails(t *testing.T) {
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
	createOpenTermFixture(t, tx, suffix)
	companyA := applicationTestCompany(t, tx, suffix, "pa")
	companyB := applicationTestCompany(t, tx, suffix, "pb")
	supervisorA := applicationTestUser(t, tx, suffix, "pa", model.RoleCompanySupervisor, &companyA.ID)
	supervisorB := applicationTestUser(t, tx, suffix, "pb", model.RoleCompanySupervisor, &companyB.ID)
	professor := applicationTestUser(t, tx, suffix, "placement-professor", model.RoleProfessor, nil)
	workflow := NewInternshipService(tx)
	applications := NewOpportunityApplicationService(tx)
	next := 0
	type fixture struct {
		student     model.User
		internship  model.InternshipCase
		preferences []model.InternshipPreference
	}
	create := func(t *testing.T, selected int) fixture {
		t.Helper()
		next++
		label := fmt.Sprintf("placement-%d", next)
		student := applicationTestUser(t, tx, suffix, label, model.RoleStudent, nil)
		assignment := model.ProfessorAssignment{StudentID: student.ID, ProfessorID: professor.ID, AssignedAt: time.Now()}
		if err := tx.Create(&assignment).Error; err != nil {
			t.Fatal(err)
		}
		item, _, err := workflow.CreateOrGetCase(student.ID)
		if err != nil {
			t.Fatal(err)
		}
		credits, mobile := 90, "09120000000"
		if _, err := workflow.UpdateCase(student.ID, &credits, &mobile); err != nil {
			t.Fatal(err)
		}
		prefs := []model.InternshipPreference{}
		for i, supervisor := range []model.User{supervisorA, supervisorB} {
			opportunity := applicationTestOpportunity(t, tx, suffix, fmt.Sprintf("%s-%d", label, i), *supervisor.CompanyID, supervisor.ID, model.OpportunityStatusOpen)
			app, err := applications.Apply(student.ID, opportunity.ID, applicationTestResume(suffix, fmt.Sprintf("%s-%d", label, i)))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := applications.Review(supervisor.ID, app.ID, model.ApplicationStatusAccepted, nil); err != nil {
				t.Fatal(err)
			}
			pref, err := workflow.AddPreference(student.ID, PreferenceInput{Priority: i + 1, OpportunityApplicationID: app.ID})
			if err != nil {
				t.Fatal(err)
			}
			prefs = append(prefs, *pref)
		}
		if _, err := workflow.SubmitCase(student.ID); err != nil {
			t.Fatal(err)
		}
		date, _ := time.Parse("2006-01-02", "2026-09-01")
		item, err = workflow.ApproveUniversityPlacement(item.ID, UniversityPlacementApprovalInput{PreferenceID: prefs[selected].ID, LetterNumber: label, LetterDate: date})
		if err != nil {
			t.Fatal(err)
		}
		return fixture{student: student, internship: *item, preferences: prefs}
	}
	input := PlacementDetailsInput{InternshipSubject: "  توسعه نرم‌افزار  ", StartDate: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC), WorkplaceAddress: "  محل واقعی  ", WorkplacePhone: "  021-123 داخلی ۴  "}
	snapshot := func(t *testing.T, id uint) model.InternshipCase {
		t.Helper()
		var item model.InternshipCase
		if err := tx.First(&item, id).Error; err != nil {
			t.Fatal(err)
		}
		return item
	}
	unchanged := func(t *testing.T, before model.InternshipCase) {
		t.Helper()
		after := snapshot(t, before.ID)
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("case changed on rejected action: before=%+v after=%+v", before, after)
		}
	}

	t.Run("selected company only, pending lists and cross-company mutation", func(t *testing.T) {
		a, b := create(t, 0), create(t, 1)
		for _, check := range []struct {
			supervisor uint
			yes, no    fixture
		}{{supervisorA.ID, a, b}, {supervisorB.ID, b, a}} {
			cases, err := workflow.ListPendingCompanyDetailsCases(check.supervisor)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, item := range cases {
				if item.ID == check.no.internship.ID {
					t.Fatal("non-selected company received case")
				}
				if item.ID == check.yes.internship.ID {
					found = true
				}
			}
			if !found {
				t.Fatal("selected case absent from pending list")
			}
			if _, err := workflow.GetCompanyCase(check.supervisor, check.yes.internship.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := workflow.GetCompanyCase(check.supervisor, check.no.internship.ID); !errors.Is(err, ErrCaseNotAssignedToCompany) {
				t.Fatalf("cross-company detail: %v", err)
			}
			before := snapshot(t, check.no.internship.ID)
			if _, err := workflow.SubmitPlacementDetails(check.supervisor, check.no.internship.ID, input); !errors.Is(err, ErrCaseNotAssignedToCompany) {
				t.Fatalf("cross-company mutation: %v", err)
			}
			unchanged(t, before)
		}
	})
	t.Run("same company but different supervisor has no access", func(t *testing.T) {
		f := create(t, 0)
		other := applicationTestUser(t, tx, suffix, "same-company-other", model.RoleCompanySupervisor, &companyA.ID)
		cases, err := workflow.ListPendingCompanyDetailsCases(other.ID)
		if err != nil || len(cases) != 0 {
			t.Fatalf("wrong supervisor pending: %+v %v", cases, err)
		}
		if _, err := workflow.GetCompanyCase(other.ID, f.internship.ID); !errors.Is(err, ErrCaseNotAssignedToCompany) {
			t.Fatalf("wrong supervisor detail: %v", err)
		}
		before := snapshot(t, f.internship.ID)
		if _, err := workflow.SubmitPlacementDetails(other.ID, f.internship.ID, input); !errors.Is(err, ErrCaseNotAssignedToCompany) {
			t.Fatalf("wrong supervisor mutation: %v", err)
		}
		unchanged(t, before)
	})
	t.Run("required fields and sizes roll back without partial writes", func(t *testing.T) {
		f := create(t, 0)
		tests := []struct {
			name   string
			change func(*PlacementDetailsInput)
			want   error
		}{
			{"subject", func(i *PlacementDetailsInput) { i.InternshipSubject = " \t " }, ErrInternshipSubjectRequired},
			{"date", func(i *PlacementDetailsInput) { i.StartDate = time.Time{} }, ErrStartDateRequired},
			{"address", func(i *PlacementDetailsInput) { i.WorkplaceAddress = " \n " }, ErrWorkplaceAddressRequired},
			{"phone", func(i *PlacementDetailsInput) { i.WorkplacePhone = " " }, ErrWorkplacePhoneRequired},
			{"long subject", func(i *PlacementDetailsInput) { i.InternshipSubject = strings.Repeat("م", 501) }, ErrPlacementDetailsTooLong},
			{"long address", func(i *PlacementDetailsInput) { i.WorkplaceAddress = strings.Repeat("م", 1001) }, ErrPlacementDetailsTooLong},
			{"long phone", func(i *PlacementDetailsInput) { i.WorkplacePhone = strings.Repeat("1", 51) }, ErrPlacementDetailsTooLong},
			{"invalid year", func(i *PlacementDetailsInput) { i.StartDate = time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC) }, ErrInvalidStartDate},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				before := snapshot(t, f.internship.ID)
				value := input
				test.change(&value)
				if _, err := workflow.SubmitPlacementDetails(supervisorA.ID, f.internship.ID, value); !errors.Is(err, test.want) {
					t.Fatalf("error=%v want=%v", err, test.want)
				}
				unchanged(t, before)
			})
		}
	})
	t.Run("every other state and repeated submission conflict", func(t *testing.T) {
		f := create(t, 0)
		for _, status := range []model.InternshipCaseStatus{model.InternshipCaseStatusDraft, model.InternshipCaseStatusPendingUniversityReview, model.InternshipCaseStatusPendingFinalApproval, model.InternshipCaseStatusReadyToStart, model.InternshipCaseStatusActive, model.InternshipCaseStatusPassed, model.InternshipCaseStatusFailed, model.InternshipCaseStatusCancelled} {
			t.Run(string(status), func(t *testing.T) {
				if err := tx.Model(&model.InternshipCase{}).Where("id = ?", f.internship.ID).Update("status", status).Error; err != nil {
					t.Fatal(err)
				}
				before := snapshot(t, f.internship.ID)
				if _, err := workflow.SubmitPlacementDetails(supervisorA.ID, f.internship.ID, input); !errors.Is(err, ErrCaseNotPendingCompanyDetails) {
					t.Fatalf("error=%v", err)
				}
				unchanged(t, before)
			})
		}
	})
	t.Run("malformed selected placement, accepted application and letter integrity", func(t *testing.T) {
		for _, name := range []string{"no preference", "no supervisor", "wrong snapshot company", "preference from other case", "application not accepted", "application from other student", "no letter number", "blank letter number", "no letter date", "supervisor moved company", "supervisor role changed"} {
			t.Run(name, func(t *testing.T) {
				f := create(t, 0)
				var app model.OpportunityApplication
				if err := tx.First(&app, f.preferences[0].OpportunityApplicationID).Error; err != nil {
					t.Fatal(err)
				}
				var restore func()
				switch name {
				case "no preference":
					if err := tx.Model(&model.InternshipCase{}).Where("id = ?", f.internship.ID).Update("selected_preference_id", nil).Error; err != nil {
						t.Fatal(err)
					}
				case "no supervisor":
					if err := tx.Model(&model.InternshipCase{}).Where("id = ?", f.internship.ID).Update("company_supervisor_id", nil).Error; err != nil {
						t.Fatal(err)
					}
				case "wrong snapshot company":
					if err := tx.Model(&model.InternshipCase{}).Where("id = ?", f.internship.ID).Update("selected_preference_id", f.preferences[1].ID).Error; err != nil {
						t.Fatal(err)
					}
				case "preference from other case":
					other := create(t, 0)
					if err := tx.Model(&model.InternshipCase{}).Where("id = ?", f.internship.ID).Update("selected_preference_id", other.preferences[0].ID).Error; err != nil {
						t.Fatal(err)
					}
				case "application not accepted":
					if err := tx.Model(&app).Update("status", model.ApplicationStatusPending).Error; err != nil {
						t.Fatal(err)
					}
				case "application from other student":
					other := create(t, 0)
					if err := tx.Model(&app).Update("student_id", other.student.ID).Error; err != nil {
						t.Fatal(err)
					}
				case "no letter number":
					if err := tx.Model(&model.InternshipCase{}).Where("id = ?", f.internship.ID).Update("letter_number", nil).Error; err != nil {
						t.Fatal(err)
					}
				case "blank letter number":
					if err := tx.Model(&model.InternshipCase{}).Where("id = ?", f.internship.ID).Update("letter_number", " ").Error; err != nil {
						t.Fatal(err)
					}
				case "no letter date":
					if err := tx.Model(&model.InternshipCase{}).Where("id = ?", f.internship.ID).Update("letter_date", nil).Error; err != nil {
						t.Fatal(err)
					}
				case "supervisor moved company":
					if err := tx.Model(&supervisorA).Update("company_id", companyB.ID).Error; err != nil {
						t.Fatal(err)
					}
					restore = func() {
						if err := tx.Model(&supervisorA).Update("company_id", companyA.ID).Error; err != nil {
							t.Fatal(err)
						}
					}
				case "supervisor role changed":
					if err := tx.Model(&supervisorA).Update("role", model.RoleStudent).Error; err != nil {
						t.Fatal(err)
					}
					restore = func() {
						if err := tx.Model(&supervisorA).Update("role", model.RoleCompanySupervisor).Error; err != nil {
							t.Fatal(err)
						}
					}
				}
				if restore != nil {
					defer restore()
				}
				before := snapshot(t, f.internship.ID)
				if _, err := workflow.SubmitPlacementDetails(supervisorA.ID, f.internship.ID, input); err == nil {
					t.Fatal("malformed case accepted")
				}
				unchanged(t, before)
				if name == "wrong snapshot company" || name == "preference from other case" || name == "application not accepted" || name == "application from other student" || name == "supervisor moved company" || name == "supervisor role changed" {
					if _, err := workflow.GetCompanyCase(supervisorA.ID, f.internship.ID); !errors.Is(err, ErrCaseNotAssignedToCompany) {
						t.Fatalf("malformed detail not isolated: %v", err)
					}
					list, err := workflow.ListCompanyCases(supervisorA.ID)
					if err != nil {
						t.Fatal(err)
					}
					for _, item := range list {
						if item.ID == f.internship.ID {
							t.Fatal("malformed case listed")
						}
					}
				}
			})
		}
	})
	t.Run("unapproved company submits, correction comment clears, recruitment unchanged and all roles can read", func(t *testing.T) {
		f := create(t, 1)
		oldSubject, address, phone := "old subject", "old address", "old phone"
		date := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		if err := tx.Model(&model.InternshipCase{}).Where("id = ?", f.internship.ID).Updates(map[string]any{"internship_subject": oldSubject, "start_date": date, "workplace_address": address, "workplace_phone": phone, "company_details_revision_comment": "اصلاح محل"}).Error; err != nil {
			t.Fatal(err)
		}
		var beforeApp model.OpportunityApplication
		if err := tx.First(&beforeApp, f.preferences[1].OpportunityApplicationID).Error; err != nil {
			t.Fatal(err)
		}
		var beforeOpportunity model.InternshipOpportunity
		if err := tx.First(&beforeOpportunity, beforeApp.OpportunityID).Error; err != nil {
			t.Fatal(err)
		}
		var beforePreference model.InternshipPreference
		if err := tx.First(&beforePreference, f.preferences[1].ID).Error; err != nil {
			t.Fatal(err)
		}
		var beforeCompany model.Company
		if err := tx.First(&beforeCompany, companyB.ID).Error; err != nil {
			t.Fatal(err)
		}
		var beforeUser model.User
		if err := tx.First(&beforeUser, supervisorB.ID).Error; err != nil {
			t.Fatal(err)
		}
		if beforeCompany.IsApproved {
			t.Fatal("fixture company must be unapproved")
		}
		result, err := workflow.SubmitPlacementDetails(supervisorB.ID, f.internship.ID, input)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status != model.InternshipCaseStatusPendingFinalApproval || result.CompanyDetailsRevisionComment != nil || result.InternshipSubject == nil || *result.InternshipSubject != "توسعه نرم‌افزار" || result.StartDate == nil || !result.StartDate.Equal(input.StartDate) || result.WorkplaceAddress == nil || *result.WorkplaceAddress != "محل واقعی" || result.WorkplacePhone == nil || *result.WorkplacePhone != "021-123 داخلی ۴" {
			t.Fatalf("unexpected submitted case: %+v", result)
		}
		var afterApp model.OpportunityApplication
		if err := tx.First(&afterApp, beforeApp.ID).Error; err != nil {
			t.Fatal(err)
		}
		var afterOpportunity model.InternshipOpportunity
		if err := tx.First(&afterOpportunity, beforeOpportunity.ID).Error; err != nil {
			t.Fatal(err)
		}
		var afterPreference model.InternshipPreference
		if err := tx.First(&afterPreference, beforePreference.ID).Error; err != nil {
			t.Fatal(err)
		}
		var afterCompany model.Company
		if err := tx.First(&afterCompany, beforeCompany.ID).Error; err != nil {
			t.Fatal(err)
		}
		var afterUser model.User
		if err := tx.First(&afterUser, beforeUser.ID).Error; err != nil {
			t.Fatal(err)
		}
		for _, pair := range [][2]any{{beforeApp, afterApp}, {beforeOpportunity, afterOpportunity}, {beforePreference, afterPreference}, {beforeCompany, afterCompany}, {beforeUser, afterUser}} {
			if !reflect.DeepEqual(pair[0], pair[1]) {
				t.Fatalf("unrelated data changed: %+v -> %+v", pair[0], pair[1])
			}
		}
		for _, get := range []func() (*model.InternshipCase, error){func() (*model.InternshipCase, error) { return workflow.GetCompanyCase(supervisorB.ID, f.internship.ID) }, func() (*model.InternshipCase, error) { return workflow.GetCurrentCase(f.student.ID) }, func() (*model.InternshipCase, error) { return workflow.GetUniversityCase(f.internship.ID) }} {
			item, err := get()
			if err != nil || item.Status != model.InternshipCaseStatusPendingFinalApproval || item.InternshipSubject == nil {
				t.Fatalf("submitted visibility: %+v %v", item, err)
			}
		}
		pending, err := workflow.ListPendingCompanyDetailsCases(supervisorB.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range pending {
			if item.ID == f.internship.ID {
				t.Fatal("submitted case still pending details")
			}
		}
		status := model.InternshipCaseStatusPendingFinalApproval
		final, err := workflow.ListUniversityCases(&status)
		if err != nil || len(final) == 0 {
			t.Fatalf("university final list: %v", err)
		}
		before := snapshot(t, f.internship.ID)
		if _, err := workflow.SubmitPlacementDetails(supervisorB.ID, f.internship.ID, input); !errors.Is(err, ErrCaseNotPendingCompanyDetails) {
			t.Fatalf("repeat=%v", err)
		}
		unchanged(t, before)
	})
	t.Run("not found", func(t *testing.T) {
		if _, err := workflow.SubmitPlacementDetails(supervisorA.ID, ^uint(0)>>1, input); !errors.Is(err, ErrCaseNotFound) {
			t.Fatalf("not found: %v", err)
		}
	})
}
