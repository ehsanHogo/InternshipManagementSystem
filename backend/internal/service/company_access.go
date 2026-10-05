package service

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"internship-management-system/backend/internal/model"
)

var ErrCompanyRegistrationRequired = errors.New("company registration is not approved")

// Registration approval is an access permission; IsApproved is university trust.
func RequireApprovedRegistration(db *gorm.DB, companyID uint) error {
	var company model.Company
	if err := db.Select("id", "registration_status").First(&company, companyID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrCompanyRegistrationRequired
	} else if err != nil {
		return fmt.Errorf("load company registration permission: %w", err)
	}
	if company.RegistrationStatus != model.CompanyRegistrationStatusApproved {
		return ErrCompanyRegistrationRequired
	}
	return nil
}

func ApprovedCompanyID(db *gorm.DB, supervisorID uint) (uint, error) {
	var supervisor model.User
	if err := db.Select("id", "company_id").Where("id = ? AND role = ?", supervisorID, model.RoleCompanySupervisor).First(&supervisor).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, ErrOpportunityCompanyMissing
	} else if err != nil {
		return 0, fmt.Errorf("load company membership: %w", err)
	}
	if supervisor.CompanyID == nil || *supervisor.CompanyID == 0 {
		return 0, ErrOpportunityCompanyMissing
	}
	if err := RequireApprovedRegistration(db, *supervisor.CompanyID); err != nil {
		return 0, err
	}
	return *supervisor.CompanyID, nil
}

// Preserve case authorization errors for missing or invalid company membership.
func requireCompanyCaseAccess(db *gorm.DB, supervisorID uint) error {
	_, err := ApprovedCompanyID(db, supervisorID)
	if errors.Is(err, ErrOpportunityCompanyMissing) {
		return ErrCaseAccessDenied
	}
	return err
}
