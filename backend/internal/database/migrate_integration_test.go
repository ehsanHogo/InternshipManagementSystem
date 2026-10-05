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

func TestLegacyCompanyRegistrationBackfillAndSeedDecisions(t *testing.T) {
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
	schema := fmt.Sprintf("m17_migration_%d", time.Now().UnixNano())
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
	// A pre-M17 table has no registration columns. Both existing trust values must survive.
	if err := db.Exec(`CREATE TABLE companies (
 id bigserial PRIMARY KEY, name varchar(250) NOT NULL,
 national_id varchar(50) NOT NULL, economic_code varchar(50) NOT NULL,
 website varchar(500), phone varchar(50), email varchar(320), address varchar(1000),
 is_approved boolean NOT NULL DEFAULT false, created_at timestamptz, updated_at timestamptz)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO companies (name,national_id,economic_code,is_approved,created_at,updated_at)
 VALUES ('legacy trusted','legacy-a','economic-a',true,NOW(),NOW()),
 ('legacy new','legacy-b','economic-b',false,NOW(),NOW())`).Error; err != nil {
		t.Fatal(err)
	}
	if err := MigrateAndSeed(db); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		name    string
		trusted bool
	}{{"legacy trusted", true}, {"legacy new", false}} {
		var company model.Company
		if err := db.Where("name = ?", fixture.name).First(&company).Error; err != nil {
			t.Fatal(err)
		}
		if company.RegistrationStatus != model.CompanyRegistrationStatusApproved || company.IsApproved != fixture.trusted {
			t.Fatalf("legacy backfill conflated approval concepts: %+v", company)
		}
	}
	var seeds []model.Company
	if err := db.Where("national_id IN ?", []string{"14000000001", "14000000002", "14000000003"}).Order("id").Find(&seeds).Error; err != nil {
		t.Fatal(err)
	}
	if len(seeds) != 3 {
		t.Fatal("demo companies missing")
	}
	for _, company := range seeds {
		if company.RegistrationStatus != model.CompanyRegistrationStatusApproved {
			t.Fatal("new seed company not operational")
		}
	}
	// Simulate real Admin decisions on known seed records, including university trust false.
	now := time.Now().UTC()
	reason := "مدارک ناقص"
	var admin model.User
	if err := db.Where("email = ?", "admin@demo.local").First(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&seeds[0]).Updates(map[string]any{"registration_status": model.CompanyRegistrationStatusPending, "is_approved": false}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&seeds[1]).Updates(map[string]any{"registration_status": model.CompanyRegistrationStatusRejected, "registration_reviewed_by": admin.ID, "registration_reviewed_at": now, "registration_rejection_reason": reason}).Error; err != nil {
		t.Fatal(err)
	}
	var once []model.Company
	if err := db.Order("id").Find(&once).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := MigrateAndSeed(db); err != nil {
			t.Fatal(err)
		}
		var after []model.Company
		if err := db.Order("id").Find(&after).Error; err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(once, after) {
			t.Fatal("repeated migration/seed rewrote company registration or trust data")
		}
	}
	// The database default after the one-time legacy backfill is PENDING.
	company := model.Company{Name: "new after migration", NationalID: "new-national", EconomicCode: "new-economic"}
	if err := db.Create(&company).Error; err != nil {
		t.Fatal(err)
	}
	if company.RegistrationStatus != model.CompanyRegistrationStatusPending || company.IsApproved {
		t.Fatal("new company inherited legacy registration default")
	}
	// Invalid statuses are rejected by the database, independently of API validation.
	if err := db.Model(&company).Update("registration_status", "UNKNOWN").Error; err == nil {
		t.Fatal("invalid registration status accepted")
	}
}
