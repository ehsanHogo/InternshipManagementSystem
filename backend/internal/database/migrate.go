package database

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"internship-management-system/backend/internal/auth"
	"internship-management-system/backend/internal/model"
)

const demoPassword = "Demo123!"

func MigrateAndSeed(db *gorm.DB) error {
	if err := db.AutoMigrate(&model.User{}); err != nil {
		return fmt.Errorf("migrate users: %w", err)
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
