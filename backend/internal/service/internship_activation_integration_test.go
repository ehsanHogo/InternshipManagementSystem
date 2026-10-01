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

func TestManualInternshipActivation(t *testing.T) {
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
	supervisor := applicationTestUser(t, tx, suffix, "final-supervisor", model.RoleCompanySupervisor, &company.ID)
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
		label := fmt.Sprintf("activation-%d", next)
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
		letter, subject, address, phone := "M9-100", "توسعه نرم‌افزار", "آدرس محل", "021-123"
		date := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
		supervisorID := supervisor.ID
		item := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusReadyToStart,
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
	// Compare complete rows, including timestamps, to detect any recruitment,
	// company approval or user affiliation mutation during university actions.
	externalSnapshot := func(t *testing.T, f fixture) []any {
		t.Helper()
		var preference model.InternshipPreference
		var application model.OpportunityApplication
		var opportunity model.InternshipOpportunity
		var companies []model.Company
		var users []model.User
		var files []model.File
		for _, query := range []*gorm.DB{
			tx.First(&preference, f.preference.ID), tx.First(&application, f.application.ID), tx.First(&opportunity, f.opportunity.ID),
			tx.Order("id").Find(&companies), tx.Order("id").Find(&users), tx.Order("id").Find(&files),
		} {
			if query.Error != nil {
				t.Fatal(query.Error)
			}
		}
		return []any{preference, application, opportunity, companies, users, files}
	}

	t.Run("successful activation allows past today and future dates and preserves all other data", func(t *testing.T) {
		for _, offset := range []int{-30, 0, 30} {
			t.Run(fmt.Sprintf("start date offset %d", offset), func(t *testing.T) {
				f := create(t)
				startDate := time.Now().UTC().AddDate(0, 0, offset)
				comment := "historical correction comment"
				submittedAt := startDate.AddDate(0, 0, -60)
				if err := tx.Model(&f.item).Updates(map[string]any{"start_date": startDate, "submitted_at": submittedAt, "company_details_revision_comment": comment}).Error; err != nil {
					t.Fatal(err)
				}
				before, external := load(t, f.item.ID), externalSnapshot(t, f)
				lowerBound := time.Now().UTC().Add(-time.Second)
				result, err := workflow.ActivateUniversityCase(f.item.ID)
				if err != nil || result == nil || result.Status != model.InternshipCaseStatusActive || result.ActivatedAt == nil {
					t.Fatalf("activate: %+v %v", result, err)
				}
				after := load(t, f.item.ID)
				if after.ActivatedAt.Before(lowerBound) || after.ActivatedAt.After(time.Now().UTC().Add(time.Second)) {
					t.Fatalf("activated_at is not now: %v", after.ActivatedAt)
				}
				before.Status, before.ActivatedAt = model.InternshipCaseStatusActive, after.ActivatedAt
				if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(external, externalSnapshot(t, f)) {
					t.Fatal("activation changed data beyond status and activated_at")
				}
				if _, err := workflow.ActivateUniversityCase(f.item.ID); !errors.Is(err, ErrCaseNotReadyToStart) {
					t.Fatalf("repeated activation: %v", err)
				}
				assertUnchanged(t, after)
				if _, err := workflow.SubmitPlacementDetails(supervisor.ID, f.item.ID, PlacementDetailsInput{}); !errors.Is(err, ErrCaseNotPendingCompanyDetails) {
					t.Fatalf("company edits active case: %v", err)
				}
				assertUnchanged(t, after)
				for _, get := range []func() (*model.InternshipCase, error){
					func() (*model.InternshipCase, error) { return workflow.GetCurrentCase(f.student.ID) },
					func() (*model.InternshipCase, error) { return workflow.GetUniversityCase(f.item.ID) },
					func() (*model.InternshipCase, error) { return workflow.GetCompanyCase(supervisor.ID, f.item.ID) },
				} {
					view, err := get()
					if err != nil || view.Status != model.InternshipCaseStatusActive || view.ActivatedAt == nil {
						t.Fatalf("active visibility: %+v %v", view, err)
					}
				}
			})
		}
	})
	t.Run("every other state rejects activation without any mutation", func(t *testing.T) {
		for _, status := range []model.InternshipCaseStatus{model.InternshipCaseStatusDraft, model.InternshipCaseStatusPendingUniversityReview, model.InternshipCaseStatusPendingCompanyDetails, model.InternshipCaseStatusPendingFinalApproval, model.InternshipCaseStatusActive, model.InternshipCaseStatusCompleted, model.InternshipCaseStatusCancelled} {
			t.Run(string(status), func(t *testing.T) {
				f := create(t)
				if err := tx.Model(&f.item).Update("status", status).Error; err != nil {
					t.Fatal(err)
				}
				before := load(t, f.item.ID)
				if _, err := workflow.ActivateUniversityCase(f.item.ID); !errors.Is(err, ErrCaseNotReadyToStart) {
					t.Fatalf("activate from %s: %v", status, err)
				}
				assertUnchanged(t, before)
			})
		}
	})
	t.Run("missing and blank essential fields reject atomically", func(t *testing.T) {
		for _, field := range []string{"selected_preference_id", "company_supervisor_id", "letter_number", "letter_date", "internship_subject", "start_date", "workplace_address", "workplace_phone"} {
			values := []any{nil}
			if field == "letter_number" || field == "internship_subject" || field == "workplace_address" || field == "workplace_phone" {
				values = append(values, " 	\n ")
			}
			for _, value := range values {
				t.Run(fmt.Sprintf("%s/%v", field, value), func(t *testing.T) {
					f := create(t)
					if err := tx.Model(&f.item).Update(field, value).Error; err != nil {
						t.Fatal(err)
					}
					before := load(t, f.item.ID)
					if _, err := workflow.ActivateUniversityCase(f.item.ID); !errors.Is(err, ErrCaseActivationIntegrityFailed) {
						t.Fatalf("incomplete activation: %v", err)
					}
					assertUnchanged(t, before)
				})
			}
		}
	})
	t.Run("corrupt selected preference rejects without repair", func(t *testing.T) {
		f, other := create(t), create(t)
		for _, preferenceID := range []uint{0, other.preference.ID} {
			if err := tx.Model(&f.item).Update("selected_preference_id", preferenceID).Error; err != nil {
				t.Fatal(err)
			}
			before := load(t, f.item.ID)
			if _, err := workflow.ActivateUniversityCase(f.item.ID); !errors.Is(err, ErrCaseActivationIntegrityFailed) {
				t.Fatalf("corrupt activation: %v", err)
			}
			assertUnchanged(t, before)
		}
	})
	t.Run("missing case", func(t *testing.T) {
		if _, err := workflow.ActivateUniversityCase(^uint(0) >> 1); !errors.Is(err, ErrCaseNotFound) {
			t.Fatalf("activate missing: %v", err)
		}
	})
}
