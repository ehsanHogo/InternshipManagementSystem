package service_test

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

// Separate connections and committed fixtures exercise actual PostgreSQL locks.
// The temporary schema is the only schema created/dropped by this test.
func TestWeeklyReportsConcurrentMutations(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	rootSQL, err := root.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer rootSQL.Close()
	suffix := time.Now().UnixNano()
	schema := fmt.Sprintf("weekly_concurrency_%d", suffix)
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
	db, err := gorm.Open(postgres.Open(scopedDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(4)
	if err := db.AutoMigrate(&model.Company{}, &model.User{}, &model.InternshipOpportunity{}, &model.ProfessorAssignment{}, &model.File{}, &model.OpportunityApplication{}, &model.InternshipCase{}, &model.InternshipPreference{}, &model.WeeklyReport{}, &model.CompanyEvaluation{}); err != nil {
		t.Fatal(err)
	}
	columns, err := db.Migrator().ColumnTypes(&model.WeeklyReport{})
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range columns {
		switch column.Name() {
		case "status", "is_confirmed", "confirmed_at", "supervisor_comment":
			t.Fatalf("obsolete/persisted derived column: %s", column.Name())
		}
	}
	professor := createTestUser(t, db, suffix, "concurrent-professor", model.RoleProfessor)
	supervisor := createTestUser(t, db, suffix, "concurrent-supervisor", model.RoleCompanySupervisor)
	item := createProfessorReviewCase(t, db, suffix, professor.ID, supervisor.ID, 0, 0, false, false)
	workflow := service.NewInternshipService(db)
	input := service.WeeklyReportInput{WeekNumber: 1, StartDate: testDate(t, "2030-01-01"), EndDate: testDate(t, "2030-01-01"), ActivityDescription: "original"}
	var week int
	create := func(t *testing.T, submitted bool) *model.WeeklyReport {
		t.Helper()
		week++
		input.WeekNumber = week
		report, err := workflow.CreateWeeklyReport(item.StudentID, input)
		if err != nil {
			t.Fatal(err)
		}
		if submitted {
			report, err = workflow.SubmitWeeklyReport(item.StudentID, report.ID)
			if err != nil {
				t.Fatal(err)
			}
		}
		return report
	}
	race := func(t *testing.T, first, second func() error) [2]error {
		t.Helper()
		start := make(chan struct{})
		a, b := make(chan error, 1), make(chan error, 1)
		go func() { <-start; a <- first() }()
		go func() { <-start; b <- second() }()
		close(start)
		timeout := time.NewTimer(10 * time.Second)
		defer timeout.Stop()
		var results [2]error
		for i, ch := range []chan error{a, b} {
			select {
			case results[i] = <-ch:
			case <-timeout.C:
				t.Fatal("concurrent mutations timed out")
			}
		}
		return results
	}
	load := func(t *testing.T, id uint) model.WeeklyReport {
		t.Helper()
		var report model.WeeklyReport
		if err := db.First(&report, id).Error; err != nil {
			t.Fatal(err)
		}
		return report
	}
	t.Run("independent approvals retain both decisions", func(t *testing.T) {
		report := create(t, true)
		results := race(t, func() error {
			_, err := workflow.ReviewWeeklyReportByCompany(supervisor.ID, item.ID, report.ID, model.WeeklyReviewApproved, nil)
			return err
		}, func() error {
			_, err := workflow.ReviewWeeklyReportByProfessor(professor.ID, item.ID, report.ID, model.WeeklyReviewApproved, nil)
			return err
		})
		if results[0] != nil || results[1] != nil || load(t, report.ID).Status() != model.WeeklyReportApproved {
			t.Fatalf("simultaneous approvals: %v", results)
		}
	})
	t.Run("repeat approval has one winner", func(t *testing.T) {
		report := create(t, true)
		approve := func() error {
			_, err := workflow.ReviewWeeklyReportByCompany(supervisor.ID, item.ID, report.ID, model.WeeklyReviewApproved, nil)
			return err
		}
		results := race(t, approve, approve)
		if !((results[0] == nil && errors.Is(results[1], service.ErrWeeklyReportState)) || (results[1] == nil && errors.Is(results[0], service.ErrWeeklyReportState))) {
			t.Fatalf("repeat approvals: %v", results)
		}
	})
	for _, revisionRole := range []string{"company", "professor"} {
		t.Run("revision blocks later approval "+revisionRole, func(t *testing.T) {
			report := create(t, true)
			comment := "fix content"
			revision := func() error {
				_, err := workflow.ReviewWeeklyReportByCompany(supervisor.ID, item.ID, report.ID, model.WeeklyReviewRevisionRequested, &comment)
				return err
			}
			approve := func() error {
				_, err := workflow.ReviewWeeklyReportByProfessor(professor.ID, item.ID, report.ID, model.WeeklyReviewApproved, nil)
				return err
			}
			if revisionRole == "professor" {
				revision = func() error {
					_, err := workflow.ReviewWeeklyReportByProfessor(professor.ID, item.ID, report.ID, model.WeeklyReviewRevisionRequested, &comment)
					return err
				}
				approve = func() error {
					_, err := workflow.ReviewWeeklyReportByCompany(supervisor.ID, item.ID, report.ID, model.WeeklyReviewApproved, nil)
					return err
				}
			}
			results := race(t, revision, approve)
			if results[0] != nil || (results[1] != nil && !errors.Is(results[1], service.ErrWeeklyReportState)) || load(t, report.ID).Status() != model.WeeklyReportRevisionRequested {
				t.Fatalf("revision/approval: %v", results)
			}
			if err := approve(); !errors.Is(err, service.ErrWeeklyReportState) {
				t.Fatalf("old version review allowed: %v", err)
			}
		})
	}
	t.Run("edit and submit preserve stable submitted content", func(t *testing.T) {
		report := create(t, false)
		updated := input
		updated.ActivityDescription = "edited"
		results := race(t, func() error { _, err := workflow.UpdateWeeklyReport(item.StudentID, report.ID, updated); return err }, func() error { _, err := workflow.SubmitWeeklyReport(item.StudentID, report.ID); return err })
		persisted := load(t, report.ID)
		if results[1] != nil || (results[0] != nil && !errors.Is(results[0], service.ErrWeeklyReportState)) || persisted.Status() != model.WeeklyReportSubmitted {
			t.Fatalf("edit/submit: %v", results)
		}
		expected := "original"
		if results[0] == nil {
			expected = "edited"
		}
		if persisted.ActivityDescription != expected {
			t.Fatalf("submitted content=%q want=%q", persisted.ActivityDescription, expected)
		}
		if _, err := workflow.UpdateWeeklyReport(item.StudentID, report.ID, updated); !errors.Is(err, service.ErrWeeklyReportState) {
			t.Fatalf("submitted edit allowed: %v", err)
		}
	})

	t.Run("resubmit and review serialize", func(t *testing.T) {
		report := create(t, true)
		comment := "correct this report"
		if _, err := workflow.ReviewWeeklyReportByProfessor(professor.ID, item.ID, report.ID, model.WeeklyReviewRevisionRequested, &comment); err != nil {
			t.Fatal(err)
		}
		results := race(t, func() error {
			_, err := workflow.SubmitWeeklyReport(item.StudentID, report.ID)
			return err
		}, func() error {
			_, err := workflow.ReviewWeeklyReportByCompany(supervisor.ID, item.ID, report.ID, model.WeeklyReviewApproved, nil)
			return err
		})
		persisted := load(t, report.ID)
		expectedCompany := model.WeeklyReviewPending
		if results[1] == nil {
			expectedCompany = model.WeeklyReviewApproved
		}
		if results[0] != nil || (results[1] != nil && !errors.Is(results[1], service.ErrWeeklyReportState)) || persisted.Status() != model.WeeklyReportSubmitted || persisted.CompanyReviewStatus != expectedCompany || persisted.ProfessorReviewStatus != model.WeeklyReviewPending {
			t.Fatalf("resubmit/review: results=%v report=%+v", results, persisted)
		}
	})
	t.Run("duplicate creation has one winner", func(t *testing.T) {
		input.WeekNumber = 8
		create := func() error { _, err := workflow.CreateWeeklyReport(item.StudentID, input); return err }
		results := race(t, create, create)
		if !((results[0] == nil && errors.Is(results[1], service.ErrDuplicateWeeklyReport)) || (results[1] == nil && errors.Is(results[0], service.ErrDuplicateWeeklyReport))) {
			t.Fatalf("duplicate creation: %v", results)
		}
	})
}
