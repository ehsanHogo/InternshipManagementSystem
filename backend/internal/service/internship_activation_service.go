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
	ErrCaseNotReadyToStart           = errors.New("internship case is not ready to start")
	ErrCaseActivationIntegrityFailed = errors.New("internship case activation integrity failed")
)

// ActivateUniversityCase manually starts an approved case. The stored start
// date's relationship to today has no effect on activation.
func (service *InternshipService) ActivateUniversityCase(caseID uint) (*model.InternshipCase, error) {
	var activated *model.InternshipCase
	err := service.db.Transaction(func(tx *gorm.DB) error {
		var item model.InternshipCase
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, caseID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCaseNotFound
		}
		if err != nil {
			return fmt.Errorf("lock internship case for activation: %w", err)
		}
		if item.Status != model.InternshipCaseStatusReadyToStart {
			return ErrCaseNotReadyToStart
		}
		if err := validateFinalPlacement(tx, &item); err != nil {
			if errors.Is(err, ErrPlacementDetailsIncomplete) || errors.Is(err, ErrPlacementRelationshipInvalid) {
				return ErrCaseActivationIntegrityFailed
			}
			return err
		}
		// UpdateColumns deliberately preserves every other field and timestamp.
		result := tx.Model(&model.InternshipCase{}).
			Where("id = ? AND status = ?", caseID, model.InternshipCaseStatusReadyToStart).
			UpdateColumns(map[string]any{"status": model.InternshipCaseStatusActive, "activated_at": time.Now().UTC()})
		if result.Error != nil {
			return fmt.Errorf("activate internship case: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrCaseNotReadyToStart
		}
		var updated model.InternshipCase
		if err := service.caseQuery(tx).First(&updated, caseID).Error; err != nil {
			return fmt.Errorf("reload activated internship case: %w", err)
		}
		activated = &updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return activated, nil
}
