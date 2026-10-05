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
	ErrCaseNotPendingFinalApproval           = errors.New("internship case is not pending final approval")
	ErrPlacementDetailsIncomplete            = errors.New("required placement details are incomplete")
	ErrPlacementRelationshipInvalid          = errors.New("selected placement relationship is invalid")
	ErrCompanyDetailsRevisionCommentRequired = errors.New("company details revision comment is required")
)

func (service *InternshipService) ListPendingFinalApprovalCases() ([]model.InternshipCase, error) {
	status := model.InternshipCaseStatusPendingFinalApproval
	return service.ListUniversityCases(&status)
}

// ApprovePlacementDetails preserves the selected placement, supervisor snapshot,
// letter and company fields, and activates the finally approved case.
func (service *InternshipService) ApprovePlacementDetails(caseID uint) (*model.InternshipCase, error) {
	return service.reviewPlacementDetails(caseID, func(tx *gorm.DB, item *model.InternshipCase) (map[string]any, error) {
		if err := validateFinalPlacement(tx, item); err != nil {
			return nil, err
		}
		return map[string]any{"status": model.InternshipCaseStatusActive, "activated_at": time.Now().UTC()}, nil
	})
}

// RequestPlacementCorrection returns only the placement fields for company editing;
// previously submitted values and all selection/letter metadata remain intact.
func (service *InternshipService) RequestPlacementCorrection(caseID uint, comment string) (*model.InternshipCase, error) {
	return service.reviewPlacementDetails(caseID, func(_ *gorm.DB, _ *model.InternshipCase) (map[string]any, error) {
		comment = strings.TrimSpace(comment)
		if comment == "" {
			return nil, ErrCompanyDetailsRevisionCommentRequired
		}
		return map[string]any{
			"status":                           model.InternshipCaseStatusPendingCompanyDetails,
			"company_details_revision_comment": comment,
		}, nil
	})
}

func (service *InternshipService) reviewPlacementDetails(caseID uint, changes func(*gorm.DB, *model.InternshipCase) (map[string]any, error)) (*model.InternshipCase, error) {
	var resultCase *model.InternshipCase
	err := service.db.Transaction(func(tx *gorm.DB) error {
		var item model.InternshipCase
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, caseID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCaseNotFound
		}
		if err != nil {
			return fmt.Errorf("lock final review case: %w", err)
		}
		if item.Status != model.InternshipCaseStatusPendingFinalApproval {
			return ErrCaseNotPendingFinalApproval
		}
		values, err := changes(tx, &item)
		if err != nil {
			return err
		}
		result := tx.Model(&model.InternshipCase{}).
			Where("id = ? AND status = ?", caseID, model.InternshipCaseStatusPendingFinalApproval).Updates(values)
		if result.Error != nil {
			return fmt.Errorf("review placement details: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrCaseNotPendingFinalApproval
		}
		var updated model.InternshipCase
		if err := service.caseQuery(tx).First(&updated, caseID).Error; err != nil {
			return fmt.Errorf("reload final review case: %w", err)
		}
		resultCase = &updated
		return nil
	})
	return resultCase, err
}

func validateFinalPlacement(tx *gorm.DB, item *model.InternshipCase) error {
	if item.SelectedPreferenceID == nil || item.CompanySupervisorID == nil ||
		blankPlacementField(item.LetterNumber) || item.LetterDate == nil || item.LetterDate.IsZero() ||
		blankPlacementField(item.InternshipSubject) || item.StartDate == nil || item.StartDate.IsZero() ||
		blankPlacementField(item.WorkplaceAddress) || blankPlacementField(item.WorkplacePhone) {
		return ErrPlacementDetailsIncomplete
	}
	// Share locks keep the validated relationship stable until the case commits.
	lock := func(db *gorm.DB) *gorm.DB { return db.Clauses(clause.Locking{Strength: "SHARE"}) }
	var preference model.InternshipPreference
	err := lock(tx).Preload("OpportunityApplication", lock).
		Preload("OpportunityApplication.Opportunity", lock).
		Preload("OpportunityApplication.Opportunity.Company", lock).
		First(&preference, *item.SelectedPreferenceID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrPlacementRelationshipInvalid
	}
	if err != nil {
		return fmt.Errorf("load final placement relationship: %w", err)
	}
	var supervisor model.User
	err = lock(tx).First(&supervisor, *item.CompanySupervisorID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrPlacementRelationshipInvalid
	}
	if err != nil {
		return fmt.Errorf("load final placement supervisor: %w", err)
	}
	application := preference.OpportunityApplication
	opportunity := application.Opportunity
	if preference.InternshipCaseID != item.ID || application.ID == 0 ||
		application.ID != preference.OpportunityApplicationID || application.StudentID != item.StudentID ||
		application.Status != model.ApplicationStatusAccepted || opportunity.ID == 0 ||
		opportunity.ID != application.OpportunityID || opportunity.Company.ID == 0 ||
		opportunity.Company.ID != opportunity.CompanyID || supervisor.Role != model.RoleCompanySupervisor ||
		supervisor.CompanyID == nil || *supervisor.CompanyID != opportunity.CompanyID {
		return ErrPlacementRelationshipInvalid
	}
	return RequireApprovedRegistration(tx, opportunity.CompanyID)
}

func blankPlacementField(value *string) bool {
	return value == nil || strings.TrimSpace(*value) == ""
}
