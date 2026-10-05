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

var (
	ErrRegistrationTransition   = errors.New("invalid company registration transition")
	ErrRegistrationReviewReason = errors.New("registration rejection reason required")
	ErrRegistrationReviewer     = errors.New("registration reviewer must be admin")
)

type CompanyRegistrationService struct{ db *gorm.DB }
type CompanyRegistrationDetail struct {
	Company     model.Company      `json:"company"`
	Supervisors []model.PublicUser `json:"supervisors"`
	Reviewer    *model.PublicUser  `json:"reviewer,omitempty"`
}

func NewCompanyRegistrationService(db *gorm.DB) *CompanyRegistrationService {
	return &CompanyRegistrationService{db: db}
}
func (service *CompanyRegistrationService) List(status model.CompanyRegistrationStatus) ([]model.Company, error) {
	if status != "" && !status.Valid() {
		return nil, ErrInvalidCompanyRegistration
	}
	companies := []model.Company{}
	query := service.db.Order("created_at DESC, id DESC")
	if status != "" {
		query = query.Where("registration_status = ?", status)
	}
	if err := query.Find(&companies).Error; err != nil {
		return nil, fmt.Errorf("list company registrations: %w", err)
	}
	return companies, nil
}
func (service *CompanyRegistrationService) Detail(id uint) (*CompanyRegistrationDetail, error) {
	detail := &CompanyRegistrationDetail{Supervisors: []model.PublicUser{}}
	if err := service.db.First(&detail.Company, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCompanyProfileNotFound
	} else if err != nil {
		return nil, err
	}
	var supervisors []model.User
	if err := service.db.Where("company_id = ? AND role = ?", id, model.RoleCompanySupervisor).Order("id").Find(&supervisors).Error; err != nil {
		return nil, err
	}
	for _, user := range supervisors {
		detail.Supervisors = append(detail.Supervisors, user.Public())
	}
	if detail.Company.RegistrationReviewedBy != nil {
		var user model.User
		if err := service.db.First(&user, *detail.Company.RegistrationReviewedBy).Error; err != nil {
			return nil, err
		}
		public := user.Public()
		detail.Reviewer = &public
	}
	return detail, nil
}
func (service *CompanyRegistrationService) Review(adminID, companyID uint, status model.CompanyRegistrationStatus, reason string) (*model.Company, error) {
	if status != model.CompanyRegistrationStatusApproved && status != model.CompanyRegistrationStatusRejected {
		return nil, ErrRegistrationTransition
	}
	reason = strings.TrimSpace(reason)
	if status == model.CompanyRegistrationStatusRejected && (reason == "" || len([]rune(reason)) > 5000) {
		return nil, ErrRegistrationReviewReason
	}
	var company model.Company
	err := service.db.Transaction(func(tx *gorm.DB) error {
		var admin model.User
		if err := tx.Where("id = ? AND role = ?", adminID, model.RoleAdmin).First(&admin).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRegistrationReviewer
		} else if err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&company, companyID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCompanyProfileNotFound
		} else if err != nil {
			return err
		}
		if company.RegistrationStatus != model.CompanyRegistrationStatusPending {
			return ErrRegistrationTransition
		}
		now := time.Now().UTC()
		var rejection *string
		if status == model.CompanyRegistrationStatusRejected {
			rejection = &reason
		}
		if err := tx.Model(&company).Updates(map[string]any{
			"registration_status": status, "registration_reviewed_at": now, "registration_reviewed_by": adminID, "registration_rejection_reason": rejection,
		}).Error; err != nil {
			return err
		}
		return tx.First(&company, companyID).Error
	})
	if err != nil {
		return nil, err
	}
	return &company, nil
}

// Company-row locks serialize profile edits, resubmissions, and admin review.
func (service *CompanyAccountService) lockedProfile(tx *gorm.DB, userID uint) (*CompanyAccount, error) {
	account, err := NewCompanyAccountService(tx).GetProfile(userID)
	if err != nil {
		return nil, err
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account.Company, account.Company.ID).Error; err != nil {
		return nil, err
	}
	if err := tx.First(&account.Supervisor, userID).Error; err != nil {
		return nil, err
	}
	return account, nil
}
func (service *CompanyAccountService) UpdateProfile(userID uint, input CompanyRegistrationInput) (*CompanyAccount, error) {
	// Password is never accepted by the profile API or changed by this operation.
	normalized, err := normalizeCompanyRegistrationDetails(input)
	if err != nil {
		return nil, err
	}
	var result *CompanyAccount
	err = service.db.Transaction(func(tx *gorm.DB) error {
		account, err := service.lockedProfile(tx, userID)
		if err != nil {
			return err
		}
		if account.Company.RegistrationStatus == model.CompanyRegistrationStatusApproved {
			return ErrRegistrationTransition
		}
		checks := []struct {
			table, query string
			value        any
			id           uint
			duplicate    error
		}{
			{"users", "email = ?", normalized.Supervisor.Email, userID, ErrEmailAlreadyExists},
			{"companies", "LOWER(name) = LOWER(?)", normalized.Company.Name, account.Company.ID, ErrCompanyNameAlreadyExists},
			{"companies", "national_id = ?", normalized.Company.NationalID, account.Company.ID, ErrNationalIDAlreadyExists},
			{"companies", "economic_code = ?", normalized.Company.EconomicCode, account.Company.ID, ErrEconomicCodeAlreadyExists},
		}
		for _, check := range checks {
			var count int64
			if err := tx.Table(check.table).Where("id <> ?", check.id).Where(check.query, check.value).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return check.duplicate
			}
		}
		c := normalized.Company
		u := normalized.Supervisor
		if err := tx.Model(&account.Company).Updates(map[string]any{"name": c.Name, "national_id": c.NationalID, "economic_code": c.EconomicCode, "website": optionalString(c.Website), "phone": c.Phone, "email": c.Email, "address": c.Address}).Error; err != nil {
			return classifyRegistrationConstraint(err)
		}
		if err := tx.Model(&account.Supervisor).Updates(map[string]any{"full_name": u.FullName, "email": u.Email, "phone": u.Phone, "job_title": u.JobTitle}).Error; err != nil {
			return classifyRegistrationConstraint(err)
		}
		result, err = NewCompanyAccountService(tx).GetProfile(userID)
		return err
	})
	return result, err
}
func (service *CompanyAccountService) Resubmit(userID uint) (*CompanyAccount, error) {
	var result *CompanyAccount
	err := service.db.Transaction(func(tx *gorm.DB) error {
		account, err := service.lockedProfile(tx, userID)
		if err != nil {
			return err
		}
		if account.Company.RegistrationStatus != model.CompanyRegistrationStatusRejected {
			return ErrRegistrationTransition
		}
		c, u := account.Company, account.Supervisor
		value := func(v *string) string {
			if v == nil {
				return ""
			}
			return *v
		}
		_, err = normalizeCompanyRegistrationDetails(CompanyRegistrationInput{
			Company:    CompanyInput{Name: c.Name, NationalID: c.NationalID, EconomicCode: c.EconomicCode, Website: value(c.Website), Phone: value(c.Phone), Email: value(c.Email), Address: value(c.Address)},
			Supervisor: CompanySupervisorRegistrationInput{FullName: u.FullName, Email: u.Email, Phone: value(u.Phone), JobTitle: value(u.JobTitle)},
		})
		if err != nil {
			return err
		}
		// Keep the last review timestamp/reviewer; clear the active rejection reason.
		if err := tx.Model(&account.Company).Updates(map[string]any{"registration_status": model.CompanyRegistrationStatusPending, "registration_rejection_reason": nil}).Error; err != nil {
			return err
		}
		result, err = NewCompanyAccountService(tx).GetProfile(userID)
		return err
	})
	return result, err
}
