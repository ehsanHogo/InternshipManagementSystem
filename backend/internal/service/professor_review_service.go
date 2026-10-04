package service

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"internship-management-system/backend/internal/model"
)

var (
	ErrCaseNotAssignedToProfessor         = fmt.Errorf("%w: case is not assigned to professor", ErrCaseAccessDenied)
	ErrInvalidProfessorResult             = errors.New("invalid professor final result")
	ErrProfessorCaseNotActive             = errors.New("internship case is not active")
	ErrProfessorWeeklyReportsIncomplete   = errors.New("all 8 weekly reports must be approved by both reviewers")
	ErrProfessorCompanyEvaluationRequired = errors.New("company evaluation is required")
	ErrProfessorFinalReportRequired       = errors.New("approved final internship report is required")
)

type ProfessorCompletionInput struct {
	Result  model.ProfessorFinalResult
	Comment *string
}

func (service *InternshipService) ListProfessorCases(professorID uint) ([]model.InternshipCase, error) {
	var cases []model.InternshipCase
	err := service.caseQuery(service.db).
		Where("professor_id = ? AND status IN ?", professorID, professorVisibleStatuses()).
		Order("updated_at DESC").
		Find(&cases).Error
	if err != nil {
		return nil, fmt.Errorf("list professor internship cases: %w", err)
	}
	return cases, nil
}

func (service *InternshipService) GetProfessorCase(professorID, caseID uint) (*model.InternshipCase, error) {
	var internshipCase model.InternshipCase
	err := service.caseQuery(service.db).
		Where("id = ? AND professor_id = ? AND status IN ?", caseID, professorID, professorVisibleStatuses()).
		First(&internshipCase).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCaseAccessDenied
	}
	if err != nil {
		return nil, fmt.Errorf("get professor internship case: %w", err)
	}
	return &internshipCase, nil
}

// CompleteProfessorCase finalizes the qualitative outcome once, using the Case
// professor snapshot. All report mutations also lock this Case before writing.
func (service *InternshipService) CompleteProfessorCase(professorID, caseID uint, input ProfessorCompletionInput) (*model.InternshipCase, error) {
	if !input.Result.Valid() {
		return nil, ErrInvalidProfessorResult
	}

	err := service.db.Transaction(func(tx *gorm.DB) error {
		var internshipCase model.InternshipCase
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&internshipCase, caseID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCaseAccessDenied
		}
		if err != nil {
			return fmt.Errorf("lock professor internship case: %w", err)
		}
		if internshipCase.ProfessorID != professorID {
			return ErrCaseNotAssignedToProfessor
		}
		if internshipCase.Status != model.InternshipCaseStatusActive {
			return ErrProfessorCaseNotActive
		}

		var reports []model.WeeklyReport
		if err := tx.Where("internship_case_id = ?", caseID).Find(&reports).Error; err != nil {
			return fmt.Errorf("load weekly reports for final evaluation: %w", err)
		}
		if !model.WeeklyReportsReadyForFinalEvaluation(reports) {
			return ErrProfessorWeeklyReportsIncomplete
		}

		var evaluationCount int64
		if err := tx.Model(&model.CompanyEvaluation{}).
			Where("internship_case_id = ?", caseID).Count(&evaluationCount).Error; err != nil {
			return fmt.Errorf("check professor company evaluation: %w", err)
		}
		if evaluationCount != 1 {
			return ErrProfessorCompanyEvaluationRequired
		}

		var approvedFinalReportCount int64
		if err := tx.Model(&model.FinalReport{}).
			Where("internship_case_id = ? AND status = ?", caseID, model.FinalReportApproved).
			Count(&approvedFinalReportCount).Error; err != nil {
			return fmt.Errorf("check professor approved final report: %w", err)
		}
		if approvedFinalReportCount != 1 {
			return ErrProfessorFinalReportRequired
		}

		nextStatus := model.InternshipCaseStatusPassed
		if input.Result == model.ProfessorFinalResultFailed {
			nextStatus = model.InternshipCaseStatusFailed
		}
		now := time.Now()
		if err := tx.Model(&internshipCase).UpdateColumns(map[string]any{
			"final_result":      input.Result,
			"professor_comment": trimmedPointer(input.Comment),
			"completed_at":      now,
			"status":            nextStatus,
		}).Error; err != nil {
			return fmt.Errorf("complete professor internship case: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return service.GetProfessorCase(professorID, caseID)
}

func professorVisibleStatuses() []model.InternshipCaseStatus {
	return []model.InternshipCaseStatus{
		model.InternshipCaseStatusActive,
		model.InternshipCaseStatusPassed,
		model.InternshipCaseStatusFailed,
	}
}
