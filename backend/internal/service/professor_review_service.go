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
	ErrInvalidProfessorResult             = errors.New("invalid professor final result")
	ErrProfessorCaseNotActive             = errors.New("internship case is not active")
	ErrProfessorWeeklyReportsIncomplete   = errors.New("all 8 weekly reports must be submitted and confirmed")
	ErrProfessorCompanyEvaluationRequired = errors.New("company evaluation is required")
	ErrProfessorFinalReportRequired       = errors.New("final internship report is required")
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
			return ErrCaseAccessDenied
		}
		if internshipCase.Status != model.InternshipCaseStatusActive {
			return ErrProfessorCaseNotActive
		}

		var reportCount, confirmedReportCount int64
		if err := tx.Model(&model.WeeklyReport{}).
			Where("internship_case_id = ?", caseID).Count(&reportCount).Error; err != nil {
			return fmt.Errorf("count professor weekly reports: %w", err)
		}
		if err := tx.Model(&model.WeeklyReport{}).
			Where("internship_case_id = ? AND is_confirmed = ?", caseID, true).
			Count(&confirmedReportCount).Error; err != nil {
			return fmt.Errorf("count professor confirmed weekly reports: %w", err)
		}
		if reportCount != 8 || confirmedReportCount != 8 {
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

		if internshipCase.FinalReportFileID == nil {
			return ErrProfessorFinalReportRequired
		}
		var fileCount int64
		if err := tx.Model(&model.File{}).Where("id = ?", *internshipCase.FinalReportFileID).Count(&fileCount).Error; err != nil {
			return fmt.Errorf("check professor final report: %w", err)
		}
		if fileCount != 1 {
			return ErrProfessorFinalReportRequired
		}

		now := time.Now()
		if err := tx.Model(&internshipCase).Updates(map[string]any{
			"final_result":      input.Result,
			"professor_comment": trimmedPointer(input.Comment),
			"completed_at":      now,
			"status":            model.InternshipCaseStatusCompleted,
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
		model.InternshipCaseStatusCompleted,
	}
}
