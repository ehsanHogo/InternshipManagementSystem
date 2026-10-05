package database

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"internship-management-system/backend/internal/auth"
	"internship-management-system/backend/internal/model"
)

const demoPassword = "Demo123!"

func MigrateAndSeed(db *gorm.DB) error {
	if err := migrateCompanyRegistrations(db); err != nil {
		return err
	}
	if err := migrateInternshipCaseStatuses(db); err != nil {
		return err
	}
	if err := db.AutoMigrate(
		&model.Company{},
		&model.User{},
		&model.InternshipOpportunity{},
		&model.ProfessorAssignment{},
		&model.File{},
		&model.OpportunityApplication{},
		&model.InternshipTerm{},
		&model.InternshipCase{},
		&model.InternshipPreference{},
		&model.FinalReport{},
		&model.WeeklyReport{},
		&model.CompanyEvaluation{},
	); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	var demoCompany model.Company
	var existingSupervisor model.User
	err := db.Where("email = ?", "company@demo.local").First(&existingSupervisor).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Demo companies are bootstrap data. Once the demo account exists,
		// names, legal identifiers and membership may all have changed at runtime.
		if err := seedCompanies(db); err != nil {
			return err
		}
		if err := db.Where("national_id = ?", "14000000001").First(&demoCompany).Error; err != nil {
			return fmt.Errorf("find demo company for supervisor: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("find demo supervisor: %w", err)
	}

	studentNumber := "40123456"
	major := "مهندسی کامپیوتر"
	phone := "02188776655"
	jobTitle := "سرپرست کارآموزی"
	users := []model.User{
		{FullName: "علی رضایی", Email: "student@demo.local", Role: model.RoleStudent, StudentNumber: &studentNumber, Major: &major},
		{FullName: "دکتر محمد احمدی", Email: "professor@demo.local", Role: model.RoleProfessor},
		{FullName: "کارشناس آموزش", Email: "university@demo.local", Role: model.RoleUniversitySupervisor},
		{FullName: "رضا محمدی", Email: "company@demo.local", Role: model.RoleCompanySupervisor, Phone: &phone, JobTitle: &jobTitle, CompanyID: &demoCompany.ID},
		{FullName: "مدیر سیستم", Email: "admin@demo.local", Role: model.RoleAdmin},
	}

	for i := range users {
		if err := createDemoUserIfMissing(db, &users[i]); err != nil {
			return err
		}
	}
	if err := seedProfessorAssignment(db); err != nil {
		return err
	}

	return nil
}

func seedCompanies(db *gorm.DB) error {
	companies := []model.Company{
		{Name: "شرکت داده‌پردازان نوین", NationalID: "14000000001", EconomicCode: "411111111111", IsApproved: true, RegistrationStatus: model.CompanyRegistrationStatusApproved},
		{Name: "شرکت فناوری سپهر", NationalID: "14000000002", EconomicCode: "422222222222", IsApproved: true, RegistrationStatus: model.CompanyRegistrationStatusApproved},
		{Name: "شرکت راهکارهای هوشمند پارس", NationalID: "14000000003", EconomicCode: "433333333333", IsApproved: true, RegistrationStatus: model.CompanyRegistrationStatusApproved},
	}
	for i := range companies {
		if err := db.Clauses(clause.OnConflict{
			DoNothing: true,
		}).Create(&companies[i]).Error; err != nil {
			return fmt.Errorf("seed company %s: %w", companies[i].Name, err)
		}
	}
	return nil
}

func seedProfessorAssignment(db *gorm.DB) error {
	var student, professor model.User
	if err := db.Where("email = ?", "student@demo.local").First(&student).Error; err != nil {
		return fmt.Errorf("find demo student for assignment: %w", err)
	}
	if err := db.Where("email = ?", "professor@demo.local").First(&professor).Error; err != nil {
		return fmt.Errorf("find demo professor for assignment: %w", err)
	}
	if student.Role != model.RoleStudent || professor.Role != model.RoleProfessor {
		return nil
	}

	assignment := model.ProfessorAssignment{
		StudentID: student.ID, ProfessorID: professor.ID, AssignedAt: time.Now(),
	}
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "student_id"}},
		DoNothing: true,
	}).Create(&assignment).Error; err != nil {
		return fmt.Errorf("seed professor assignment: %w", err)
	}
	return nil
}

func createDemoUserIfMissing(db *gorm.DB, user *model.User) error {
	var existing model.User
	result := db.Where("email = ?", strings.ToLower(user.Email)).First(&existing)
	if result.Error == nil {
		return nil
	}
	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("look up demo user %s: %w", user.Email, result.Error)
	}

	hash, err := auth.HashPassword(demoPassword)
	if err != nil {
		return fmt.Errorf("hash demo password: %w", err)
	}
	user.Email = strings.ToLower(user.Email)
	user.PasswordHash = hash
	if err := db.Create(user).Error; err != nil {
		return fmt.Errorf("create demo user %s: %w", user.Email, err)
	}
	return nil
}

// GORM does not replace an existing named CHECK when its expression changes.
// Convert approved legacy rows before atomically replacing that constraint.
func migrateInternshipCaseStatuses(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.InternshipCase{}) {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("LOCK TABLE internship_cases IN ACCESS EXCLUSIVE MODE").Error; err != nil {
			return fmt.Errorf("lock internship status migration: %w", err)
		}
		// updated_at is the best existing audit timestamp for legacy ready cases.
		// Preserve activated_at when present and never rewrite unrelated timestamps.
		if err := tx.Exec(`UPDATE internship_cases SET status = 'ACTIVE',
   activated_at = COALESCE(activated_at, updated_at, submitted_at, created_at, CURRENT_TIMESTAMP)
   WHERE status = 'READY_TO_START'`).Error; err != nil {
			return fmt.Errorf("migrate approved internship cases: %w", err)
		}
		if err := tx.Exec("ALTER TABLE internship_cases DROP CONSTRAINT IF EXISTS chk_internship_case_status").Error; err != nil {
			return fmt.Errorf("drop legacy internship status constraint: %w", err)
		}
		if err := tx.Migrator().CreateConstraint(&model.InternshipCase{}, "chk_internship_case_status"); err != nil {
			return fmt.Errorf("create internship status constraint: %w", err)
		}
		return nil
	})
}

// Only the first addition backfills legacy companies. Subsequent startups never
// rewrite decisions, including PENDING and REJECTED seed-company records.
func migrateCompanyRegistrations(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.Company{}) {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("LOCK TABLE companies IN ACCESS EXCLUSIVE MODE").Error; err != nil {
			return err
		}
		if !tx.Migrator().HasColumn(&model.Company{}, "registration_status") {
			if err := tx.Exec("ALTER TABLE companies ADD COLUMN registration_status varchar(16) NOT NULL DEFAULT 'APPROVED'").Error; err != nil {
				return fmt.Errorf("backfill company registrations: %w", err)
			}
		}
		return tx.Exec("ALTER TABLE companies ALTER COLUMN registration_status SET DEFAULT 'PENDING'").Error
	})
}
