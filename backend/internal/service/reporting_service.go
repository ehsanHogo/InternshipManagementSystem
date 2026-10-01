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
	ErrWeeklyReportNotFound        = errors.New("weekly report not found")
	ErrInvalidWeeklyReport         = errors.New("invalid weekly report")
	ErrDuplicateWeeklyReport       = errors.New("weekly report for this week already exists")
	ErrWeeklyReportState           = errors.New("weekly report action is not allowed in its current state")
	ErrWeeklyReviewCommentRequired = errors.New("revision feedback is required")
	ErrEvaluationNotFound          = errors.New("company evaluation not found")
	ErrInvalidEvaluation           = errors.New("invalid company evaluation")
	ErrDuplicateEvaluation         = errors.New("company evaluation already exists")
	ErrWeeklyReportsIncomplete     = errors.New("all 8 weekly reports must be approved by the company before final company evaluation")
	ErrFileNotFound                = errors.New("file not found")
)

type WeeklyReportInput struct {
	WeekNumber          int
	StartDate           time.Time
	EndDate             time.Time
	ActivityDescription string
}

type CompanyEvaluationInput struct {
	AttendanceRating         model.EvaluationRating
	ParticipationRating      model.EvaluationRating
	LearningRating           model.EvaluationRating
	InterestRating           model.EvaluationRating
	PersistenceRating        model.EvaluationRating
	SuggestionRating         model.EvaluationRating
	ResourceUsageRating      model.EvaluationRating
	ReportQualityRating      model.EvaluationRating
	ProjectPerformanceRating model.EvaluationRating
	LeaveDays                int
	AbsenceDays              int
	Suggestions              *string
}

func (service *InternshipService) ListStudentWeeklyReports(studentID uint) ([]model.WeeklyReport, error) {
	var internshipCase model.InternshipCase
	err := service.db.Where("student_id = ? AND status IN ?", studentID, []model.InternshipCaseStatus{
		model.InternshipCaseStatusActive, model.InternshipCaseStatusCompleted,
	}).Order("created_at DESC").First(&internshipCase).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrInvalidCaseStatus
	}
	if err != nil {
		return nil, fmt.Errorf("get reporting-visible student case: %w", err)
	}
	return service.listWeeklyReports(internshipCase.ID)
}

func (service *InternshipService) CreateWeeklyReport(studentID uint, input WeeklyReportInput) (*model.WeeklyReport, error) {
	if err := validateWeeklyReport(input); err != nil {
		return nil, err
	}
	var report model.WeeklyReport
	err := service.db.Transaction(func(tx *gorm.DB) error {
		internshipCase, err := service.findStudentActiveCase(tx, studentID, true)
		if err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.WeeklyReport{}).Where("internship_case_id = ? AND week_number = ?", internshipCase.ID, input.WeekNumber).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return ErrDuplicateWeeklyReport
		}
		report = model.WeeklyReport{
			InternshipCaseID: internshipCase.ID,
			WeekNumber:       input.WeekNumber, StartDate: input.StartDate, EndDate: input.EndDate,
			ActivityDescription: strings.TrimSpace(input.ActivityDescription),
			CompanyReviewStatus: model.WeeklyReviewPending, ProfessorReviewStatus: model.WeeklyReviewPending,
		}
		return tx.Create(&report).Error
	})
	if err != nil {
		return nil, err
	}
	return &report, nil
}

func (service *InternshipService) GetStudentWeeklyReport(studentID, reportID uint) (*model.WeeklyReport, error) {
	var report model.WeeklyReport
	err := service.db.Model(&model.WeeklyReport{}).Joins("JOIN internship_cases c ON c.id = weekly_reports.internship_case_id").Where("weekly_reports.id = ? AND c.student_id = ?", reportID, studentID).First(&report).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrWeeklyReportNotFound
	}
	if err != nil {
		return nil, err
	}
	return &report, nil
}

// Every mutation locks the case before the report. This also serializes activation,
// completion, evaluation readiness, resubmission, and simultaneous reviews.
func lockWeeklyReport(tx *gorm.DB, caseID, reportID uint) (*model.WeeklyReport, error) {
	var report model.WeeklyReport
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND internship_case_id = ?", reportID, caseID).First(&report).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrWeeklyReportNotFound
	}
	if err != nil {
		return nil, err
	}
	return &report, nil
}

func (service *InternshipService) mutateStudentWeeklyReport(studentID, reportID uint, mutate func(*gorm.DB, *model.WeeklyReport) error) (*model.WeeklyReport, error) {
	var result *model.WeeklyReport
	err := service.db.Transaction(func(tx *gorm.DB) error {
		internshipCase, err := service.findStudentActiveCase(tx, studentID, true)
		if err != nil {
			return err
		}
		report, err := lockWeeklyReport(tx, internshipCase.ID, reportID)
		if err != nil {
			return err
		}
		if report.Status() != model.WeeklyReportDraft && report.Status() != model.WeeklyReportRevisionRequested {
			return ErrWeeklyReportState
		}
		if err := mutate(tx, report); err != nil {
			return err
		}
		result = report
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (service *InternshipService) UpdateWeeklyReport(studentID, reportID uint, input WeeklyReportInput) (*model.WeeklyReport, error) {
	if err := validateWeeklyReport(input); err != nil {
		return nil, err
	}
	return service.mutateStudentWeeklyReport(studentID, reportID, func(tx *gorm.DB, report *model.WeeklyReport) error {
		if input.WeekNumber != report.WeekNumber {
			return ErrInvalidWeeklyReport
		}
		report.StartDate, report.EndDate = input.StartDate, input.EndDate
		report.ActivityDescription = strings.TrimSpace(input.ActivityDescription)
		return tx.Save(report).Error
	})
}

func (service *InternshipService) SubmitWeeklyReport(studentID, reportID uint) (*model.WeeklyReport, error) {
	return service.mutateStudentWeeklyReport(studentID, reportID, func(tx *gorm.DB, report *model.WeeklyReport) error {
		if err := validateWeeklyReport(WeeklyReportInput{WeekNumber: report.WeekNumber, StartDate: report.StartDate, EndDate: report.EndDate, ActivityDescription: report.ActivityDescription}); err != nil {
			return err
		}
		now := time.Now()
		report.SubmittedAt = &now
		// Keep the latest feedback until its author reviews the new submission.
		report.CompanyReviewStatus, report.ProfessorReviewStatus = model.WeeklyReviewPending, model.WeeklyReviewPending
		return tx.Save(report).Error
	})
}

func (service *InternshipService) ListCompanyWeeklyReports(supervisorID, caseID uint) ([]model.WeeklyReport, error) {
	if _, err := service.GetCompanyCase(supervisorID, caseID); err != nil {
		return nil, err
	}
	return service.listSubmittedWeeklyReports(caseID)
}
func (service *InternshipService) ListProfessorWeeklyReports(professorID, caseID uint) ([]model.WeeklyReport, error) {
	if _, err := service.GetProfessorCase(professorID, caseID); err != nil {
		return nil, err
	}
	return service.listSubmittedWeeklyReports(caseID)
}
func (service *InternshipService) listSubmittedWeeklyReports(caseID uint) ([]model.WeeklyReport, error) {
	reports := []model.WeeklyReport{}
	err := service.db.Where("internship_case_id = ? AND submitted_at IS NOT NULL", caseID).Order("week_number ASC").Find(&reports).Error
	return reports, err
}
func (service *InternshipService) GetReviewerWeeklyReport(reviewerID, caseID, reportID uint, role model.Role) (*model.WeeklyReport, error) {
	var err error
	switch role {
	case model.RoleCompanySupervisor:
		_, err = service.GetCompanyCase(reviewerID, caseID)
	case model.RoleProfessor:
		_, err = service.GetProfessorCase(reviewerID, caseID)
	default:
		return nil, ErrCaseAccessDenied
	}
	if err != nil {
		return nil, err
	}
	var report model.WeeklyReport
	err = service.db.Where("id = ? AND internship_case_id = ? AND submitted_at IS NOT NULL", reportID, caseID).First(&report).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrWeeklyReportNotFound
	}
	if err != nil {
		return nil, err
	}
	return &report, nil
}

func (service *InternshipService) ReviewWeeklyReportByCompany(supervisorID, caseID, reportID uint, decision model.WeeklyReviewStatus, comment *string) (*model.WeeklyReport, error) {
	return service.reviewWeeklyReport(supervisorID, caseID, reportID, model.RoleCompanySupervisor, decision, comment)
}
func (service *InternshipService) ReviewWeeklyReportByProfessor(professorID, caseID, reportID uint, decision model.WeeklyReviewStatus, comment *string) (*model.WeeklyReport, error) {
	return service.reviewWeeklyReport(professorID, caseID, reportID, model.RoleProfessor, decision, comment)
}
func (service *InternshipService) reviewWeeklyReport(reviewerID, caseID, reportID uint, role model.Role, decision model.WeeklyReviewStatus, comment *string) (*model.WeeklyReport, error) {
	if decision != model.WeeklyReviewApproved && decision != model.WeeklyReviewRevisionRequested {
		return nil, ErrInvalidWeeklyReport
	}
	comment = trimmedPointer(comment)
	if decision == model.WeeklyReviewRevisionRequested && comment == nil {
		return nil, ErrWeeklyReviewCommentRequired
	}
	var result *model.WeeklyReport
	err := service.db.Transaction(func(tx *gorm.DB) error {
		switch role {
		case model.RoleCompanySupervisor:
			if _, err := service.findAssignedActiveCase(tx, reviewerID, caseID, true); err != nil {
				return err
			}
		case model.RoleProfessor:
			var internshipCase model.InternshipCase
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND professor_id = ?", caseID, reviewerID).First(&internshipCase).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCaseAccessDenied
			}
			if err != nil {
				return err
			}
			if internshipCase.Status != model.InternshipCaseStatusActive {
				return ErrInvalidCaseStatus
			}
		default:
			return ErrCaseAccessDenied
		}
		report, err := lockWeeklyReport(tx, caseID, reportID)
		if err != nil {
			return err
		}
		if report.Status() != model.WeeklyReportSubmitted {
			return ErrWeeklyReportState
		}
		now := time.Now()
		if role == model.RoleCompanySupervisor {
			if report.CompanyReviewStatus != model.WeeklyReviewPending {
				return ErrWeeklyReportState
			}
			report.CompanyReviewStatus, report.CompanyReviewComment, report.CompanyReviewedAt = decision, comment, &now
		} else {
			if report.ProfessorReviewStatus != model.WeeklyReviewPending {
				return ErrWeeklyReportState
			}
			report.ProfessorReviewStatus, report.ProfessorReviewComment, report.ProfessorReviewedAt = decision, comment, &now
		}
		if err := tx.Save(report).Error; err != nil {
			return err
		}
		result = report
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (service *InternshipService) GetCompanyEvaluation(supervisorID, caseID uint) (*model.CompanyEvaluation, error) {
	if _, err := service.GetCompanyCase(supervisorID, caseID); err != nil {
		return nil, err
	}
	var evaluation model.CompanyEvaluation
	err := service.db.Where("internship_case_id = ?", caseID).First(&evaluation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrEvaluationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get company evaluation: %w", err)
	}
	return &evaluation, nil
}

func (service *InternshipService) CreateCompanyEvaluation(supervisorID, caseID uint, input CompanyEvaluationInput) (*model.CompanyEvaluation, error) {
	if !validEvaluationInput(input) {
		return nil, ErrInvalidEvaluation
	}
	var evaluation model.CompanyEvaluation
	err := service.db.Transaction(func(tx *gorm.DB) error {
		if _, err := service.findAssignedActiveCase(tx, supervisorID, caseID, true); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.CompanyEvaluation{}).Where("internship_case_id = ?", caseID).Count(&count).Error; err != nil {
			return fmt.Errorf("check company evaluation: %w", err)
		}
		if count != 0 {
			return ErrDuplicateEvaluation
		}

		var reportCount int64
		var companyApprovedReportCount int64
		if err := tx.Model(&model.WeeklyReport{}).Where("internship_case_id = ?", caseID).Count(&reportCount).Error; err != nil {
			return fmt.Errorf("count weekly reports for evaluation: %w", err)
		}
		if err := tx.Model(&model.WeeklyReport{}).
			Where("internship_case_id = ? AND company_review_status = ?", caseID, model.WeeklyReviewApproved).
			Count(&companyApprovedReportCount).Error; err != nil {
			return fmt.Errorf("count company-approved weekly reports for evaluation: %w", err)
		}
		if reportCount != 8 || companyApprovedReportCount != 8 {
			return ErrWeeklyReportsIncomplete
		}
		evaluation = model.CompanyEvaluation{
			InternshipCaseID: caseID, CompanySupervisorID: supervisorID,
			AttendanceRating: input.AttendanceRating, ParticipationRating: input.ParticipationRating,
			LearningRating: input.LearningRating, InterestRating: input.InterestRating,
			PersistenceRating: input.PersistenceRating, SuggestionRating: input.SuggestionRating,
			ResourceUsageRating: input.ResourceUsageRating, ReportQualityRating: input.ReportQualityRating,
			ProjectPerformanceRating: input.ProjectPerformanceRating,
			LeaveDays:                input.LeaveDays, AbsenceDays: input.AbsenceDays,
			Suggestions: trimmedPointer(input.Suggestions), SubmittedAt: time.Now(),
		}
		if err := tx.Create(&evaluation).Error; err != nil {
			return fmt.Errorf("create company evaluation: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &evaluation, nil
}

func (service *InternshipService) GetAccessibleFile(userID uint, role model.Role, fileID uint) (*model.File, error) {
	var file model.File
	if err := service.db.First(&file, fileID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrFileNotFound
	} else if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}

	finalReportQuery := service.db.Model(&model.FinalReport{}).
		Joins("JOIN internship_cases ON internship_cases.id = final_reports.internship_case_id").
		Where("final_reports.current_file_id = ?", fileID)
	switch role {
	case model.RoleStudent:
		finalReportQuery = finalReportQuery.Where("student_id = ?", userID)
	case model.RoleProfessor:
		finalReportQuery = finalReportQuery.Where("professor_id = ?", userID)
	case model.RoleCompanySupervisor:
		// The existing company case page grants its assigned supervisor read-only downloads.
		finalReportQuery = finalReportQuery.Where("company_supervisor_id = ?", userID)
	case model.RoleUniversitySupervisor:
		// University supervisors retain their existing system-wide final-report access.
	default:
		return nil, ErrCaseAccessDenied
	}
	var finalReportCount int64
	if err := finalReportQuery.Count(&finalReportCount).Error; err != nil {
		return nil, fmt.Errorf("authorize final report file: %w", err)
	}
	if finalReportCount > 0 {
		return &file, nil
	}

	var resumeCount int64
	resumeQuery := service.db.Model(&model.OpportunityApplication{}).
		Joins("JOIN internship_opportunities ON internship_opportunities.id = opportunity_applications.opportunity_id").
		Where("opportunity_applications.resume_file_id = ?", fileID)
	switch role {
	case model.RoleStudent:
		resumeQuery = resumeQuery.Where("opportunity_applications.student_id = ?", userID)
	case model.RoleCompanySupervisor:
		resumeQuery = resumeQuery.Joins("JOIN users AS resume_supervisors ON resume_supervisors.company_id = internship_opportunities.company_id").
			Where("resume_supervisors.id = ? AND resume_supervisors.role = ?", userID, model.RoleCompanySupervisor)
	default:
		return nil, ErrCaseAccessDenied
	}
	if err := resumeQuery.Count(&resumeCount).Error; err != nil {
		return nil, fmt.Errorf("authorize resume file: %w", err)
	}
	if resumeCount == 0 {
		return nil, ErrCaseAccessDenied
	}
	return &file, nil
}

func (service *InternshipService) listWeeklyReports(caseID uint) ([]model.WeeklyReport, error) {
	reports := []model.WeeklyReport{}
	if err := service.db.Where("internship_case_id = ?", caseID).Order("week_number ASC").Find(&reports).Error; err != nil {
		return nil, fmt.Errorf("list weekly reports: %w", err)
	}
	return reports, nil
}

func (service *InternshipService) findStudentActiveCase(db *gorm.DB, studentID uint, lock bool) (*model.InternshipCase, error) {
	query := db.Where("student_id = ? AND status = ?", studentID, model.InternshipCaseStatusActive)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var internshipCase model.InternshipCase
	if err := query.Order("created_at DESC").First(&internshipCase).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrInvalidCaseStatus
	} else if err != nil {
		return nil, fmt.Errorf("get active internship case: %w", err)
	}
	return &internshipCase, nil
}

func (service *InternshipService) findAssignedActiveCase(db *gorm.DB, supervisorID, caseID uint, lock bool) (*model.InternshipCase, error) {
	query := service.companyCaseQuery(db, supervisorID).Where("internship_cases.id = ?", caseID)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var internshipCase model.InternshipCase
	if err := query.First(&internshipCase).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCaseAccessDenied
	} else if err != nil {
		return nil, fmt.Errorf("get assigned internship case: %w", err)
	}
	if internshipCase.Status != model.InternshipCaseStatusActive {
		return nil, ErrInvalidCaseStatus
	}
	return &internshipCase, nil
}

func validateWeeklyReport(input WeeklyReportInput) error {
	if input.WeekNumber < 1 || input.WeekNumber > 8 || input.StartDate.IsZero() || input.EndDate.IsZero() ||
		input.EndDate.Before(input.StartDate) || strings.TrimSpace(input.ActivityDescription) == "" {
		return ErrInvalidWeeklyReport
	}
	return nil
}

func validEvaluationInput(input CompanyEvaluationInput) bool {
	ratings := []model.EvaluationRating{
		input.AttendanceRating, input.ParticipationRating, input.LearningRating,
		input.InterestRating, input.PersistenceRating, input.SuggestionRating,
		input.ResourceUsageRating, input.ReportQualityRating, input.ProjectPerformanceRating,
	}
	if input.LeaveDays < 0 || input.AbsenceDays < 0 {
		return false
	}
	for _, rating := range ratings {
		if !rating.Valid() {
			return false
		}
	}
	return true
}
