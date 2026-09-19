package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"internship-management-system/backend/internal/model"
)

type SendToCompanyInput struct {
	PreferenceID        uint
	CompanySupervisorID uint
	LetterNumber        string
	LetterDate          time.Time
}

type CompanyConfirmationInput struct {
	InternshipSubject string
	StartDate         time.Time
	WorkplaceAddress  string
	WorkplacePhone    string
}

func (service *InternshipService) ListCompanySupervisors() ([]model.User, error) {
	var users []model.User
	if err := service.db.Where("role = ?", model.RoleCompanySupervisor).Order("full_name ASC").Find(&users).Error; err != nil {
		return nil, fmt.Errorf("list company supervisors: %w", err)
	}
	return users, nil
}

func (service *InternshipService) ListUniversityCases(status *model.InternshipCaseStatus) ([]model.InternshipCase, error) {
	statuses := universityCaseStatuses()
	query := service.caseQuery(service.db).Where("status IN ?", statuses)
	if status != nil {
		if !containsStatus(statuses, *status) {
			return nil, ErrInvalidCaseStatus
		}
		query = query.Where("status = ?", *status)
	}

	var cases []model.InternshipCase
	if err := query.Order("submitted_at DESC NULLS LAST, created_at DESC").Find(&cases).Error; err != nil {
		return nil, fmt.Errorf("list university internship cases: %w", err)
	}
	return cases, nil
}

func (service *InternshipService) GetUniversityCase(caseID uint) (*model.InternshipCase, error) {
	var internshipCase model.InternshipCase
	err := service.caseQuery(service.db).Where("status IN ?", universityCaseStatuses()).First(&internshipCase, caseID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCaseNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get university internship case: %w", err)
	}
	return &internshipCase, nil
}

func (service *InternshipService) SendToCompany(caseID uint, input SendToCompanyInput) (*model.InternshipCase, error) {
	letterNumber := strings.TrimSpace(input.LetterNumber)
	if input.PreferenceID == 0 || input.CompanySupervisorID == 0 || letterNumber == "" || input.LetterDate.IsZero() {
		return nil, ErrInvalidApplication
	}

	err := service.db.Transaction(func(tx *gorm.DB) error {
		internshipCase, err := lockCase(tx, caseID)
		if err != nil {
			return err
		}
		if internshipCase.Status != model.InternshipCaseStatusPendingUniversityApproval {
			return ErrInvalidTransition
		}

		var preference model.InternshipPreference
		if err := tx.Where("id = ? AND internship_case_id = ?", input.PreferenceID, caseID).First(&preference).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPreferenceNotFound
		} else if err != nil {
			return fmt.Errorf("validate selected preference: %w", err)
		}

		var supervisor model.User
		if err := tx.Where("id = ? AND role = ?", input.CompanySupervisorID, model.RoleCompanySupervisor).First(&supervisor).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCompanySupervisor
		} else if err != nil {
			return fmt.Errorf("validate company supervisor: %w", err)
		}

		if err := tx.Model(internshipCase).Updates(map[string]any{
			"selected_preference_id": input.PreferenceID,
			"company_supervisor_id":  input.CompanySupervisorID,
			"letter_number":          letterNumber,
			"letter_date":            input.LetterDate,
			"status":                 model.InternshipCaseStatusPendingCompanyApproval,
		}).Error; err != nil {
			return fmt.Errorf("send internship case to company: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return service.getCaseByID(caseID)
}

func (service *InternshipService) ListCompanyCases(supervisorID uint) ([]model.InternshipCase, error) {
	var cases []model.InternshipCase
	if err := service.caseQuery(service.db).
		Where("company_supervisor_id = ? AND status IN ?", supervisorID, companyVisibleStatuses()).
		Order("updated_at DESC").Find(&cases).Error; err != nil {
		return nil, fmt.Errorf("list company internship cases: %w", err)
	}
	return cases, nil
}

func (service *InternshipService) GetCompanyCase(supervisorID, caseID uint) (*model.InternshipCase, error) {
	var internshipCase model.InternshipCase
	err := service.caseQuery(service.db).
		Where("company_supervisor_id = ? AND status IN ?", supervisorID, companyVisibleStatuses()).
		First(&internshipCase, caseID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCaseAccessDenied
	}
	if err != nil {
		return nil, fmt.Errorf("get company internship case: %w", err)
	}
	return &internshipCase, nil
}

func (service *InternshipService) ConfirmCompanyCase(supervisorID, caseID uint, input CompanyConfirmationInput) (*model.InternshipCase, error) {
	subject := strings.TrimSpace(input.InternshipSubject)
	address := strings.TrimSpace(input.WorkplaceAddress)
	phone := strings.TrimSpace(input.WorkplacePhone)
	if subject == "" || input.StartDate.IsZero() || address == "" || phone == "" {
		return nil, ErrInvalidApplication
	}

	err := service.db.Transaction(func(tx *gorm.DB) error {
		internshipCase, err := lockCase(tx, caseID)
		if err != nil {
			return err
		}
		if internshipCase.CompanySupervisorID == nil || *internshipCase.CompanySupervisorID != supervisorID {
			return ErrCaseAccessDenied
		}
		if internshipCase.Status != model.InternshipCaseStatusPendingCompanyApproval {
			return ErrInvalidTransition
		}
		if internshipCase.SelectedPreferenceID == nil {
			return ErrInvalidApplication
		}

		var preference model.InternshipPreference
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND internship_case_id = ?", *internshipCase.SelectedPreferenceID, caseID).
			First(&preference).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPreferenceNotFound
		} else if err != nil {
			return fmt.Errorf("lock selected preference: %w", err)
		}

		var companyID uint
		if preference.CompanyID != nil {
			var company model.Company
			if err := tx.Where("id = ? AND is_approved = ?", *preference.CompanyID, true).First(&company).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvalidPreference
			} else if err != nil {
				return fmt.Errorf("validate selected company: %w", err)
			}
			companyID = company.ID
		} else {
			createdCompanyID, err := service.approveProposedCompany(tx, &preference)
			if err != nil {
				return err
			}
			companyID = createdCompanyID
			if err := tx.Model(&model.User{}).
				Where("id = ? AND role = ?", supervisorID, model.RoleCompanySupervisor).
				Update("company_id", companyID).Error; err != nil {
				return fmt.Errorf("associate company supervisor with company: %w", err)
			}
		}

		now := time.Now()
		if err := tx.Model(internshipCase).Updates(map[string]any{
			"internship_subject":   subject,
			"start_date":           input.StartDate,
			"workplace_address":    address,
			"workplace_phone":      phone,
			"company_confirmed_at": now,
			"status":               model.InternshipCaseStatusCompanyApproved,
		}).Error; err != nil {
			return fmt.Errorf("confirm company placement: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return service.getCaseByID(caseID)
}

func (service *InternshipService) ApproveUniversityCase(caseID uint) (*model.InternshipCase, error) {
	err := service.db.Transaction(func(tx *gorm.DB) error {
		internshipCase, err := lockCase(tx, caseID)
		if err != nil {
			return err
		}
		if internshipCase.Status != model.InternshipCaseStatusCompanyApproved {
			return ErrInvalidTransition
		}
		if internshipCase.SelectedPreferenceID == nil || internshipCase.CompanySupervisorID == nil ||
			internshipCase.CompanyConfirmedAt == nil || internshipCase.InternshipSubject == nil ||
			internshipCase.StartDate == nil || internshipCase.WorkplaceAddress == nil ||
			internshipCase.WorkplacePhone == nil || internshipCase.LetterNumber == nil || internshipCase.LetterDate == nil {
			return ErrInvalidApplication
		}

		var count int64
		if err := tx.Model(&model.InternshipPreference{}).
			Joins("JOIN companies ON companies.id = internship_preferences.company_id").
			Where("internship_preferences.id = ? AND internship_preferences.internship_case_id = ? AND companies.is_approved = ?",
				*internshipCase.SelectedPreferenceID, caseID, true).
			Count(&count).Error; err != nil {
			return fmt.Errorf("validate approved selected company: %w", err)
		}
		if count == 0 {
			return ErrInvalidApplication
		}

		if err := tx.Model(internshipCase).Updates(map[string]any{
			"university_approved_at": time.Now(),
			"status":                 model.InternshipCaseStatusUniversityApproved,
		}).Error; err != nil {
			return fmt.Errorf("approve internship case: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return service.getCaseByID(caseID)
}

func (service *InternshipService) ActivateUniversityCase(caseID uint) (*model.InternshipCase, error) {
	err := service.db.Transaction(func(tx *gorm.DB) error {
		internshipCase, err := lockCase(tx, caseID)
		if err != nil {
			return err
		}
		if internshipCase.Status != model.InternshipCaseStatusUniversityApproved {
			return ErrInvalidTransition
		}
		if err := tx.Model(internshipCase).Updates(map[string]any{
			"activated_at": time.Now(),
			"status":       model.InternshipCaseStatusActive,
		}).Error; err != nil {
			return fmt.Errorf("activate internship case: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return service.getCaseByID(caseID)
}

func (service *InternshipService) approveProposedCompany(tx *gorm.DB, preference *model.InternshipPreference) (uint, error) {
	if preference.ProposedCompanyName == nil || strings.TrimSpace(*preference.ProposedCompanyName) == "" {
		return 0, ErrInvalidPreference
	}
	name := strings.TrimSpace(*preference.ProposedCompanyName)
	address := trimmedPointer(&preference.City)
	company := model.Company{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("name = ?", name).First(&company).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		company = model.Company{
			Name: name, Website: preference.ProposedWebsite, Phone: preference.ProposedPhone,
			Email: preference.ProposedEmail, Address: address, IsApproved: true,
		}
		if err := tx.Create(&company).Error; err != nil {
			return 0, fmt.Errorf("create approved proposed company: %w", err)
		}
	} else if err != nil {
		return 0, fmt.Errorf("look up proposed company: %w", err)
	} else if !company.IsApproved {
		if err := tx.Model(&company).Updates(map[string]any{"is_approved": true}).Error; err != nil {
			return 0, fmt.Errorf("approve existing proposed company: %w", err)
		}
	}
	if err := tx.Model(preference).Update("company_id", company.ID).Error; err != nil {
		return 0, fmt.Errorf("link preference to approved company: %w", err)
	}
	return company.ID, nil
}

func lockCase(tx *gorm.DB, caseID uint) (*model.InternshipCase, error) {
	var internshipCase model.InternshipCase
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&internshipCase, caseID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCaseNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock internship case: %w", err)
	}
	return &internshipCase, nil
}

func universityCaseStatuses() []model.InternshipCaseStatus {
	return []model.InternshipCaseStatus{
		model.InternshipCaseStatusPendingUniversityApproval,
		model.InternshipCaseStatusPendingCompanyApproval,
		model.InternshipCaseStatusCompanyApproved,
		model.InternshipCaseStatusUniversityApproved,
		model.InternshipCaseStatusActive,
	}
}

func companyVisibleStatuses() []model.InternshipCaseStatus {
	return []model.InternshipCaseStatus{
		model.InternshipCaseStatusPendingCompanyApproval,
		model.InternshipCaseStatusCompanyApproved,
		model.InternshipCaseStatusUniversityApproved,
		model.InternshipCaseStatusActive,
	}
}

func containsStatus(statuses []model.InternshipCaseStatus, status model.InternshipCaseStatus) bool {
	for _, candidate := range statuses {
		if candidate == status {
			return true
		}
	}
	return false
}
