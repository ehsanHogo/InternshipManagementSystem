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
	if err := db.AutoMigrate(
		&model.Company{},
		&model.User{},
		&model.InternshipOpportunity{},
		&model.ProfessorAssignment{},
		&model.File{},
		&model.OpportunityApplication{},
		&model.InternshipCase{},
		&model.InternshipPreference{},
		&model.WeeklyReport{},
		&model.CompanyEvaluation{},
	); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	if err := seedCompanies(db); err != nil {
		return err
	}
	var demoCompany model.Company
	if err := db.Where("national_id = ?", "14000000001").First(&demoCompany).Error; err != nil {
		return fmt.Errorf("find demo company for supervisor: %w", err)
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
		{Name: "شرکت داده‌پردازان نوین", NationalID: "14000000001", EconomicCode: "411111111111", IsApproved: true},
		{Name: "شرکت فناوری سپهر", NationalID: "14000000002", EconomicCode: "422222222222", IsApproved: true},
		{Name: "شرکت راهکارهای هوشمند پارس", NationalID: "14000000003", EconomicCode: "433333333333", IsApproved: true},
	}
	for i := range companies {
		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "name"}},
			DoUpdates: clause.Assignments(map[string]any{
				"national_id": companies[i].NationalID, "economic_code": companies[i].EconomicCode, "is_approved": true,
			}),
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

	assignment := model.ProfessorAssignment{
		StudentID: student.ID, ProfessorID: professor.ID, AssignedAt: time.Now(),
	}
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "student_id"}},
		DoUpdates: clause.Assignments(map[string]any{"professor_id": professor.ID}),
	}).Create(&assignment).Error; err != nil {
		return fmt.Errorf("seed professor assignment: %w", err)
	}
	return nil
}

func createDemoUserIfMissing(db *gorm.DB, user *model.User) error {
	var existing model.User
	result := db.Where("email = ?", strings.ToLower(user.Email)).First(&existing)
	if result.Error == nil {
		return db.Model(&existing).Updates(map[string]any{
			"phone": user.Phone, "job_title": user.JobTitle, "company_id": user.CompanyID,
		}).Error
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
