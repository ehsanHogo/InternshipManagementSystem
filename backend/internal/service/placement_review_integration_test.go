package service

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"internship-management-system/backend/internal/model"
)

func TestUniversityFinalPlacementReview(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Company{}, &model.User{}, &model.InternshipOpportunity{}, &model.ProfessorAssignment{}, &model.File{}, &model.OpportunityApplication{}, &model.InternshipCase{}, &model.InternshipPreference{}, &model.WeeklyReport{}, &model.CompanyEvaluation{}); err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	workflow := NewInternshipService(tx)
	suffix := time.Now().UnixNano()
	company := applicationTestCompany(t, tx, suffix, "fa")
	otherCompany := applicationTestCompany(t, tx, suffix, "fb")
	supervisor := applicationTestUser(t, tx, suffix, "final-supervisor", model.RoleCompanySupervisor, &company.ID)
	otherSupervisor := applicationTestUser(t, tx, suffix, "other-final-supervisor", model.RoleCompanySupervisor, &otherCompany.ID)
	professor := applicationTestUser(t, tx, suffix, "final-professor", model.RoleProfessor, nil)
	next := 0
	type fixture struct {
		item        model.InternshipCase
		preference  model.InternshipPreference
		application model.OpportunityApplication
		opportunity model.InternshipOpportunity
		student     model.User
	}
	create := func(t *testing.T) fixture {
		t.Helper()
		next++
		label := fmt.Sprintf("final-placement-%d", next)
		student := applicationTestUser(t, tx, suffix, label, model.RoleStudent, nil)
		opportunity := applicationTestOpportunity(t, tx, suffix, label, company.ID, supervisor.ID, model.OpportunityStatusOpen)
		file := applicationTestResume(suffix, label)
		file.UploadedBy = student.ID
		if err := tx.Create(file).Error; err != nil {
			t.Fatal(err)
		}
		application := model.OpportunityApplication{StudentID: student.ID, OpportunityID: opportunity.ID, ResumeFileID: file.ID, Status: model.ApplicationStatusAccepted, AppliedAt: time.Now()}
		if err := tx.Create(&application).Error; err != nil {
			t.Fatal(err)
		}
		credits, mobile := 90, "09123456789"
		letter, subject, address, phone := "M8-100", "توسعه نرم‌افزار", "آدرس محل", "021-123"
		date := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
		supervisorID := supervisor.ID
		item := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusPendingFinalApproval,
			PassedCredits: &credits, Mobile: &mobile, CompanySupervisorID: &supervisorID,
			LetterNumber: &letter, LetterDate: &date, InternshipSubject: &subject, StartDate: &date, WorkplaceAddress: &address, WorkplacePhone: &phone}
		if err := tx.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
		preference := model.InternshipPreference{InternshipCaseID: item.ID, OpportunityApplicationID: application.ID, Priority: 1}
		if err := tx.Create(&preference).Error; err != nil {
			t.Fatal(err)
		}
		if err := tx.Model(&item).Update("selected_preference_id", preference.ID).Error; err != nil {
			t.Fatal(err)
		}
		return fixture{item, preference, application, opportunity, student}
	}
	load := func(t *testing.T, id uint) model.InternshipCase {
		t.Helper()
		var item model.InternshipCase
		if err := tx.First(&item, id).Error; err != nil {
			t.Fatal(err)
		}
		return item
	}
	assertUnchanged := func(t *testing.T, before model.InternshipCase) {
		t.Helper()
		if after := load(t, before.ID); !reflect.DeepEqual(before, after) {
			t.Fatalf("rejected action mutated case: %+v -> %+v", before, after)
		}
	}
	assertChange := func(t *testing.T, before model.InternshipCase, status model.InternshipCaseStatus, comment *string) {
		t.Helper()
		after := load(t, before.ID)
		before.Status, before.CompanyDetailsRevisionComment, before.UpdatedAt = status, comment, after.UpdatedAt
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("unexpected case mutation: want %+v got %+v", before, after)
		}
	}
	// Compare complete rows, including timestamps, to detect any recruitment,
	// company approval or user affiliation mutation during university actions.
	externalSnapshot := func(t *testing.T, f fixture) []any {
		t.Helper()
		var preference model.InternshipPreference
		var application model.OpportunityApplication
		var opportunity model.InternshipOpportunity
		var companies []model.Company
		var users []model.User
		for _, query := range []*gorm.DB{
			tx.First(&preference, f.preference.ID), tx.First(&application, f.application.ID), tx.First(&opportunity, f.opportunity.ID),
			tx.Order("id").Find(&companies), tx.Order("id").Find(&users),
		} {
			if query.Error != nil {
				t.Fatal(query.Error)
			}
		}
		return []any{preference, application, opportunity, companies, users}
	}

	t.Run("approval accepts past and future dates and preserves all data", func(t *testing.T) {
		for _, year := range []int{2020, 2099} {
			f := create(t)
			if err := tx.Model(&f.item).Update("start_date", time.Date(year, 1, 2, 0, 0, 0, 0, time.UTC)).Error; err != nil {
				t.Fatal(err)
			}
			before, external := load(t, f.item.ID), externalSnapshot(t, f)
			result, err := workflow.ApprovePlacementDetails(f.item.ID)
			if err != nil || result.Status != model.InternshipCaseStatusReadyToStart {
				t.Fatalf("approve: %+v %v", result, err)
			}
			assertChange(t, before, model.InternshipCaseStatusReadyToStart, nil)
			if !reflect.DeepEqual(external, externalSnapshot(t, f)) {
				t.Fatal("approval changed unrelated rows")
			}
			for _, action := range []func() error{
				func() error { _, err := workflow.ApprovePlacementDetails(f.item.ID); return err },
				func() error { _, err := workflow.RequestPlacementCorrection(f.item.ID, "اصلاح"); return err },
			} {
				before := load(t, f.item.ID)
				if err := action(); !errors.Is(err, ErrCaseNotPendingFinalApproval) {
					t.Fatalf("repeated final action: %v", err)
				}
				assertUnchanged(t, before)
			}
			before = load(t, f.item.ID)
			if _, err := workflow.SubmitPlacementDetails(supervisor.ID, f.item.ID, PlacementDetailsInput{}); !errors.Is(err, ErrCaseNotPendingCompanyDetails) {
				t.Fatalf("company edits ready case: %v", err)
			}
			assertUnchanged(t, before)
		}
	})
	t.Run("all missing and blank required fields reject approval atomically", func(t *testing.T) {
		for _, field := range []string{"selected_preference_id", "company_supervisor_id", "letter_number", "letter_date", "internship_subject", "start_date", "workplace_address", "workplace_phone"} {
			values := []any{nil}
			if field == "letter_number" || field == "internship_subject" || field == "workplace_address" || field == "workplace_phone" {
				values = append(values, " \t\n ")
			}
			for _, value := range values {
				t.Run(fmt.Sprintf("%s/%v", field, value), func(t *testing.T) {
					f := create(t)
					if err := tx.Model(&f.item).Update(field, value).Error; err != nil {
						t.Fatal(err)
					}
					before := load(t, f.item.ID)
					if _, err := workflow.ApprovePlacementDetails(f.item.ID); !errors.Is(err, ErrPlacementDetailsIncomplete) {
						t.Fatalf("incomplete placement: %v", err)
					}
					assertUnchanged(t, before)
				})
			}
		}
	})
	t.Run("corrupt relationship rejected without repair", func(t *testing.T) {
		for _, name := range []string{"missing preference", "other case preference", "nonaccepted application", "other student application", "other company snapshot", "supervisor moved company", "supervisor changed role"} {
			t.Run(name, func(t *testing.T) {
				f := create(t)
				var mutation *gorm.DB
				switch name {
				case "missing preference":
					mutation = tx.Model(&f.item).Update("selected_preference_id", 0)
				case "other case preference":
					other := create(t)
					mutation = tx.Model(&f.item).Update("selected_preference_id", other.preference.ID)
				case "nonaccepted application":
					mutation = tx.Model(&f.application).Update("status", model.ApplicationStatusRejected)
				case "other student application":
					mutation = tx.Model(&f.application).Update("student_id", professor.ID)
				case "other company snapshot":
					mutation = tx.Model(&f.item).Update("company_supervisor_id", otherSupervisor.ID)
				case "supervisor moved company":
					mutation = tx.Model(&model.User{}).Where("id = ?", supervisor.ID).Update("company_id", otherCompany.ID)
					defer func() {
						if err := tx.Model(&model.User{}).Where("id = ?", supervisor.ID).Update("company_id", company.ID).Error; err != nil {
							t.Fatal(err)
						}
					}()
				case "supervisor changed role":
					mutation = tx.Model(&model.User{}).Where("id = ?", supervisor.ID).Update("role", model.RoleStudent)
					defer func() {
						if err := tx.Model(&model.User{}).Where("id = ?", supervisor.ID).Update("role", model.RoleCompanySupervisor).Error; err != nil {
							t.Fatal(err)
						}
					}()
				}
				if mutation.Error != nil {
					t.Fatal(mutation.Error)
				}
				before := load(t, f.item.ID)
				if _, err := workflow.ApprovePlacementDetails(f.item.ID); !errors.Is(err, ErrPlacementRelationshipInvalid) {
					t.Fatalf("corrupt placement: %v", err)
				}
				assertUnchanged(t, before)
			})
		}
	})
	t.Run("blank correction rejected", func(t *testing.T) {
		f := create(t)
		for _, comment := range []string{"", " \t\n ", "\u2003\u00a0"} {
			before := load(t, f.item.ID)
			if _, err := workflow.RequestPlacementCorrection(f.item.ID, comment); !errors.Is(err, ErrCompanyDetailsRevisionCommentRequired) {
				t.Fatalf("blank comment: %v", err)
			}
			assertUnchanged(t, before)
		}
	})
	t.Run("correction resubmission approval loop preserves ownership and related rows", func(t *testing.T) {
		f := create(t)
		before, external := load(t, f.item.ID), externalSnapshot(t, f)
		comment := "آدرس و موضوع را اصلاح کنید"
		result, err := workflow.RequestPlacementCorrection(f.item.ID, " \t"+comment+" \n")
		if err != nil || result.Status != model.InternshipCaseStatusPendingCompanyDetails {
			t.Fatalf("correction: %+v %v", result, err)
		}
		assertChange(t, before, model.InternshipCaseStatusPendingCompanyDetails, &comment)
		if !reflect.DeepEqual(external, externalSnapshot(t, f)) {
			t.Fatal("correction changed unrelated rows")
		}
		pending, err := workflow.ListPendingCompanyDetailsCases(supervisor.ID)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, item := range pending {
			if item.ID == f.item.ID {
				found = true
			}
		}
		if !found {
			t.Fatal("corrected case absent from assigned company's pending list")
		}
		view, err := workflow.GetCompanyCase(supervisor.ID, f.item.ID)
		if err != nil || view.CompanyDetailsRevisionComment == nil || *view.CompanyDetailsRevisionComment != comment || view.InternshipSubject == nil {
			t.Fatalf("company correction view: %+v %v", view, err)
		}
		before = load(t, f.item.ID)
		if _, err := workflow.RequestPlacementCorrection(f.item.ID, "دوباره"); !errors.Is(err, ErrCaseNotPendingFinalApproval) {
			t.Fatalf("repeated correction: %v", err)
		}
		assertUnchanged(t, before)
		input := PlacementDetailsInput{InternshipSubject: "موضوع اصلاح‌شده", StartDate: *before.StartDate, WorkplaceAddress: "آدرس اصلاح‌شده", WorkplacePhone: *before.WorkplacePhone}
		if _, err := workflow.SubmitPlacementDetails(otherSupervisor.ID, f.item.ID, input); !errors.Is(err, ErrCaseNotAssignedToCompany) {
			t.Fatalf("cross-company resubmission: %v", err)
		}
		assertUnchanged(t, before)
		result, err = workflow.SubmitPlacementDetails(supervisor.ID, f.item.ID, input)
		if err != nil || result.Status != model.InternshipCaseStatusPendingFinalApproval || result.CompanyDetailsRevisionComment != nil {
			t.Fatalf("resubmission: %+v %v", result, err)
		}
		before.InternshipSubject, before.WorkplaceAddress = &input.InternshipSubject, &input.WorkplaceAddress
		assertChange(t, before, model.InternshipCaseStatusPendingFinalApproval, nil)
		before = load(t, f.item.ID)
		result, err = workflow.ApprovePlacementDetails(f.item.ID)
		if err != nil || result.Status != model.InternshipCaseStatusReadyToStart {
			t.Fatalf("corrected approval: %+v %v", result, err)
		}
		assertChange(t, before, model.InternshipCaseStatusReadyToStart, nil)
		if !reflect.DeepEqual(external, externalSnapshot(t, f)) {
			t.Fatal("loop changed unrelated rows")
		}
		for _, get := range []func() (*model.InternshipCase, error){
			func() (*model.InternshipCase, error) { return workflow.GetCurrentCase(f.student.ID) },
			func() (*model.InternshipCase, error) { return workflow.GetUniversityCase(f.item.ID) },
			func() (*model.InternshipCase, error) { return workflow.GetCompanyCase(supervisor.ID, f.item.ID) },
		} {
			view, err := get()
			if err != nil || view.Status != model.InternshipCaseStatusReadyToStart || *view.InternshipSubject != input.InternshipSubject || *view.WorkplaceAddress != input.WorkplaceAddress || view.ActivatedAt != nil {
				t.Fatalf("corrected final visibility: %+v %v", view, err)
			}
		}
	})
	t.Run("only pending final approval accepts either action", func(t *testing.T) {
		f := create(t)
		for _, status := range []model.InternshipCaseStatus{model.InternshipCaseStatusDraft, model.InternshipCaseStatusPendingUniversityReview, model.InternshipCaseStatusPendingCompanyDetails, model.InternshipCaseStatusReadyToStart, model.InternshipCaseStatusActive, model.InternshipCaseStatusCompleted, model.InternshipCaseStatusCancelled} {
			if err := tx.Model(&f.item).Update("status", status).Error; err != nil {
				t.Fatal(err)
			}
			before := load(t, f.item.ID)
			if _, err := workflow.ApprovePlacementDetails(f.item.ID); !errors.Is(err, ErrCaseNotPendingFinalApproval) {
				t.Fatalf("approve from %s: %v", status, err)
			}
			assertUnchanged(t, before)
			if _, err := workflow.RequestPlacementCorrection(f.item.ID, "اصلاح"); !errors.Is(err, ErrCaseNotPendingFinalApproval) {
				t.Fatalf("correct from %s: %v", status, err)
			}
			assertUnchanged(t, before)
		}
	})
	t.Run("pending final list excludes other states", func(t *testing.T) {
		f := create(t)
		cases, err := workflow.ListPendingFinalApprovalCases()
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, item := range cases {
			if item.Status != model.InternshipCaseStatusPendingFinalApproval {
				t.Fatalf("unexpected list state: %s", item.Status)
			}
			if item.ID == f.item.ID {
				found = true
			}
		}
		if !found {
			t.Fatal("pending final case missing")
		}
	})
	t.Run("missing case", func(t *testing.T) {
		if _, err := workflow.ApprovePlacementDetails(^uint(0) >> 1); !errors.Is(err, ErrCaseNotFound) {
			t.Fatalf("approve missing: %v", err)
		}
		if _, err := workflow.RequestPlacementCorrection(^uint(0)>>1, "اصلاح"); !errors.Is(err, ErrCaseNotFound) {
			t.Fatalf("correct missing: %v", err)
		}
	})
}
