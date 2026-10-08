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

// Committed fixtures and separate connections exercise PostgreSQL row locks.
func TestFinalReportConcurrentMutations(t *testing.T) {
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
	schema := fmt.Sprintf("final_report_concurrency_%d", suffix)
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
	if err := db.AutoMigrate(&model.Company{}, &model.User{}, &model.Notification{}, &model.InternshipOpportunity{}, &model.ProfessorAssignment{}, &model.File{}, &model.OpportunityApplication{}, &model.InternshipCase{}, &model.StudentInternshipRating{}, &model.InternshipPreference{}, &model.WeeklyReport{}, &model.CompanyEvaluation{}, &model.FinalReport{}); err != nil {
		t.Fatal(err)
	}
	columns, err := db.Migrator().ColumnTypes(&model.InternshipCase{})
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range columns {
		if column.Name() == "final_report_file_id" {
			t.Fatal("fresh schema retained legacy final report column")
		}
	}
	professor := createTestUser(t, db, suffix, "fr-concurrent-professor", model.RoleProfessor)
	supervisor := createTestUser(t, db, suffix, "fr-concurrent-supervisor", model.RoleCompanySupervisor)
	workflow := service.NewInternshipService(db)
	var counter int64
	fixture := func(t *testing.T, ready bool) model.InternshipCase {
		t.Helper()
		counter++
		weeks := 0
		if ready {
			weeks = 8
		}
		return createProfessorReviewCase(t, db, suffix+counter, professor.ID, supervisor.ID, weeks, weeks, ready, false)
	}
	file := func(label string) *model.File {
		return &model.File{OriginalName: label + ".pdf", StoredName: fmt.Sprintf("fr-concurrent-%d-%s.pdf", suffix, label), Path: "/tmp/" + label + ".pdf", MimeType: "application/pdf", SizeBytes: 128, UploadedAt: time.Now()}
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
				t.Fatal("concurrent final report mutations timed out")
			}
		}
		return results
	}
	oneWinner := func(t *testing.T, results [2]error) {
		t.Helper()
		if !((results[0] == nil && errors.Is(results[1], service.ErrFinalReportState)) || (results[1] == nil && errors.Is(results[0], service.ErrFinalReportState))) {
			t.Fatalf("expected one winner: %v", results)
		}
	}
	load := func(t *testing.T, caseID uint) model.FinalReport {
		t.Helper()
		var report model.FinalReport
		if err := db.Where("internship_case_id = ?", caseID).First(&report).Error; err != nil {
			t.Fatal(err)
		}
		return report
	}
	fileCount := func(t *testing.T) int64 {
		t.Helper()
		var count int64
		if err := db.Model(&model.File{}).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		return count
	}
	upload := func(studentID uint, uploaded *model.File) error {
		_, _, err := workflow.AttachFinalReport(studentID, uploaded)
		return err
	}

	t.Run("duplicate first uploads have one winner and no orphan metadata", func(t *testing.T) {
		item := fixture(t, true)
		before := fileCount(t)
		a, b := file("first-a"), file("first-b")
		results := race(t, func() error { return upload(item.StudentID, a) }, func() error { return upload(item.StudentID, b) })
		oneWinner(t, results)
		report := load(t, item.ID)
		if report.Status != model.FinalReportSubmitted || fileCount(t) != before+1 {
			t.Fatal("concurrent first upload left partial state")
		}
		expected := a.ID
		if results[1] == nil {
			expected = b.ID
		}
		if report.CurrentFileID != expected {
			t.Fatal("current PDF differs from successful upload")
		}
	})

	t.Run("competing professor decisions cannot overwrite", func(t *testing.T) {
		item := fixture(t, true)
		if err := upload(item.StudentID, file("review")); err != nil {
			t.Fatal(err)
		}
		comment := "correct content"
		results := race(t, func() error {
			_, err := workflow.ReviewFinalReport(professor.ID, item.ID, model.FinalReportApproved, nil)
			return err
		}, func() error {
			_, err := workflow.ReviewFinalReport(professor.ID, item.ID, model.FinalReportRevisionRequested, &comment)
			return err
		})
		oneWinner(t, results)
		report := load(t, item.ID)
		expected := model.FinalReportApproved
		if results[1] == nil {
			expected = model.FinalReportRevisionRequested
		}
		if report.Status != expected || report.ReviewedAt == nil {
			t.Fatal("winning professor decision was overwritten")
		}
	})

	t.Run("duplicate corrections replace once", func(t *testing.T) {
		item := fixture(t, true)
		old := file("correction-old")
		if err := upload(item.StudentID, old); err != nil {
			t.Fatal(err)
		}
		comment := "correct"
		if _, err := workflow.ReviewFinalReport(professor.ID, item.ID, model.FinalReportRevisionRequested, &comment); err != nil {
			t.Fatal(err)
		}
		before := fileCount(t)
		a, b := file("correction-a"), file("correction-b")
		results := race(t, func() error { return upload(item.StudentID, a) }, func() error { return upload(item.StudentID, b) })
		oneWinner(t, results)
		report := load(t, item.ID)
		if report.Status != model.FinalReportSubmitted || report.CurrentFileID == old.ID || fileCount(t) != before {
			t.Fatal("concurrent correction left extra files or wrong state")
		}
		if err := db.First(&model.File{}, old.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("old metadata survived: %v", err)
		}
	})

	t.Run("correction and review serialize on the current content", func(t *testing.T) {
		item := fixture(t, true)
		if err := upload(item.StudentID, file("resubmit-old")); err != nil {
			t.Fatal(err)
		}
		comment := "correct"
		if _, err := workflow.ReviewFinalReport(professor.ID, item.ID, model.FinalReportRevisionRequested, &comment); err != nil {
			t.Fatal(err)
		}
		corrected := file("resubmit-new")
		results := race(t, func() error { return upload(item.StudentID, corrected) }, func() error {
			_, err := workflow.ReviewFinalReport(professor.ID, item.ID, model.FinalReportApproved, nil)
			return err
		})
		if results[0] != nil || (results[1] != nil && !errors.Is(results[1], service.ErrFinalReportState)) {
			t.Fatalf("correction/review: %v", results)
		}
		report := load(t, item.ID)
		expected := model.FinalReportSubmitted
		if results[1] == nil {
			expected = model.FinalReportApproved
		}
		if report.Status != expected || report.CurrentFileID != corrected.ID {
			t.Fatal("approved content differs from corrected PDF")
		}
	})

	t.Run("completion cannot race past final approval", func(t *testing.T) {
		item := fixture(t, true)
		if err := upload(item.StudentID, file("completion")); err != nil {
			t.Fatal(err)
		}
		results := race(t, func() error {
			_, err := workflow.ReviewFinalReport(professor.ID, item.ID, model.FinalReportApproved, nil)
			return err
		}, func() error {
			_, err := workflow.CompleteProfessorCase(professor.ID, item.ID, service.ProfessorCompletionInput{Result: model.ProfessorFinalResultGood})
			return err
		})
		if results[0] != nil || (results[1] != nil && !errors.Is(results[1], service.ErrProfessorFinalReportRequired)) {
			t.Fatalf("approval/completion: %v", results)
		}
		if load(t, item.ID).Status != model.FinalReportApproved {
			t.Fatal("completion raced past final approval")
		}
		var persisted model.InternshipCase
		if err := db.First(&persisted, item.ID).Error; err != nil {
			t.Fatal(err)
		}
		expected := model.InternshipCaseStatusActive
		if results[1] == nil {
			expected = model.InternshipCaseStatusPassed
		}
		if persisted.Status != expected {
			t.Fatal("case changed after blocked completion")
		}
	})
}
