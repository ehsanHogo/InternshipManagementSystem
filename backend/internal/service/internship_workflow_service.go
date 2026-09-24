package service

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

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

// Deprecated: the V1 endpoint manually selected an unrelated supervisor. V2 will
// resolve the supervisor through the selected opportunity application.
func (service *InternshipService) SendToCompany(caseID uint, input SendToCompanyInput) (*model.InternshipCase, error) {
	return nil, ErrObsoleteWorkflow
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

// Deprecated: company placement details will be submitted by the supervisor
// resolved from the selected opportunity in a later milestone.
func (service *InternshipService) ConfirmCompanyCase(supervisorID, caseID uint, input CompanyConfirmationInput) (*model.InternshipCase, error) {
	return nil, ErrObsoleteWorkflow
}

// Deprecated: final approval transitions belong to the V2 workflow milestone.
func (service *InternshipService) ApproveUniversityCase(caseID uint) (*model.InternshipCase, error) {
	return nil, ErrObsoleteWorkflow
}

// Deprecated: automatic activation is intentionally deferred.
func (service *InternshipService) ActivateUniversityCase(caseID uint) (*model.InternshipCase, error) {
	return nil, ErrObsoleteWorkflow
}

func universityCaseStatuses() []model.InternshipCaseStatus {
	return []model.InternshipCaseStatus{
		model.InternshipCaseStatusPendingUniversityReview,
		model.InternshipCaseStatusPendingCompanyDetails,
		model.InternshipCaseStatusPendingFinalApproval,
		model.InternshipCaseStatusReadyToStart,
		model.InternshipCaseStatusActive,
		model.InternshipCaseStatusCompleted,
		model.InternshipCaseStatusCancelled,
	}
}

func companyVisibleStatuses() []model.InternshipCaseStatus {
	return []model.InternshipCaseStatus{
		model.InternshipCaseStatusPendingCompanyDetails,
		model.InternshipCaseStatusPendingFinalApproval,
		model.InternshipCaseStatusReadyToStart,
		model.InternshipCaseStatusActive,
		model.InternshipCaseStatusCompleted,
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
