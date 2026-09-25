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

type UniversityPlacementApprovalInput struct {
	PreferenceID uint
	LetterNumber string
	LetterDate   time.Time
}

type CompanyConfirmationInput struct {
	InternshipSubject string
	StartDate         time.Time
	WorkplaceAddress  string
	WorkplacePhone    string
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

func (service *InternshipService) ListPendingUniversityReviewCases() ([]model.InternshipCase, error) {
	status := model.InternshipCaseStatusPendingUniversityReview
	return service.ListUniversityCases(&status)
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

// ApproveUniversityPlacement atomically selects one submitted preference,
// records the introduction-letter metadata, resolves the opportunity owner as
// the company-supervisor snapshot, and advances the case to company details.
func (service *InternshipService) ApproveUniversityPlacement(caseID uint, input UniversityPlacementApprovalInput) (*model.InternshipCase, error) {
	letterNumber := strings.TrimSpace(input.LetterNumber)
	if letterNumber == "" {
		return nil, ErrIntroductionLetterNumberRequired
	}
	if input.LetterDate.IsZero() {
		return nil, ErrIntroductionLetterDateRequired
	}

	err := service.db.Transaction(func(tx *gorm.DB) error {
		var internshipCase model.InternshipCase
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&internshipCase, caseID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCaseNotFound
		}
		if err != nil {
			return fmt.Errorf("lock university internship case: %w", err)
		}
		if internshipCase.Status != model.InternshipCaseStatusPendingUniversityReview {
			return ErrCaseNotPendingUniversityReview
		}

		var preference model.InternshipPreference
		err = tx.Preload("OpportunityApplication.Opportunity.Creator").First(&preference, input.PreferenceID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPreferenceNotFound
		}
		if err != nil {
			return fmt.Errorf("get university selected preference: %w", err)
		}
		if preference.InternshipCaseID != internshipCase.ID {
			return ErrPreferenceNotInCase
		}
		application := preference.OpportunityApplication
		if application.Status != model.ApplicationStatusAccepted {
			return ErrPreferenceNotAccepted
		}
		opportunity := application.Opportunity
		creator := opportunity.Creator
		if creator.ID == 0 || creator.Role != model.RoleCompanySupervisor || creator.CompanyID == nil ||
			*creator.CompanyID != opportunity.CompanyID {
			return ErrCompanySupervisorResolution
		}

		result := tx.Model(&model.InternshipCase{}).
			Where("id = ? AND status = ?", internshipCase.ID, model.InternshipCaseStatusPendingUniversityReview).
			Updates(map[string]any{
				"selected_preference_id": preference.ID,
				"letter_number":          letterNumber,
				"letter_date":            input.LetterDate.UTC(),
				"company_supervisor_id":  creator.ID,
				"status":                 model.InternshipCaseStatusPendingCompanyDetails,
			})
		if result.Error != nil {
			return fmt.Errorf("approve university placement: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrCaseNotPendingUniversityReview
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return service.getCaseByID(caseID)
}

// CancelUniversityReview atomically closes a case during its first official
// university review. Preferences and recruitment applications are untouched.
func (service *InternshipService) CancelUniversityReview(caseID uint, comment string) (*model.InternshipCase, error) {
	comment = strings.TrimSpace(comment)
	if comment == "" {
		return nil, ErrCancellationCommentRequired
	}

	err := service.db.Transaction(func(tx *gorm.DB) error {
		var internshipCase model.InternshipCase
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&internshipCase, caseID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCaseNotFound
		}
		if err != nil {
			return fmt.Errorf("lock university internship case for cancellation: %w", err)
		}
		if internshipCase.Status != model.InternshipCaseStatusPendingUniversityReview {
			return ErrCaseNotPendingUniversityReview
		}
		now := time.Now()
		result := tx.Model(&model.InternshipCase{}).
			Where("id = ? AND status = ?", internshipCase.ID, model.InternshipCaseStatusPendingUniversityReview).
			Updates(map[string]any{
				"cancellation_comment": comment,
				"cancelled_at":         now,
				"status":               model.InternshipCaseStatusCancelled,
			})
		if result.Error != nil {
			return fmt.Errorf("cancel university internship case: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrCaseNotPendingUniversityReview
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
