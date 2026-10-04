package service_test

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

// A fresh, isolated schema and committed fixtures exercise real PostgreSQL
// constraints, transactions, and simultaneous final-evaluation requests.
func TestProfessorFinalEvaluation(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	rootSQL, err := root.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer rootSQL.Close()
	suffix := time.Now().UnixNano()
	schema := fmt.Sprintf("professor_final_%d", suffix)
	if err := root.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
	}()
	scopedDSN := dsn + " search_path=" + schema
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		scopedDSN = parsed.String()
	}
	db, err := gorm.Open(postgres.Open(scopedDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(4)
	if err := db.AutoMigrate(&model.Company{}, &model.User{}, &model.InternshipOpportunity{}, &model.ProfessorAssignment{}, &model.File{}, &model.OpportunityApplication{}, &model.InternshipCase{}, &model.InternshipPreference{}, &model.WeeklyReport{}, &model.CompanyEvaluation{}, &model.FinalReport{}); err != nil {
		t.Fatal(err)
	}
	professor := createTestUser(t, db, suffix, "final-professor", model.RoleProfessor)
	otherProfessor := createTestUser(t, db, suffix, "final-other-professor", model.RoleProfessor)
	supervisor := createTestUser(t, db, suffix, "final-supervisor", model.RoleCompanySupervisor)
	workflow := service.NewInternshipService(db)
	applications := service.NewOpportunityApplicationService(db)
	var counter int64
	fixture := func(t *testing.T) model.InternshipCase {
		t.Helper()
		counter++
		item := createProfessorReviewCase(t, db, suffix+counter, professor.ID, supervisor.ID, 8, 8, true, true)
		createAssignment(t, db, item.StudentID, otherProfessor.ID) // Snapshot wins over current assignment.
		now := time.Now().Add(-24 * time.Hour).UTC()
		if err := db.Model(&item).Updates(map[string]any{"activated_at": now, "submitted_at": now, "letter_number": "letter-12", "letter_date": now, "internship_subject": "subject", "start_date": now, "workplace_address": "address", "workplace_phone": "02112345678"}).Error; err != nil {
			t.Fatal(err)
		}
		return item
	}
	load := func(t *testing.T, id uint) *model.InternshipCase {
		t.Helper()
		item, err := workflow.GetUniversityCase(id)
		if err != nil {
			t.Fatal(err)
		}
		return item
	}
	unchanged := func(t *testing.T, before, after *model.InternshipCase) {
		t.Helper()
		if !reflect.DeepEqual(before, after) {
			t.Fatal("rejected action changed case or related data")
		}
	}
	finalize := func(t *testing.T, item model.InternshipCase, result model.ProfessorFinalResult) *model.InternshipCase {
		t.Helper()
		comment := "  overall professor comment  "
		actual, err := workflow.CompleteProfessorCase(professor.ID, item.ID, service.ProfessorCompletionInput{Result: result, Comment: &comment})
		if err != nil {
			t.Fatal(err)
		}
		return actual
	}
	opportunity := func(t *testing.T, label string) model.InternshipOpportunity {
		t.Helper()
		var user model.User
		if err := db.First(&user, supervisor.ID).Error; err != nil {
			t.Fatal(err)
		}
		return finalEvaluationTestOpportunity(t, db, suffix, label, *user.CompanyID, supervisor.ID, model.OpportunityStatusOpen)
	}

	for _, result := range []model.ProfessorFinalResult{model.ProfessorFinalResultExcellent, model.ProfessorFinalResultGood, model.ProfessorFinalResultFailed} {
		t.Run(string(result)+" mapping, invariants, history and terminal guards", func(t *testing.T) {
			item := fixture(t)
			before := load(t, item.ID)
			var companyBefore model.Company
			companyID := before.SelectedPreference.OpportunityApplication.Opportunity.CompanyID
			if err := db.First(&companyBefore, companyID).Error; err != nil {
				t.Fatal(err)
			}
			if companyBefore.IsApproved {
				t.Fatal("company fixture must be unapproved")
			}
			var studentBefore, supervisorBefore model.User
			if err := db.First(&studentBefore, item.StudentID).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.First(&supervisorBefore, supervisor.ID).Error; err != nil {
				t.Fatal(err)
			}
			actual := finalize(t, item, result)
			expected := model.InternshipCaseStatusPassed
			if result == model.ProfessorFinalResultFailed {
				expected = model.InternshipCaseStatusFailed
			}
			if actual.Status != expected || actual.FinalResult == nil || *actual.FinalResult != result || actual.CompletedAt == nil || actual.ProfessorComment == nil || *actual.ProfessorComment != "overall professor comment" {
				t.Fatalf("incorrect final evaluation: %+v", actual)
			}
			// Compare the entire reloaded graph after masking only the four permitted fields.
			after := load(t, item.ID)
			after.Status, after.FinalResult, after.ProfessorComment, after.CompletedAt = before.Status, before.FinalResult, before.ProfessorComment, before.CompletedAt
			unchanged(t, before, after)
			var companyAfter model.Company
			if err := db.First(&companyAfter, companyID).Error; err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(companyBefore, companyAfter) {
				t.Fatal("final evaluation changed company approval/data")
			}
			var studentAfter, supervisorAfter model.User
			if err := db.First(&studentAfter, item.StudentID).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.First(&supervisorAfter, supervisor.ID).Error; err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(studentBefore, studentAfter) || !reflect.DeepEqual(supervisorBefore, supervisorAfter) {
				t.Fatal("final evaluation changed user/company linkage")
			}
			terminal := load(t, item.ID)
			for _, retry := range []model.ProfessorFinalResult{model.ProfessorFinalResultGood, model.ProfessorFinalResultFailed} {
				if _, err := workflow.CompleteProfessorCase(professor.ID, item.ID, service.ProfessorCompletionInput{Result: retry}); !errors.Is(err, service.ErrProfessorCaseNotActive) {
					t.Fatalf("reevaluation error = %v", err)
				}
				unchanged(t, terminal, load(t, item.ID))
			}
			if _, err := workflow.ReviewWeeklyReportByProfessor(professor.ID, item.ID, before.WeeklyReports[0].ID, model.WeeklyReviewApproved, nil); !errors.Is(err, service.ErrInvalidCaseStatus) {
				t.Fatalf("terminal weekly review = %v", err)
			}
			if _, err := workflow.ReviewWeeklyReportByCompany(supervisor.ID, item.ID, before.WeeklyReports[0].ID, model.WeeklyReviewApproved, nil); !errors.Is(err, service.ErrInvalidCaseStatus) {
				t.Fatalf("terminal company review = %v", err)
			}
			if _, err := workflow.ReviewFinalReport(professor.ID, item.ID, model.FinalReportApproved, nil); !errors.Is(err, service.ErrProfessorCaseNotActive) {
				t.Fatalf("terminal final report review = %v", err)
			}
			if _, _, err := workflow.AttachFinalReport(item.StudentID, finalEvaluationTestResume(suffix, "terminal-replacement")); !errors.Is(err, service.ErrInvalidCaseStatus) {
				t.Fatalf("terminal final report replacement = %v", err)
			}
			if _, err := workflow.SubmitWeeklyReport(item.StudentID, before.WeeklyReports[0].ID); !errors.Is(err, service.ErrInvalidCaseStatus) {
				t.Fatalf("terminal student weekly mutation = %v", err)
			}
			if _, err := workflow.SubmitPlacementDetails(supervisor.ID, item.ID, service.PlacementDetailsInput{InternshipSubject: "changed", StartDate: time.Now(), WorkplaceAddress: "changed", WorkplacePhone: "02112345678"}); !errors.Is(err, service.ErrCaseNotPendingCompanyDetails) {
				t.Fatalf("terminal placement mutation = %v", err)
			}
			unchanged(t, terminal, load(t, item.ID))
			if _, err := workflow.GetCompanyCase(supervisor.ID, item.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := workflow.GetProfessorCase(professor.ID, item.ID); err != nil {
				t.Fatal(err)
			}
			open := opportunity(t, "after-"+string(result))
			if expected == model.InternshipCaseStatusPassed {
				if _, created, err := workflow.CreateOrGetCase(item.StudentID); !errors.Is(err, service.ErrInternshipPassed) || created {
					t.Fatalf("passed case creation: %v", err)
				}
				if _, err := applications.Apply(item.StudentID, open.ID, finalEvaluationTestResume(suffix, "passed-"+string(result))); !errors.Is(err, service.ErrInternshipAlreadyPassed) {
					t.Fatalf("passed application: %v", err)
				}
				browsed, err := service.NewOpportunityService(db).GetStudent(open.ID)
				if err != nil || browsed.ID != open.ID {
					t.Fatalf("passed browsing: %v", err)
				}
			} else {
				if _, err := applications.Apply(item.StudentID, open.ID, finalEvaluationTestResume(suffix, "failed-retry")); err != nil {
					t.Fatalf("failed history blocked application: %v", err)
				}
				draft, created, err := workflow.CreateOrGetCase(item.StudentID)
				if err != nil || !created || draft.ID == item.ID || draft.Status != model.InternshipCaseStatusDraft {
					t.Fatalf("failed retry draft: %+v %v", draft, err)
				}
				current, createdAgain, err := workflow.CreateOrGetCase(item.StudentID)
				if err != nil || createdAgain || current.ID != draft.ID {
					t.Fatal("retry created more than one current draft")
				}
				if _, err := workflow.ReplacePreferences(item.StudentID, []uint{before.SelectedPreference.OpportunityApplicationID}); err != nil {
					t.Fatalf("accepted application reuse: %v", err)
				}
				if err := db.Model(draft).Update("status", model.InternshipCaseStatusPendingUniversityReview).Error; err != nil {
					t.Fatal(err)
				}
				if eligibility, err := applications.CheckEligibility(item.StudentID, open.ID); err != nil || eligibility.RestrictionCode != service.ApplicationRestrictionCaseActive {
					t.Fatalf("new current case must block recruitment: %+v %v", eligibility, err)
				}
				unchanged(t, terminal, load(t, item.ID))
			}
			history, err := workflow.ListStudentHistoricalCases(item.StudentID)
			if err != nil || len(history) != 1 || history[0].ID != item.ID || len(history[0].WeeklyReports) != 8 || history[0].FinalReport == nil || history[0].CompanyEvaluation == nil {
				t.Fatalf("terminal history missing after retry: %+v %v", history, err)
			}
		})
	}

	blockers := []struct {
		name   string
		mutate func(*testing.T, model.InternshipCase)
		want   error
	}{
		{"missing week", func(t *testing.T, item model.InternshipCase) {
			if err := db.Where("internship_case_id = ? AND week_number = 8", item.ID).Delete(&model.WeeklyReport{}).Error; err != nil {
				t.Fatal(err)
			}
		}, service.ErrProfessorWeeklyReportsIncomplete},
		{"company pending", func(t *testing.T, item model.InternshipCase) {
			if err := db.Model(&model.WeeklyReport{}).Where("internship_case_id = ? AND week_number = 8", item.ID).Update("company_review_status", model.WeeklyReviewPending).Error; err != nil {
				t.Fatal(err)
			}
		}, service.ErrProfessorWeeklyReportsIncomplete},
		{"professor pending", func(t *testing.T, item model.InternshipCase) {
			if err := db.Model(&model.WeeklyReport{}).Where("internship_case_id = ? AND week_number = 8", item.ID).Update("professor_review_status", model.WeeklyReviewPending).Error; err != nil {
				t.Fatal(err)
			}
		}, service.ErrProfessorWeeklyReportsIncomplete},
		{"company revision", func(t *testing.T, item model.InternshipCase) {
			if err := db.Model(&model.WeeklyReport{}).Where("internship_case_id = ? AND week_number = 8", item.ID).Update("company_review_status", model.WeeklyReviewRevisionRequested).Error; err != nil {
				t.Fatal(err)
			}
		}, service.ErrProfessorWeeklyReportsIncomplete},
		{"professor revision", func(t *testing.T, item model.InternshipCase) {
			if err := db.Model(&model.WeeklyReport{}).Where("internship_case_id = ? AND week_number = 8", item.ID).Update("professor_review_status", model.WeeklyReviewRevisionRequested).Error; err != nil {
				t.Fatal(err)
			}
		}, service.ErrProfessorWeeklyReportsIncomplete},
		{"no company evaluation", func(t *testing.T, item model.InternshipCase) {
			if err := db.Where("internship_case_id = ?", item.ID).Delete(&model.CompanyEvaluation{}).Error; err != nil {
				t.Fatal(err)
			}
		}, service.ErrProfessorCompanyEvaluationRequired},
		{"no final report", func(t *testing.T, item model.InternshipCase) {
			if err := db.Where("internship_case_id = ?", item.ID).Delete(&model.FinalReport{}).Error; err != nil {
				t.Fatal(err)
			}
		}, service.ErrProfessorFinalReportRequired},
		{"final report submitted", func(t *testing.T, item model.InternshipCase) {
			if err := db.Model(&model.FinalReport{}).Where("internship_case_id = ?", item.ID).Update("status", model.FinalReportSubmitted).Error; err != nil {
				t.Fatal(err)
			}
		}, service.ErrProfessorFinalReportRequired},
		{"final report revision", func(t *testing.T, item model.InternshipCase) {
			if err := db.Model(&model.FinalReport{}).Where("internship_case_id = ?", item.ID).Update("status", model.FinalReportRevisionRequested).Error; err != nil {
				t.Fatal(err)
			}
		}, service.ErrProfessorFinalReportRequired},
	}
	for _, block := range blockers {
		t.Run(block.name, func(t *testing.T) {
			item := fixture(t)
			block.mutate(t, item)
			before := load(t, item.ID)
			if _, err := workflow.CompleteProfessorCase(professor.ID, item.ID, service.ProfessorCompletionInput{Result: model.ProfessorFinalResultExcellent}); !errors.Is(err, block.want) {
				t.Fatalf("error = %v, want %v", err, block.want)
			}
			unchanged(t, before, load(t, item.ID))
		})
	}

	t.Run("snapshot authorization and invalid results do not mutate", func(t *testing.T) {
		item := fixture(t)
		before := load(t, item.ID)
		if _, err := workflow.CompleteProfessorCase(otherProfessor.ID, item.ID, service.ProfessorCompletionInput{Result: model.ProfessorFinalResultGood}); !errors.Is(err, service.ErrCaseNotAssignedToProfessor) {
			t.Fatalf("wrong professor = %v", err)
		}
		for _, result := range []model.ProfessorFinalResult{"", "UNKNOWN", "18", " GOOD "} {
			if _, err := workflow.CompleteProfessorCase(professor.ID, item.ID, service.ProfessorCompletionInput{Result: result}); !errors.Is(err, service.ErrInvalidProfessorResult) {
				t.Fatalf("invalid result = %v", err)
			}
		}
		unchanged(t, before, load(t, item.ID))
	})
	for _, status := range []model.InternshipCaseStatus{model.InternshipCaseStatusDraft, model.InternshipCaseStatusPendingUniversityReview, model.InternshipCaseStatusPendingCompanyDetails, model.InternshipCaseStatusPendingFinalApproval, model.InternshipCaseStatusReadyToStart, model.InternshipCaseStatusCancelled} {
		t.Run("cannot finalize "+string(status), func(t *testing.T) {
			item := fixture(t)
			if err := db.Model(&item).Update("status", status).Error; err != nil {
				t.Fatal(err)
			}
			var before model.InternshipCase
			if err := db.First(&before, item.ID).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := workflow.CompleteProfessorCase(professor.ID, item.ID, service.ProfessorCompletionInput{Result: model.ProfessorFinalResultGood}); !errors.Is(err, service.ErrProfessorCaseNotActive) {
				t.Fatalf("inactive error = %v", err)
			}
			var after model.InternshipCase
			if err := db.First(&after, item.ID).Error; err != nil {
				t.Fatal(err)
			}
			unchanged(t, &before, &after)
		})
	}
	t.Run("passed history takes priority over failed and draft", func(t *testing.T) {
		item := fixture(t)
		finalize(t, item, model.ProfessorFinalResultFailed)
		finalEvaluationTestCase(t, db, item.StudentID, professor.ID, model.InternshipCaseStatusPassed)
		finalEvaluationTestCase(t, db, item.StudentID, professor.ID, model.InternshipCaseStatusDraft)
		open := opportunity(t, "passed-priority")
		if _, created, err := workflow.CreateOrGetCase(item.StudentID); !errors.Is(err, service.ErrInternshipPassed) || created {
			t.Fatalf("passed priority case: %v", err)
		}
		if _, err := applications.Apply(item.StudentID, open.ID, finalEvaluationTestResume(suffix, "passed-priority")); !errors.Is(err, service.ErrInternshipAlreadyPassed) {
			t.Fatalf("passed priority application: %v", err)
		}
	})
	t.Run("schema rejects obsolete completed", func(t *testing.T) {
		item := fixture(t)
		if err := db.Model(&item).Update("status", "COMPLETED").Error; err == nil {
			t.Fatal("clean schema accepted obsolete status")
		}
		if load(t, item.ID).Status != model.InternshipCaseStatusActive {
			t.Fatal("invalid schema update mutated case")
		}
	})
	t.Run("concurrent final evaluations have exactly one winner", func(t *testing.T) {
		item := fixture(t)
		start := make(chan struct{})
		out := make(chan struct {
			result  model.ProfessorFinalResult
			comment string
			err     error
		}, 2)
		for _, result := range []model.ProfessorFinalResult{model.ProfessorFinalResultGood, model.ProfessorFinalResultFailed} {
			go func(result model.ProfessorFinalResult) {
				<-start
				comment := "winner " + string(result)
				_, err := workflow.CompleteProfessorCase(professor.ID, item.ID, service.ProfessorCompletionInput{Result: result, Comment: &comment})
				out <- struct {
					result  model.ProfessorFinalResult
					comment string
					err     error
				}{result, comment, err}
			}(result)
		}
		close(start)
		wins := 0
		for i := 0; i < 2; i++ {
			select {
			case response := <-out:
				if response.err == nil {
					wins++
					persisted := load(t, item.ID)
					if persisted.FinalResult == nil || *persisted.FinalResult != response.result || persisted.ProfessorComment == nil || *persisted.ProfessorComment != response.comment || persisted.CompletedAt == nil {
						t.Fatal("winner fields overwritten")
					}
				} else if !errors.Is(response.err, service.ErrProfessorCaseNotActive) {
					t.Fatalf("loser error = %v", response.err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("concurrent final evaluation timed out")
			}
		}
		if wins != 1 {
			t.Fatalf("success count = %d", wins)
		}
	})
}

func finalEvaluationTestOpportunity(t *testing.T, db *gorm.DB, suffix int64, label string, companyID, supervisorID uint, status model.OpportunityStatus) model.InternshipOpportunity {
	t.Helper()
	opportunity := model.InternshipOpportunity{CompanyID: companyID, CreatedBy: supervisorID, Title: fmt.Sprintf("فرصت %d %s", suffix, label), Description: "شرح", WorkField: "نرم‌افزار", Location: "تهران", Status: status}
	if err := db.Create(&opportunity).Error; err != nil {
		t.Fatal(err)
	}
	return opportunity
}
func finalEvaluationTestResume(suffix int64, label string) *model.File {
	return &model.File{OriginalName: label + ".pdf", StoredName: fmt.Sprintf("%d-%s.pdf", suffix, label), Path: fmt.Sprintf("/tmp/%d-%s.pdf", suffix, label), MimeType: "application/pdf", SizeBytes: 128, UploadedAt: time.Now()}
}
func finalEvaluationTestCase(t *testing.T, db *gorm.DB, studentID, professorID uint, status model.InternshipCaseStatus) {
	t.Helper()
	item := model.InternshipCase{StudentID: studentID, ProfessorID: professorID, Status: status}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
}
