package service_test

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"internship-management-system/backend/internal/auth"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

func TestUniversityManagement(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect to integration database: %v", err)
	}
	if err := db.AutoMigrate(&model.Company{}, &model.User{}, &model.ProfessorAssignment{}, &model.InternshipCase{}); err != nil {
		t.Fatalf("migrate integration database: %v", err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin test transaction: %v", tx.Error)
	}
	defer tx.Rollback()

	management := service.NewUniversityManagementService(tx)
	workflow := service.NewInternshipService(tx)
	suffix := time.Now().UnixNano()
	studentNumber := fmt.Sprintf("m7-%d", suffix)
	studentEmail := fmt.Sprintf("m7-student-%d@example.test", suffix)
	student, studentPassword, err := management.CreateManagedUser(model.RoleStudent, service.ManagedUserInput{
		FullName: "دانشجوی آزمون", Email: studentEmail, StudentNumber: studentNumber, Major: "مهندسی کامپیوتر",
	})
	if err != nil {
		t.Fatalf("create student: %v", err)
	}
	if student.PasswordHash == studentPassword || !auth.CheckPassword(student.PasswordHash, studentPassword) {
		t.Fatal("temporary student credential does not match its bcrypt hash")
	}
	if _, _, err := management.CreateManagedUser(model.RoleStudent, service.ManagedUserInput{
		FullName: "تکراری", Email: studentEmail, StudentNumber: studentNumber + "-other", Major: "کامپیوتر",
	}); !errors.Is(err, service.ErrEmailAlreadyExists) {
		t.Fatalf("duplicate email error = %v", err)
	}
	if _, _, err := management.CreateManagedUser(model.RoleStudent, service.ManagedUserInput{
		FullName: "تکراری", Email: fmt.Sprintf("m7-other-%d@example.test", suffix), StudentNumber: studentNumber, Major: "کامپیوتر",
	}); !errors.Is(err, service.ErrStudentNumberExists) {
		t.Fatalf("duplicate student number error = %v", err)
	}

	professorOne, professorPassword, err := management.CreateManagedUser(model.RoleProfessor, service.ManagedUserInput{
		FullName: "استاد اول", Email: fmt.Sprintf("m7-professor-1-%d@example.test", suffix),
	})
	if err != nil || !auth.CheckPassword(professorOne.PasswordHash, professorPassword) {
		t.Fatalf("create professor or verify temporary password: user=%+v err=%v", professorOne, err)
	}
	professorTwo, _, err := management.CreateManagedUser(model.RoleProfessor, service.ManagedUserInput{
		FullName: "استاد دوم", Email: fmt.Sprintf("m7-professor-2-%d@example.test", suffix),
	})
	if err != nil {
		t.Fatalf("create second professor: %v", err)
	}
	if _, err := management.AssignProfessor(student.ID, professorOne.ID); err != nil {
		t.Fatalf("assign first professor: %v", err)
	}
	existingCase, created, err := workflow.CreateOrGetCase(student.ID)
	if err != nil || !created || existingCase.ProfessorID != professorOne.ID {
		t.Fatalf("create case with first professor: case=%+v created=%v err=%v", existingCase, created, err)
	}
	if _, err := management.AssignProfessor(student.ID, professorTwo.ID); err != nil {
		t.Fatalf("reassign professor: %v", err)
	}
	var unchangedCase model.InternshipCase
	if err := tx.First(&unchangedCase, existingCase.ID).Error; err != nil || unchangedCase.ProfessorID != professorOne.ID {
		t.Fatalf("existing case professor changed after reassignment: case=%+v err=%v", unchangedCase, err)
	}

	newStudentNumber := fmt.Sprintf("m7-new-%d", suffix)
	newStudent, _, err := management.CreateManagedUser(model.RoleStudent, service.ManagedUserInput{
		FullName: "دانشجوی دوم", Email: fmt.Sprintf("m7-new-student-%d@example.test", suffix), StudentNumber: newStudentNumber, Major: "مهندسی برق",
	})
	if err != nil {
		t.Fatalf("create second student: %v", err)
	}
	if _, err := management.AssignProfessor(newStudent.ID, professorTwo.ID); err != nil {
		t.Fatalf("assign current professor to second student: %v", err)
	}
	newCase, _, err := workflow.CreateOrGetCase(newStudent.ID)
	if err != nil || newCase.ProfessorID != professorTwo.ID {
		t.Fatalf("new case did not use current professor: case=%+v err=%v", newCase, err)
	}

	company, err := management.CreateCompany(service.CompanyInput{Name: fmt.Sprintf("شرکت آزمون %d", suffix), NationalID: fmt.Sprintf("national-%d", suffix), EconomicCode: fmt.Sprintf("economic-%d", suffix), Email: "company@example.test"})
	if err != nil || !company.IsApproved {
		t.Fatalf("create approved company: company=%+v err=%v", company, err)
	}
	supervisor, supervisorPassword, err := management.CreateManagedUser(model.RoleCompanySupervisor, service.ManagedUserInput{
		FullName: "سرپرست آزمون", Email: fmt.Sprintf("m7-supervisor-%d@example.test", suffix), CompanyID: company.ID,
	})
	if err != nil || supervisor.CompanyID == nil || *supervisor.CompanyID != company.ID || !auth.CheckPassword(supervisor.PasswordHash, supervisorPassword) {
		t.Fatalf("create linked company supervisor: supervisor=%+v err=%v", supervisor, err)
	}
}
