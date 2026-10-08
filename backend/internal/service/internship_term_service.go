package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"internship-management-system/backend/internal/model"
)

var (
	ErrTermNotFound         = errors.New("internship term not found")
	ErrInvalidTerm          = errors.New("invalid internship term")
	ErrDuplicateTerm        = errors.New("academic internship term already exists")
	ErrTermAlreadyOpen      = errors.New("another internship term is open")
	ErrTermClosed           = errors.New("internship term is already closed")
	ErrTermAccessDenied     = errors.New("internship term management access denied")
	ErrNoOpenInternshipTerm = errors.New("no internship term is open")
)

const TermClosureCancellationComment = "لغو خودکار به دلیل بسته شدن ترم کارآموزی"

type InternshipTermService struct{ db *gorm.DB }

func NewInternshipTermService(db *gorm.DB) *InternshipTermService {
	return &InternshipTermService{db: db}
}

func requireTermSupervisor(db *gorm.DB, actorID uint) error {
	var user model.User
	err := db.Select("id", "role").First(&user, actorID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && user.Role != model.RoleUniversitySupervisor) {
		return ErrTermAccessDenied
	}
	return err
}

func (service *InternshipTermService) List(actorID uint) ([]model.InternshipTerm, error) {
	if err := requireTermSupervisor(service.db, actorID); err != nil {
		return nil, err
	}
	terms := []model.InternshipTerm{}
	err := service.db.Order("opened_at DESC, id DESC").Find(&terms).Error
	return terms, err
}

// Current exposes only term metadata to students, never other students' cases.
func (service *InternshipTermService) Current() (*model.InternshipTerm, error) {
	var term model.InternshipTerm
	err := service.db.Where("status = ?", model.InternshipTermStatusOpen).First(&term).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &term, err
}

func (service *InternshipTermService) Detail(actorID, termID uint) (*model.InternshipTerm, []model.InternshipCase, error) {
	if err := requireTermSupervisor(service.db, actorID); err != nil {
		return nil, nil, err
	}
	var term model.InternshipTerm
	cases := []model.InternshipCase{}
	err := service.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&term, termID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTermNotFound
		}
		if err != nil {
			return err
		}
		// Includes drafts without changing the regular university review list.
		return NewInternshipService(tx).caseQuery(tx).Where("term_id = ?", termID).
			Order("created_at DESC, id DESC").Find(&cases).Error
	})
	return &term, cases, err
}

func (service *InternshipTermService) Create(actorID uint, academicYear int, termType model.InternshipTermType) (*model.InternshipTerm, error) {
	if err := requireTermSupervisor(service.db, actorID); err != nil {
		return nil, err
	}
	if academicYear <= 0 || academicYear > 9999 || !termType.Valid() {
		return nil, ErrInvalidTerm
	}
	var term model.InternshipTerm
	err := service.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.InternshipTerm{}).Where("academic_year = ? AND term_type = ?", academicYear, termType).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrDuplicateTerm
		}
		if err := tx.Model(&model.InternshipTerm{}).Where("status = ?", model.InternshipTermStatusOpen).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrTermAlreadyOpen
		}
		term = model.InternshipTerm{AcademicYear: academicYear, TermType: termType, Status: model.InternshipTermStatusOpen, OpenedAt: time.Now().UTC(), CreatedBy: actorID}
		return tx.Create(&term).Error
	})
	// The database indexes also arbitrate concurrent creation attempts.
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" {
		switch pgError.ConstraintName {
		case "idx_internship_term_identity":
			return nil, ErrDuplicateTerm
		case "idx_internship_term_one_open":
			return nil, ErrTermAlreadyOpen
		}
	}
	if err != nil {
		return nil, fmt.Errorf("create internship term: %w", err)
	}
	return &term, nil
}

func (service *InternshipTermService) Close(actorID, termID uint) (*model.InternshipTerm, error) {
	if err := requireTermSupervisor(service.db, actorID); err != nil {
		return nil, err
	}
	var term model.InternshipTerm
	err := service.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&term, termID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTermNotFound
		}
		if err != nil {
			return err
		}
		if term.Status != model.InternshipTermStatusOpen {
			return ErrTermClosed
		}
		now := time.Now().UTC()
		// UPDATE locks affected cases and rechecks the terminal predicate after
		// concurrent lifecycle changes; completed cases and their audit stay intact.
		// RETURNING identifies only cases actually cancelled by this transaction.
		var cancelledCases []model.InternshipCase
		if err := tx.Model(&cancelledCases).Clauses(clause.Returning{Columns: []clause.Column{{Name: "student_id"}}}).
			Where("term_id = ? AND status NOT IN ?", termID, terminalCaseStatuses()).
			Updates(map[string]any{"status": model.InternshipCaseStatusCancelled,
				"cancellation_comment": TermClosureCancellationComment, "cancelled_at": now}).Error; err != nil {
			return fmt.Errorf("cancel unfinished term cases: %w", err)
		}
		// ClosedBy records the actor for the term and its automatic cancellations.
		if err := tx.Model(&term).Updates(map[string]any{"status": model.InternshipTermStatusClosed,
			"closed_at": now, "closed_by": actorID}).Error; err != nil {
			return fmt.Errorf("close internship term: %w", err)
		}
		for _, item := range cancelledCases {
			if err := NewNotificationService(tx).CreateStudentNotification(item.StudentID,
				"لغو پرونده با بسته شدن ترم", "پرونده کارآموزی شما به دلیل بسته شدن ترم لغو شد. سوابق پرونده همچنان قابل مشاهده است.", "/student/application"); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &term, nil
}
