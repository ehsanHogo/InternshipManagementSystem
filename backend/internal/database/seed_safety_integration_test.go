package database

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"internship-management-system/backend/internal/model"
)

func TestSeedPreservesMutableOperationalData(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	// Isolate bootstrap tests from all existing test/bootstrap records.
	schema := fmt.Sprintf("m19_seed_%d", time.Now().UnixNano())
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(tx.Exec("CREATE SCHEMA " + schema).Error)
	must(tx.Exec("SET LOCAL search_path TO " + schema).Error)
	must(MigrateAndSeed(tx))
	var companies []model.Company
	var users []model.User
	var assignment model.ProfessorAssignment
	must(tx.Order("id").Find(&companies).Error)
	must(tx.Order("id").Find(&users).Error)
	must(tx.First(&assignment).Error)
	if len(companies) != 3 || len(users) != 5 {
		t.Fatal("bootstrap records missing")
	}
	var supervisor, student, professor, admin model.User
	for _, user := range users {
		switch user.Role {
		case model.RoleCompanySupervisor:
			supervisor = user
		case model.RoleStudent:
			student = user
		case model.RoleProfessor:
			professor = user
		case model.RoleAdmin:
			admin = user
		}
	}
	otherProfessor := model.User{Email: "runtime-professor@example.test", FullName: "runtime professor", Role: model.RoleProfessor, PasswordHash: "fixture"}
	must(tx.Create(&otherProfessor).Error)
	must(tx.Model(&assignment).Updates(map[string]any{"professor_id": otherProfessor.ID, "assigned_at": time.Now().Add(-24 * time.Hour)}).Error)
	now := time.Now().UTC()
	for i, company := range companies {
		// All natural seed keys can change; startup must neither reset nor duplicate them.
		must(tx.Model(&company).Updates(map[string]any{"name": fmt.Sprintf("runtime-%d", i), "national_id": fmt.Sprintf("runtime-national-%d", i), "economic_code": fmt.Sprintf("runtime-economic-%d", i), "phone": "runtime phone", "address": "runtime address", "is_approved": false, "registration_status": model.CompanyRegistrationStatusRejected, "registration_reviewed_by": admin.ID, "registration_reviewed_at": now, "registration_rejection_reason": "runtime decision"}).Error)
	}
	must(tx.Model(&supervisor).Updates(map[string]any{"full_name": "runtime name", "phone": "runtime supervisor phone", "job_title": "runtime title", "company_id": companies[1].ID, "password_hash": "runtime password hash"}).Error)
	must(tx.Model(&student).Updates(map[string]any{"phone": "runtime student phone", "job_title": "runtime student title", "company_id": companies[2].ID, "major": "runtime major"}).Error)
	must(tx.Model(&professor).Update("phone", "runtime professor phone").Error)
	var beforeCompanies []model.Company
	var beforeUsers []model.User
	var beforeAssignment model.ProfessorAssignment
	must(tx.Order("id").Find(&beforeCompanies).Error)
	must(tx.Order("id").Find(&beforeUsers).Error)
	must(tx.First(&beforeAssignment, assignment.ID).Error)
	for i := 0; i < 2; i++ {
		must(MigrateAndSeed(tx))
	}
	var afterCompanies []model.Company
	var afterUsers []model.User
	var afterAssignment model.ProfessorAssignment
	must(tx.Order("id").Find(&afterCompanies).Error)
	must(tx.Order("id").Find(&afterUsers).Error)
	must(tx.First(&afterAssignment, assignment.ID).Error)
	if !reflect.DeepEqual(beforeCompanies, afterCompanies) {
		t.Fatal("seed changed reviewed company data or duplicated bootstrap companies")
	}
	if !reflect.DeepEqual(beforeUsers, afterUsers) {
		t.Fatal("seed changed user profile, password, role or membership")
	}
	if !reflect.DeepEqual(beforeAssignment, afterAssignment) {
		t.Fatal("seed reset professor assignment or timestamps")
	}
	// Missing relationships are safely established once with the original demo roles.
	must(tx.Delete(&assignment).Error)
	must(MigrateAndSeed(tx))
	var recreated model.ProfessorAssignment
	must(tx.Where("student_id = ?", student.ID).First(&recreated).Error)
	if recreated.ProfessorID != professor.ID {
		t.Fatal("missing initial assignment not established")
	}
}
