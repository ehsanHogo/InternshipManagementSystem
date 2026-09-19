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
	ErrWeeklyReportNotFound    = errors.New("weekly report not found")
	ErrInvalidWeeklyReport     = errors.New("invalid weekly report")
	ErrDuplicateWeeklyReport   = errors.New("weekly report for this week already exists")
	ErrWeeklyReportConfirmed   = errors.New("confirmed weekly report cannot be changed")
	ErrEvaluationNotFound      = errors.New("company evaluation not found")
	ErrInvalidEvaluation       = errors.New("invalid company evaluation")
	ErrDuplicateEvaluation     = errors.New("company evaluation already exists")
	ErrWeeklyReportsIncomplete = errors.New("all 8 weekly reports must be submitted and confirmed before final company evaluation")
	ErrFileNotFound            = errors.New("file not found")
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
	internshipCase, err := service.findStudentActiveCase(service.db, studentID, false)
	if err != nil {
		return nil, err
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
		if err := tx.Model(&model.WeeklyReport{}).
			Where("internship_case_id = ? AND week_number = ?", internshipCase.ID, input.WeekNumber).
			Count(&count).Error; err != nil {
			return fmt.Errorf("check weekly report: %w", err)
		}
		if count != 0 {
			return ErrDuplicateWeeklyReport
		}
		report = model.WeeklyReport{
			InternshipCaseID: internshipCase.ID,
			WeekNumber:       input.WeekNumber, StartDate: input.StartDate, EndDate: input.EndDate,
			ActivityDescription: strings.TrimSpace(input.ActivityDescription), SubmittedAt: time.Now(),
		}
		if err := tx.Create(&report).Error; err != nil {
			return fmt.Errorf("create weekly report: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &report, nil
}

func (service *InternshipService) UpdateWeeklyReport(studentID, reportID uint, input WeeklyReportInput) (*model.WeeklyReport, error) {
	if err := validateWeeklyReport(input); err != nil {
		return nil, err
	}
	err := service.db.Transaction(func(tx *gorm.DB) error {
		internshipCase, err := service.findStudentActiveCase(tx, studentID, true)
		if err != nil {
			return err
		}
		var report model.WeeklyReport
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND internship_case_id = ?", reportID, internshipCase.ID).First(&report).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrWeeklyReportNotFound
		}
		if err != nil {
			return fmt.Errorf("get weekly report: %w", err)
		}
		if report.IsConfirmed {
			return ErrWeeklyReportConfirmed
		}
		var count int64
		if err := tx.Model(&model.WeeklyReport{}).
			Where("internship_case_id = ? AND week_number = ? AND id <> ?", internshipCase.ID, input.WeekNumber, report.ID).
			Count(&count).Error; err != nil {
			return fmt.Errorf("check weekly report: %w", err)
		}
		if count != 0 {
			return ErrDuplicateWeeklyReport
		}
		return tx.Model(&report).Updates(map[string]any{
			"week_number": input.WeekNumber, "start_date": input.StartDate, "end_date": input.EndDate,
			"activity_description": strings.TrimSpace(input.ActivityDescription), "submitted_at": time.Now(),
		}).Error
	})
	if err != nil {
		return nil, err
	}
	var report model.WeeklyReport
	if err := service.db.First(&report, reportID).Error; err != nil {
		return nil, fmt.Errorf("reload weekly report: %w", err)
	}
	return &report, nil
}

func (service *InternshipService) ListCompanyWeeklyReports(supervisorID, caseID uint) ([]model.WeeklyReport, error) {
	if _, err := service.findAssignedActiveCase(service.db, supervisorID, caseID, false); err != nil {
		return nil, err
	}
	return service.listWeeklyReports(caseID)
}

func (service *InternshipService) ConfirmWeeklyReport(supervisorID, caseID, reportID uint, comment *string) (*model.WeeklyReport, error) {
	err := service.db.Transaction(func(tx *gorm.DB) error {
		if _, err := service.findAssignedActiveCase(tx, supervisorID, caseID, true); err != nil {
			return err
		}
		var report model.WeeklyReport
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND internship_case_id = ?", reportID, caseID).First(&report).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrWeeklyReportNotFound
		}
		if err != nil {
			return fmt.Errorf("get weekly report: %w", err)
		}
		if report.IsConfirmed {
			return ErrWeeklyReportConfirmed
		}
		return tx.Model(&report).Updates(map[string]any{
			"is_confirmed": true, "supervisor_comment": trimmedPointer(comment), "confirmed_at": time.Now(),
		}).Error
	})
	if err != nil {
		return nil, err
	}
	var report model.WeeklyReport
	if err := service.db.First(&report, reportID).Error; err != nil {
		return nil, fmt.Errorf("reload weekly report: %w", err)
	}
	return &report, nil
}

func (service *InternshipService) GetCompanyEvaluation(supervisorID, caseID uint) (*model.CompanyEvaluation, error) {
	if _, err := service.findAssignedActiveCase(service.db, supervisorID, caseID, false); err != nil {
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
		var confirmedReportCount int64
		if err := tx.Model(&model.WeeklyReport{}).Where("internship_case_id = ?", caseID).Count(&reportCount).Error; err != nil {
			return fmt.Errorf("count weekly reports for evaluation: %w", err)
		}
		if err := tx.Model(&model.WeeklyReport{}).
			Where("internship_case_id = ? AND is_confirmed = ?", caseID, true).
			Count(&confirmedReportCount).Error; err != nil {
			return fmt.Errorf("count confirmed weekly reports for evaluation: %w", err)
		}
		if reportCount != 8 || confirmedReportCount != 8 {
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

func (service *InternshipService) EnsureStudentActiveCase(studentID uint) error {
	_, err := service.findStudentActiveCase(service.db, studentID, false)
	return err
}

func (service *InternshipService) AttachFinalReport(studentID uint, file *model.File) (*model.File, *model.File, error) {
	var oldFile *model.File
	err := service.db.Transaction(func(tx *gorm.DB) error {
		internshipCase, err := service.findStudentActiveCase(tx, studentID, true)
		if err != nil {
			return err
		}
		if err := tx.Create(file).Error; err != nil {
			return fmt.Errorf("save final report metadata: %w", err)
		}
		if internshipCase.FinalReportFileID != nil {
			var previous model.File
			if err := tx.First(&previous, *internshipCase.FinalReportFileID).Error; err == nil {
				oldFile = &previous
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("get previous final report: %w", err)
			}
		}
		if err := tx.Model(internshipCase).Update("final_report_file_id", file.ID).Error; err != nil {
			return fmt.Errorf("attach final report: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return file, oldFile, nil
}

func (service *InternshipService) DeleteFileMetadata(fileID uint) error {
	if fileID == 0 {
		return nil
	}
	return service.db.Delete(&model.File{}, fileID).Error
}

func (service *InternshipService) GetAccessibleFile(userID uint, role model.Role, fileID uint) (*model.File, error) {
	var file model.File
	if err := service.db.First(&file, fileID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrFileNotFound
	} else if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}
	query := service.db.Model(&model.InternshipCase{}).Where("final_report_file_id = ?", fileID)
	switch role {
	case model.RoleStudent:
		query = query.Where("student_id = ?", userID)
	case model.RoleProfessor:
		query = query.Where("professor_id = ?", userID)
	case model.RoleCompanySupervisor:
		query = query.Where("company_supervisor_id = ?", userID)
	case model.RoleUniversitySupervisor:
		// University supervisors already have system-wide read access to internship cases.
	default:
		return nil, ErrCaseAccessDenied
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, fmt.Errorf("authorize file: %w", err)
	}
	if count == 0 {
		return nil, ErrCaseAccessDenied
	}
	return &file, nil
}

func (service *InternshipService) listWeeklyReports(caseID uint) ([]model.WeeklyReport, error) {
	var reports []model.WeeklyReport
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
	query := db.Where("id = ? AND company_supervisor_id = ?", caseID, supervisorID)
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
