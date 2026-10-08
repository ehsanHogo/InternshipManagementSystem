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
	ErrFinalReportState                   = errors.New("final report does not allow this action")
	ErrFinalReportCommentRequired         = errors.New("final report revision comment is required")
	ErrInvalidFinalReportFile             = errors.New("invalid final report PDF")
	ErrFinalReportTooLarge                = errors.New("final report exceeds size limit")
	ErrFinalReportWeeklyReportsIncomplete = errors.New("all 8 weekly reports must be approved by both reviewers before the initial final report upload")
)

// CanUploadFinalReport uses the preloaded case data for response readiness.
// The upload path enforces the same rule again under the case lock.
func CanUploadFinalReport(item *model.InternshipCase) bool {
	return finalReportUploadError(item) == nil
}

func finalReportUploadError(item *model.InternshipCase) error {
	if item.Status != model.InternshipCaseStatusActive {
		return ErrInvalidCaseStatus
	}
	if item.FinalReport != nil {
		if item.FinalReport.Status != model.FinalReportRevisionRequested {
			return ErrFinalReportState
		}
		// Corrections retain the existing lifecycle even with inconsistent historical weeks.
		return nil
	}
	if !model.WeeklyReportsReadyForFinalEvaluation(item.WeeklyReports) {
		return ErrFinalReportWeeklyReportsIncomplete
	}
	return nil
}

func (service *InternshipService) GetStudentFinalReport(studentID uint) (*model.FinalReport, error) {
	item, err := service.GetCurrentCase(studentID)
	if err != nil {
		return nil, err
	}
	return item.FinalReport, nil
}

func (service *InternshipService) GetProfessorFinalReport(professorID, caseID uint) (*model.FinalReport, error) {
	item, err := service.GetProfessorCase(professorID, caseID)
	if err != nil {
		return nil, err
	}
	return item.FinalReport, nil
}

func (service *InternshipService) EnsureCanUploadFinalReport(studentID uint) error {
	item, err := service.findStudentActiveCase(service.db, studentID, false)
	if err != nil {
		return err
	}
	_, err = uploadableFinalReport(service.db, item)
	return err
}

func uploadableFinalReport(db *gorm.DB, item *model.InternshipCase) (*model.FinalReport, error) {
	var report model.FinalReport
	err := db.Where("internship_case_id = ?", item.ID).First(&report).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := db.Where("internship_case_id = ?", item.ID).Find(&item.WeeklyReports).Error; err != nil {
			return nil, fmt.Errorf("load weekly reports for final report upload: %w", err)
		}
		return nil, finalReportUploadError(item)
	}
	if err != nil {
		return nil, fmt.Errorf("get final report for upload: %w", err)
	}
	item.FinalReport = &report
	return &report, finalReportUploadError(item)
}

// The case lock serializes first uploads, corrections, reviews, and completion.
// All metadata changes, including deletion of superseded metadata, commit together.
// The handler removes the new disk file on failure and the old disk file after success.
func (service *InternshipService) AttachFinalReport(studentID uint, file *model.File) (*model.FinalReport, *model.File, error) {
	if file == nil || file.MimeType != "application/pdf" || file.SizeBytes <= 0 {
		return nil, nil, ErrInvalidFinalReportFile
	}
	var report *model.FinalReport
	var oldFile *model.File
	err := service.db.Transaction(func(tx *gorm.DB) error {
		item, err := service.findStudentActiveCase(tx, studentID, true)
		if err != nil {
			return err
		}
		report, err = uploadableFinalReport(tx, item)
		if err != nil {
			return err
		}
		if report != nil {
			var previous model.File
			if err := tx.First(&previous, report.CurrentFileID).Error; err != nil {
				return fmt.Errorf("get previous final PDF: %w", err)
			}
			oldFile = &previous
		}
		file.UploadedBy = studentID
		if err := tx.Create(file).Error; err != nil {
			return fmt.Errorf("save final PDF metadata: %w", err)
		}
		now := time.Now()
		if report == nil {
			report = &model.FinalReport{InternshipCaseID: item.ID, CurrentFileID: file.ID, Status: model.FinalReportSubmitted, SubmittedAt: now}
			if err := tx.Create(report).Error; err != nil {
				return fmt.Errorf("create final report: %w", err)
			}
		} else {
			if err := tx.Model(report).Updates(map[string]any{
				"current_file_id": file.ID, "status": model.FinalReportSubmitted, "submitted_at": now, "reviewed_at": nil,
			}).Error; err != nil {
				return fmt.Errorf("resubmit final report: %w", err)
			}
			report.CurrentFileID, report.Status, report.SubmittedAt, report.ReviewedAt = file.ID, model.FinalReportSubmitted, now, nil
			if err := tx.Delete(oldFile).Error; err != nil {
				return fmt.Errorf("delete superseded final PDF metadata: %w", err)
			}
		}
		report.CurrentFile = *file
		return NewNotificationService(tx).NotifyProfessorActionRequired(item.ProfessorID)
	})
	if err != nil {
		return nil, nil, err
	}
	return report, oldFile, nil
}

func (service *InternshipService) ReviewFinalReport(professorID, caseID uint, status model.FinalReportStatus, comment *string) (*model.FinalReport, error) {
	if status != model.FinalReportApproved && status != model.FinalReportRevisionRequested {
		return nil, ErrFinalReportState
	}
	comment = trimmedPointer(comment)
	if status == model.FinalReportRevisionRequested && comment == nil {
		return nil, ErrFinalReportCommentRequired
	}
	var report model.FinalReport
	err := service.db.Transaction(func(tx *gorm.DB) error {
		var item model.InternshipCase
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND professor_id = ?", caseID, professorID).First(&item).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCaseAccessDenied
		}
		if err != nil {
			return fmt.Errorf("lock final report case: %w", err)
		}
		if item.Status != model.InternshipCaseStatusActive {
			return ErrProfessorCaseNotActive
		}
		err = tx.Where("internship_case_id = ?", caseID).First(&report).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFinalReportState
		}
		if err != nil {
			return fmt.Errorf("get final report for review: %w", err)
		}
		if report.Status != model.FinalReportSubmitted {
			return ErrFinalReportState
		}
		now := time.Now()
		if err := tx.Model(&report).Updates(map[string]any{"status": status, "review_comment": comment, "reviewed_at": now}).Error; err != nil {
			return fmt.Errorf("review final report: %w", err)
		}
		if err := tx.Preload("CurrentFile").First(&report, report.ID).Error; err != nil {
			return err
		}
		message := "گزارش نهایی شما نیازمند اصلاح است."
		if status == model.FinalReportApproved {
			message = "گزارش نهایی شما تأیید شد."
		}
		return NewNotificationService(tx).CreateStudentNotification(item.StudentID, "نتیجه بررسی گزارش نهایی", message, "/student/final-report")
	})
	if err != nil {
		return nil, err
	}
	return &report, nil
}
