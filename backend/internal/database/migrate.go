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
		&model.User{},
		&model.Company{},
		&model.ProfessorAssignment{},
		&model.File{},
		&model.InternshipCase{},
		&model.InternshipPreference{},
		&model.WeeklyReport{},
		&model.CompanyEvaluation{},
	); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	studentNumber := "40123456"
	major := "مهندسی کامپیوتر"
	users := []model.User{
		{FullName: "علی رضایی", Email: "student@demo.local", Role: model.RoleStudent, StudentNumber: &studentNumber, Major: &major},
		{FullName: "دکتر محمد احمدی", Email: "professor@demo.local", Role: model.RoleProfessor},
		{FullName: "کارشناس آموزش", Email: "university@demo.local", Role: model.RoleUniversitySupervisor},
		{FullName: "رضا محمدی", Email: "company@demo.local", Role: model.RoleCompanySupervisor},
		{FullName: "مدیر سیستم", Email: "admin@demo.local", Role: model.RoleAdmin},
	}

	for i := range users {
		if err := createDemoUserIfMissing(db, &users[i]); err != nil {
			return err
		}
	}
	if err := seedCompanies(db); err != nil {
		return err
	}
	if err := seedProfessorAssignment(db); err != nil {
		return err
	}

	return nil
}

func seedCompanies(db *gorm.DB) error {
	companies := []model.Company{
		{Name: "شرکت داده‌پردازان نوین", IsApproved: true},
		{Name: "شرکت فناوری سپهر", IsApproved: true},
		{Name: "شرکت راهکارهای هوشمند پارس", IsApproved: true},
	}
	for i := range companies {
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "name"}},
			DoUpdates: clause.Assignments(map[string]any{"is_approved": true}),
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
