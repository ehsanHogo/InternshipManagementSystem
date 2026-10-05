package database

import (
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"internship-management-system/backend/internal/model"
)

func TestLegacyReadyCasesMigration(t *testing.T) {
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
	schema := fmt.Sprintf("m16_migration_%d", time.Now().UnixNano())
	if err := root.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	defer root.Exec("DROP SCHEMA " + schema + " CASCADE")
	scoped := dsn + " search_path=" + schema
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		scoped = parsed.String()
	}
	db, err := gorm.Open(postgres.Open(scoped), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := MigrateAndSeed(db); err != nil {
		t.Fatal(err)
	}
	// Simulate the previous deployed CHECK constraint and stored approved rows.
	if err := db.Exec("ALTER TABLE internship_cases DROP CONSTRAINT chk_internship_case_status").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`ALTER TABLE internship_cases ADD CONSTRAINT chk_internship_case_status CHECK
  (status IN ('DRAFT','PENDING_UNIVERSITY_REVIEW','PENDING_COMPANY_DETAILS','PENDING_FINAL_APPROVAL','READY_TO_START','ACTIVE','PASSED','FAILED','CANCELLED'))`).Error; err != nil {
		t.Fatal(err)
	}
	var student, professor, supervisor, university model.User
	for _, fixture := range []struct {
		email string
		user  *model.User
	}{{"student@demo.local", &student}, {"professor@demo.local", &professor}, {"company@demo.local", &supervisor}, {"university@demo.local", &university}} {
		if err := db.Where("email = ?", fixture.email).First(fixture.user).Error; err != nil {
			t.Fatal(err)
		}
	}
	term := model.InternshipTerm{AcademicYear: 1405, TermType: model.InternshipTermTypeSummer, Status: model.InternshipTermStatusOpen, OpenedAt: time.Now(), CreatedBy: university.ID}
	if err := db.Create(&term).Error; err != nil {
		t.Fatal(err)
	}
	old := time.Date(2025, 5, 6, 7, 8, 9, 0, time.UTC)
	activated := old.Add(-time.Hour)
	text, credits := "preserved", 90
	var rows []model.InternshipCase
	for i, status := range []model.InternshipCaseStatus{"READY_TO_START", "READY_TO_START", model.InternshipCaseStatusActive, model.InternshipCaseStatusPassed, model.InternshipCaseStatusFailed, model.InternshipCaseStatusCancelled, "READY_TO_START"} {
		item := model.InternshipCase{TermID: &term.ID, StudentID: student.ID, ProfessorID: professor.ID, Status: status, PassedCredits: &credits, Mobile: &text, CompanySupervisorID: &supervisor.ID, LetterNumber: &text, LetterDate: &old, InternshipSubject: &text, StartDate: &old, WorkplaceAddress: &text, WorkplacePhone: &text, SubmittedAt: &old, CreatedAt: old.Add(-24 * time.Hour), UpdatedAt: old}
		if i == 1 {
			item.ActivatedAt = &activated
		}
		if err := db.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
		if i == 6 {
			if err := db.Model(&item).UpdateColumns(map[string]any{"updated_at": nil}).Error; err != nil {
				t.Fatal(err)
			}
		}
		if err := db.First(&item, item.ID).Error; err != nil {
			t.Fatal(err)
		}
		rows = append(rows, item)
	}
	if err := MigrateAndSeed(db); err != nil {
		t.Fatal(err)
	}
	for _, before := range rows {
		var after model.InternshipCase
		if err := db.First(&after, before.ID).Error; err != nil {
			t.Fatal(err)
		}
		expected := before
		if before.Status == "READY_TO_START" {
			expected.Status = model.InternshipCaseStatusActive
			if expected.ActivatedAt == nil {
				stamp := before.UpdatedAt
				if stamp.IsZero() {
					stamp = *before.SubmittedAt
				}
				expected.ActivatedAt = &stamp
			}
		}
		if !reflect.DeepEqual(expected, after) {
			t.Fatalf("migration changed unexpected data: %+v -> %+v", expected, after)
		}
	}
	var once, twice []model.InternshipCase
	if err := db.Order("id").Find(&once).Error; err != nil {
		t.Fatal(err)
	}
	if err := MigrateAndSeed(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Order("id").Find(&twice).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(once, twice) {
		t.Fatal("repeated startup changed case data")
	}
	// New status is accepted, and both new legacy inserts and updates are rejected.
	revision := model.InternshipCase{TermID: &term.ID, StudentID: student.ID, ProfessorID: professor.ID, Status: model.InternshipCaseStatusRevisionRequested}
	if err := db.Create(&revision).Error; err != nil {
		t.Fatal(err)
	}
	legacy := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, Status: "READY_TO_START"}
	if err := db.Create(&legacy).Error; err == nil {
		t.Fatal("legacy status still accepted for new cases")
	}
	if err := db.Model(&revision).Update("status", "READY_TO_START").Error; err == nil {
		t.Fatal("legacy status still accepted for case updates")
	}
}
