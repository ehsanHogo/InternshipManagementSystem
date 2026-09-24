package service

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"internship-management-system/backend/internal/auth"
	"internship-management-system/backend/internal/model"
)

var (
	ErrInvalidCompanyRegistration = errors.New("invalid company registration")
	ErrNationalIDAlreadyExists    = errors.New("company national id already exists")
	ErrEconomicCodeAlreadyExists  = errors.New("company economic code already exists")
	ErrCompanyNameAlreadyExists   = errors.New("company name already exists")
	ErrCompanyProfileNotFound     = errors.New("company profile not found")
)

type CompanyAccountService struct {
	db *gorm.DB
}

type CompanySupervisorRegistrationInput struct {
	FullName string
	Email    string
	Password string
	Phone    string
	JobTitle string
}

type CompanyRegistrationInput struct {
	Supervisor CompanySupervisorRegistrationInput
	Company    CompanyInput
}

type CompanyAccount struct {
	Company    model.Company
	Supervisor model.User
}

func NewCompanyAccountService(db *gorm.DB) *CompanyAccountService {
	return &CompanyAccountService{db: db}
}

func (service *CompanyAccountService) Register(input CompanyRegistrationInput) (*CompanyAccount, error) {
	normalized, err := normalizeCompanyRegistration(input)
	if err != nil {
		return nil, err
	}

	passwordHash, err := auth.HashPassword(normalized.Supervisor.Password)
	if err != nil {
		return nil, fmt.Errorf("hash company supervisor password: %w", err)
	}

	account := &CompanyAccount{}
	err = service.db.Transaction(func(tx *gorm.DB) error {
		if err := ensureCompanyRegistrationUnique(tx, normalized); err != nil {
			return err
		}

		company := model.Company{
			Name:         normalized.Company.Name,
			NationalID:   normalized.Company.NationalID,
			EconomicCode: normalized.Company.EconomicCode,
			Website:      optionalString(normalized.Company.Website),
			Phone:        optionalString(normalized.Company.Phone),
			Email:        optionalString(normalized.Company.Email),
			Address:      optionalString(normalized.Company.Address),
			IsApproved:   false,
		}
		if err := tx.Create(&company).Error; err != nil {
			return classifyRegistrationConstraint(err)
		}

		phone := normalized.Supervisor.Phone
		jobTitle := normalized.Supervisor.JobTitle
		supervisor := model.User{
			FullName:     normalized.Supervisor.FullName,
			Email:        normalized.Supervisor.Email,
			PasswordHash: passwordHash,
			Role:         model.RoleCompanySupervisor,
			Phone:        &phone,
			JobTitle:     &jobTitle,
			CompanyID:    &company.ID,
		}
		if err := tx.Create(&supervisor).Error; err != nil {
			return classifyRegistrationConstraint(err)
		}

		account.Company = company
		account.Supervisor = supervisor
		return nil
	})
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (service *CompanyAccountService) GetProfile(userID uint) (*CompanyAccount, error) {
	var supervisor model.User
	result := service.db.Where("id = ? AND role = ?", userID, model.RoleCompanySupervisor).First(&supervisor)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, ErrCompanyProfileNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("find company supervisor profile: %w", result.Error)
	}
	if supervisor.CompanyID == nil {
		return nil, ErrCompanyProfileNotFound
	}

	var company model.Company
	if err := service.db.First(&company, *supervisor.CompanyID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCompanyProfileNotFound
	} else if err != nil {
		return nil, fmt.Errorf("find supervisor company profile: %w", err)
	}

	return &CompanyAccount{Company: company, Supervisor: supervisor}, nil
}

func normalizeCompanyRegistration(input CompanyRegistrationInput) (CompanyRegistrationInput, error) {
	input.Supervisor.FullName = strings.TrimSpace(input.Supervisor.FullName)
	input.Supervisor.Email = strings.ToLower(strings.TrimSpace(input.Supervisor.Email))
	input.Supervisor.Phone = strings.TrimSpace(input.Supervisor.Phone)
	input.Supervisor.JobTitle = strings.TrimSpace(input.Supervisor.JobTitle)
	input.Company.Name = strings.TrimSpace(input.Company.Name)
	input.Company.NationalID = strings.TrimSpace(input.Company.NationalID)
	input.Company.EconomicCode = strings.TrimSpace(input.Company.EconomicCode)
	input.Company.Website = strings.TrimSpace(input.Company.Website)
	input.Company.Phone = strings.TrimSpace(input.Company.Phone)
	input.Company.Email = strings.ToLower(strings.TrimSpace(input.Company.Email))
	input.Company.Address = strings.TrimSpace(input.Company.Address)

	if input.Supervisor.FullName == "" || !validEmail(input.Supervisor.Email) ||
		strings.TrimSpace(input.Supervisor.Password) == "" || len([]byte(input.Supervisor.Password)) > 72 ||
		input.Supervisor.Phone == "" || input.Supervisor.JobTitle == "" ||
		input.Company.Name == "" || input.Company.NationalID == "" || input.Company.EconomicCode == "" ||
		input.Company.Phone == "" || !validEmail(input.Company.Email) || input.Company.Address == "" {
		return CompanyRegistrationInput{}, ErrInvalidCompanyRegistration
	}
	return input, nil
}

func ensureCompanyRegistrationUnique(db *gorm.DB, input CompanyRegistrationInput) error {
	checks := []struct {
		model any
		query string
		value any
		err   error
	}{
		{model: &model.User{}, query: "email = ?", value: input.Supervisor.Email, err: ErrEmailAlreadyExists},
		{model: &model.Company{}, query: "LOWER(name) = LOWER(?)", value: input.Company.Name, err: ErrCompanyNameAlreadyExists},
		{model: &model.Company{}, query: "national_id = ?", value: input.Company.NationalID, err: ErrNationalIDAlreadyExists},
		{model: &model.Company{}, query: "economic_code = ?", value: input.Company.EconomicCode, err: ErrEconomicCodeAlreadyExists},
	}
	for _, check := range checks {
		var count int64
		if err := db.Model(check.model).Where(check.query, check.value).Count(&count).Error; err != nil {
			return fmt.Errorf("check company registration uniqueness: %w", err)
		}
		if count > 0 {
			return check.err
		}
	}
	return nil
}

func classifyRegistrationConstraint(err error) error {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "idx_users_email") || strings.Contains(message, "users_email_key"):
		return ErrEmailAlreadyExists
	case strings.Contains(message, "idx_companies_national_id") || strings.Contains(message, "companies_national_id_key"):
		return ErrNationalIDAlreadyExists
	case strings.Contains(message, "idx_companies_economic_code") || strings.Contains(message, "companies_economic_code_key"):
		return ErrEconomicCodeAlreadyExists
	case strings.Contains(message, "idx_companies_name") || strings.Contains(message, "companies_name_key"):
		return ErrCompanyNameAlreadyExists
	default:
		return fmt.Errorf("create company registration account: %w", err)
	}
}
