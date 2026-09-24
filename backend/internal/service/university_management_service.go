package service

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"internship-management-system/backend/internal/auth"
	"internship-management-system/backend/internal/model"
)

var (
	ErrInvalidManagementInput = errors.New("invalid management input")
	ErrEmailAlreadyExists     = errors.New("email already exists")
	ErrStudentNumberExists    = errors.New("student number already exists")
	ErrCompanyAlreadyExists   = errors.New("company already exists")
	ErrManagedUserNotFound    = errors.New("managed user not found")
	ErrCompanyNotFound        = errors.New("approved company not found")
	ErrAssignmentNotFoundMgmt = errors.New("professor assignment not found")
)

type UniversityManagementService struct {
	db *gorm.DB
}

type ManagedUserInput struct {
	FullName      string
	Email         string
	StudentNumber string
	Major         string
	CompanyID     uint
	Phone         string
	JobTitle      string
}

type CompanyInput struct {
	Name         string
	NationalID   string
	EconomicCode string
	Website      string
	Phone        string
	Email        string
	Address      string
}

type ProfessorAssignmentView struct {
	ID        uint
	Student   model.User
	Professor *model.User
}

func NewUniversityManagementService(db *gorm.DB) *UniversityManagementService {
	return &UniversityManagementService{db: db}
}

func (service *UniversityManagementService) ListUsers(role model.Role) ([]model.User, error) {
	var users []model.User
	query := service.db.Where("role = ?", role).Order("full_name ASC")
	if role == model.RoleCompanySupervisor {
		query = query.Preload("Company")
	}
	if err := query.Find(&users).Error; err != nil {
		return nil, fmt.Errorf("list %s users: %w", role, err)
	}
	return users, nil
}

func (service *UniversityManagementService) CreateManagedUser(role model.Role, input ManagedUserInput) (*model.User, string, error) {
	if role != model.RoleStudent && role != model.RoleProfessor && role != model.RoleCompanySupervisor {
		return nil, "", ErrInvalidManagementInput
	}
	fullName := strings.TrimSpace(input.FullName)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if fullName == "" || !validEmail(email) {
		return nil, "", ErrInvalidManagementInput
	}

	user := model.User{FullName: fullName, Email: email, Role: role}
	if role == model.RoleStudent {
		studentNumber := strings.TrimSpace(input.StudentNumber)
		major := strings.TrimSpace(input.Major)
		if studentNumber == "" || major == "" {
			return nil, "", ErrInvalidManagementInput
		}
		user.StudentNumber = &studentNumber
		user.Major = &major
	}
	if role == model.RoleCompanySupervisor {
		if input.CompanyID == 0 {
			return nil, "", ErrInvalidManagementInput
		}
		var company model.Company
		if err := service.db.Where("id = ? AND is_approved = ?", input.CompanyID, true).First(&company).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", ErrCompanyNotFound
		} else if err != nil {
			return nil, "", fmt.Errorf("validate company: %w", err)
		}
		user.CompanyID = &input.CompanyID
		user.Phone = optionalString(input.Phone)
		user.JobTitle = optionalString(input.JobTitle)
	}

	if err := service.ensureUserUnique(email, user.StudentNumber); err != nil {
		return nil, "", err
	}
	temporaryPassword, err := auth.GenerateTemporaryPassword()
	if err != nil {
		return nil, "", fmt.Errorf("generate temporary password: %w", err)
	}
	user.PasswordHash, err = auth.HashPassword(temporaryPassword)
	if err != nil {
		return nil, "", fmt.Errorf("hash temporary password: %w", err)
	}
	if err := service.db.Create(&user).Error; err != nil {
		if duplicateErr := service.ensureUserUnique(email, user.StudentNumber); duplicateErr != nil {
			return nil, "", duplicateErr
		}
		return nil, "", fmt.Errorf("create managed user: %w", err)
	}
	return &user, temporaryPassword, nil
}

func (service *UniversityManagementService) ensureUserUnique(email string, studentNumber *string) error {
	var count int64
	if err := service.db.Model(&model.User{}).Where("email = ?", email).Count(&count).Error; err != nil {
		return fmt.Errorf("check duplicate email: %w", err)
	}
	if count > 0 {
		return ErrEmailAlreadyExists
	}
	if studentNumber != nil {
		if err := service.db.Model(&model.User{}).Where("student_number = ?", *studentNumber).Count(&count).Error; err != nil {
			return fmt.Errorf("check duplicate student number: %w", err)
		}
		if count > 0 {
			return ErrStudentNumberExists
		}
	}
	return nil
}

func (service *UniversityManagementService) ListCompanies() ([]model.Company, error) {
	var companies []model.Company
	if err := service.db.Where("is_approved = ?", true).Order("name ASC").Find(&companies).Error; err != nil {
		return nil, fmt.Errorf("list managed companies: %w", err)
	}
	return companies, nil
}

func (service *UniversityManagementService) CreateCompany(input CompanyInput) (*model.Company, error) {
	name := strings.TrimSpace(input.Name)
	nationalID := strings.TrimSpace(input.NationalID)
	economicCode := strings.TrimSpace(input.EconomicCode)
	if name == "" || nationalID == "" || economicCode == "" {
		return nil, ErrInvalidManagementInput
	}
	var count int64
	if err := service.db.Model(&model.Company{}).
		Where("LOWER(name) = LOWER(?) OR national_id = ? OR economic_code = ?", name, nationalID, economicCode).
		Count(&count).Error; err != nil {
		return nil, fmt.Errorf("check duplicate company: %w", err)
	}
	if count > 0 {
		return nil, ErrCompanyAlreadyExists
	}
	company := model.Company{
		Name: name, NationalID: nationalID, EconomicCode: economicCode,
		Website: optionalString(input.Website), Phone: optionalString(input.Phone),
		Email: optionalString(strings.ToLower(input.Email)), Address: optionalString(input.Address), IsApproved: true,
	}
	if err := service.db.Create(&company).Error; err != nil {
		return nil, fmt.Errorf("create company: %w", err)
	}
	return &company, nil
}

func (service *UniversityManagementService) ListProfessorAssignments() ([]ProfessorAssignmentView, error) {
	students, err := service.ListUsers(model.RoleStudent)
	if err != nil {
		return nil, err
	}
	var assignments []model.ProfessorAssignment
	if err := service.db.Preload("Professor").Find(&assignments).Error; err != nil {
		return nil, fmt.Errorf("list professor assignments: %w", err)
	}
	byStudent := make(map[uint]model.ProfessorAssignment, len(assignments))
	for _, assignment := range assignments {
		byStudent[assignment.StudentID] = assignment
	}
	views := make([]ProfessorAssignmentView, 0, len(students))
	for _, student := range students {
		view := ProfessorAssignmentView{Student: student}
		if assignment, found := byStudent[student.ID]; found {
			view.ID = assignment.ID
			professor := assignment.Professor
			view.Professor = &professor
		}
		views = append(views, view)
	}
	return views, nil
}

func (service *UniversityManagementService) AssignProfessor(studentID, professorID uint) (*model.ProfessorAssignment, error) {
	if err := service.validateAssignmentUsers(studentID, professorID); err != nil {
		return nil, err
	}
	assignment := model.ProfessorAssignment{StudentID: studentID, ProfessorID: professorID, AssignedAt: time.Now()}
	if err := service.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "student_id"}},
		DoUpdates: clause.Assignments(map[string]any{"professor_id": professorID, "assigned_at": assignment.AssignedAt, "updated_at": time.Now()}),
	}).Create(&assignment).Error; err != nil {
		return nil, fmt.Errorf("assign professor: %w", err)
	}
	if err := service.db.Where("student_id = ?", studentID).First(&assignment).Error; err != nil {
		return nil, fmt.Errorf("load professor assignment: %w", err)
	}
	return &assignment, nil
}

func (service *UniversityManagementService) UpdateProfessorAssignment(assignmentID, studentID, professorID uint) (*model.ProfessorAssignment, error) {
	var assignment model.ProfessorAssignment
	if err := service.db.First(&assignment, assignmentID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAssignmentNotFoundMgmt
	} else if err != nil {
		return nil, fmt.Errorf("find professor assignment: %w", err)
	}
	if assignment.StudentID != studentID {
		return nil, ErrInvalidManagementInput
	}
	return service.AssignProfessor(studentID, professorID)
}

func (service *UniversityManagementService) validateAssignmentUsers(studentID, professorID uint) error {
	if studentID == 0 || professorID == 0 {
		return ErrInvalidManagementInput
	}
	var count int64
	if err := service.db.Model(&model.User{}).Where("id = ? AND role = ?", studentID, model.RoleStudent).Count(&count).Error; err != nil {
		return fmt.Errorf("validate student: %w", err)
	}
	if count == 0 {
		return ErrManagedUserNotFound
	}
	if err := service.db.Model(&model.User{}).Where("id = ? AND role = ?", professorID, model.RoleProfessor).Count(&count).Error; err != nil {
		return fmt.Errorf("validate professor: %w", err)
	}
	if count == 0 {
		return ErrManagedUserNotFound
	}
	return nil
}

func validEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value
}

func optionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
