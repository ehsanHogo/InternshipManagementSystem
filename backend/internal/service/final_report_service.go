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
	ErrFinalReportState           = errors.New("final report does not allow this action")
	ErrFinalReportCommentRequired = errors.New("final report revision comment is required")
	ErrInvalidFinalReportFile     = errors.New("invalid final report PDF")
	ErrFinalReportTooLarge        = errors.New("final report exceeds size limit")
)

func (service *InternshipService) GetStudentFinalReport(studentID uint) (*model.FinalReport, error) {
	item, err := service.GetCurrentCase(studentID)
	// Keep the existing final-report read endpoint usable after finalization.
	// History detail selects a specific case once a newer draft exists.
	if errors.Is(err, ErrCaseNotFound) {
		item, err = service.findOwnedCase(service.db, studentID)
		if err == nil {
			item, err = service.getCaseByID(item.ID)
		}
	}
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
	_, err = uploadableFinalReport(service.db, item.ID)
	return err
}

func uploadableFinalReport(db *gorm.DB, caseID uint) (*model.FinalReport, error) {
	var report model.FinalReport
	err := db.Where("internship_case_id = ?", caseID).First(&report).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get final report for upload: %w", err)
	}
	if report.Status != model.FinalReportRevisionRequested {
		return nil, ErrFinalReportState
	}
	return &report, nil
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
		report, err = uploadableFinalReport(tx, item.ID)
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
		return nil
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
		return tx.Preload("CurrentFile").First(&report, report.ID).Error
	})
	if err != nil {
		return nil, err
	}
	return &report, nil
}
