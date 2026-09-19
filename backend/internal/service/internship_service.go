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
	ErrCaseNotFound       = errors.New("internship case not found")
	ErrAssignmentNotFound = errors.New("professor assignment not found")
	ErrCaseNotEditable    = errors.New("internship case is not editable")
	ErrPreferenceNotFound = errors.New("internship preference not found")
	ErrPreferenceLimit    = errors.New("preference limit reached")
	ErrDuplicatePriority  = errors.New("preference priority already exists")
	ErrInvalidPreference  = errors.New("invalid internship preference")
	ErrInvalidApplication = errors.New("internship application is incomplete")
	ErrInvalidCaseStatus  = errors.New("invalid internship case status")
	ErrInvalidTransition  = errors.New("invalid internship case transition")
	ErrCompanySupervisor  = errors.New("invalid company supervisor")
	ErrCaseAccessDenied   = errors.New("internship case access denied")
)

type InternshipService struct {
	db *gorm.DB
}

type PreferenceInput struct {
	Priority               int
	CompanyID              *uint
	ProposedCompanyName    *string
	ProposedWebsite        *string
	ProposedPhone          *string
	ProposedEmail          *string
	ProposedSupervisorName *string
	City                   string
	WorkField              string
}

func NewInternshipService(db *gorm.DB) *InternshipService {
	return &InternshipService{db: db}
}

func (service *InternshipService) ListApprovedCompanies() ([]model.Company, error) {
	var companies []model.Company
	if err := service.db.Where("is_approved = ?", true).Order("name ASC").Find(&companies).Error; err != nil {
		return nil, fmt.Errorf("list approved companies: %w", err)
	}
	return companies, nil
}

func (service *InternshipService) GetCurrentCase(studentID uint) (*model.InternshipCase, error) {
	var internshipCase model.InternshipCase
	err := service.caseQuery(service.db).
		Where("student_id = ? AND status IN ?", studentID, currentCaseStatuses()).
		Order("created_at DESC").
		First(&internshipCase).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCaseNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get current internship case: %w", err)
	}
	return &internshipCase, nil
}

func (service *InternshipService) CreateOrGetCase(studentID uint) (*model.InternshipCase, bool, error) {
	var caseID uint
	created := false
	err := service.db.Transaction(func(tx *gorm.DB) error {
		var assignment model.ProfessorAssignment
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("student_id = ?", studentID).
			First(&assignment).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAssignmentNotFound
		}
		if err != nil {
			return fmt.Errorf("get professor assignment: %w", err)
		}

		var existing model.InternshipCase
		err = tx.Where("student_id = ? AND status IN ?", studentID, currentCaseStatuses()).
			Order("created_at DESC").First(&existing).Error
		if err == nil {
			caseID = existing.ID
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("look up current internship case: %w", err)
		}

		internshipCase := model.InternshipCase{
			StudentID: studentID, ProfessorID: assignment.ProfessorID,
			Status: model.InternshipCaseStatusDraft,
		}
		if err := tx.Create(&internshipCase).Error; err != nil {
			return fmt.Errorf("create internship case: %w", err)
		}
		caseID = internshipCase.ID
		created = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}

	internshipCase, err := service.getCaseByID(caseID)
	return internshipCase, created, err
}

func (service *InternshipService) UpdateCase(studentID uint, passedCredits *int, mobile *string) (*model.InternshipCase, error) {
	updates := map[string]any{}
	if passedCredits != nil {
		if *passedCredits < 0 {
			return nil, ErrInvalidApplication
		}
		updates["passed_credits"] = *passedCredits
	}
	if mobile != nil {
		trimmed := strings.TrimSpace(*mobile)
		updates["mobile"] = nullableString(trimmed)
	}

	var caseID uint
	err := service.db.Transaction(func(tx *gorm.DB) error {
		internshipCase, err := service.findOwnedCaseForUpdate(tx, studentID)
		if err != nil {
			return err
		}
		if internshipCase.Status != model.InternshipCaseStatusDraft {
			return ErrCaseNotEditable
		}
		caseID = internshipCase.ID
		if len(updates) > 0 {
			if err := tx.Model(internshipCase).Updates(updates).Error; err != nil {
				return fmt.Errorf("update internship case: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return service.getCaseByID(caseID)
}

func (service *InternshipService) AddPreference(studentID uint, input PreferenceInput) (*model.InternshipPreference, error) {
	if err := service.validatePreferenceInput(input); err != nil {
		return nil, err
	}

	var preference model.InternshipPreference
	err := service.db.Transaction(func(tx *gorm.DB) error {
		internshipCase, err := service.findOwnedCaseForUpdate(tx, studentID)
		if err != nil {
			return err
		}
		if internshipCase.Status != model.InternshipCaseStatusDraft {
			return ErrCaseNotEditable
		}

		var count int64
		if err := tx.Model(&model.InternshipPreference{}).
			Where("internship_case_id = ?", internshipCase.ID).Count(&count).Error; err != nil {
			return fmt.Errorf("count internship preferences: %w", err)
		}
		if count >= model.MaxPreferenceCount {
			return ErrPreferenceLimit
		}
		if err := service.ensurePriorityAvailable(tx, internshipCase.ID, input.Priority, 0); err != nil {
			return err
		}
		if err := service.ensureApprovedCompany(tx, input.CompanyID); err != nil {
			return err
		}

		preference = preferenceFromInput(internshipCase.ID, input)
		if err := tx.Create(&preference).Error; err != nil {
			return fmt.Errorf("create internship preference: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return service.getPreference(preference.ID)
}

func (service *InternshipService) UpdatePreference(studentID, preferenceID uint, input PreferenceInput) (*model.InternshipPreference, error) {
	if err := service.validatePreferenceInput(input); err != nil {
		return nil, err
	}

	err := service.db.Transaction(func(tx *gorm.DB) error {
		var preference model.InternshipPreference
		err := tx.Joins("JOIN internship_cases ON internship_cases.id = internship_preferences.internship_case_id").
			Where("internship_preferences.id = ? AND internship_cases.student_id = ?", preferenceID, studentID).
			First(&preference).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPreferenceNotFound
		}
		if err != nil {
			return fmt.Errorf("get internship preference: %w", err)
		}

		internshipCase, err := service.findOwnedCaseForUpdate(tx, studentID)
		if err != nil {
			return err
		}
		if internshipCase.ID != preference.InternshipCaseID {
			return ErrPreferenceNotFound
		}
		if internshipCase.Status != model.InternshipCaseStatusDraft {
			return ErrCaseNotEditable
		}
		if err := service.ensurePriorityAvailable(tx, internshipCase.ID, input.Priority, preference.ID); err != nil {
			return err
		}
		if err := service.ensureApprovedCompany(tx, input.CompanyID); err != nil {
			return err
		}

		updated := preferenceFromInput(internshipCase.ID, input)
		updates := map[string]any{
			"priority": updated.Priority, "company_id": updated.CompanyID,
			"proposed_company_name": updated.ProposedCompanyName,
			"proposed_website":      updated.ProposedWebsite, "proposed_phone": updated.ProposedPhone,
			"proposed_email": updated.ProposedEmail, "proposed_supervisor_name": updated.ProposedSupervisorName,
			"city": updated.City, "work_field": updated.WorkField,
		}
		if err := tx.Model(&preference).Updates(updates).Error; err != nil {
			return fmt.Errorf("update internship preference: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return service.getPreference(preferenceID)
}

func (service *InternshipService) DeletePreference(studentID, preferenceID uint) error {
	return service.db.Transaction(func(tx *gorm.DB) error {
		var preference model.InternshipPreference
		err := tx.Joins("JOIN internship_cases ON internship_cases.id = internship_preferences.internship_case_id").
			Where("internship_preferences.id = ? AND internship_cases.student_id = ?", preferenceID, studentID).
			First(&preference).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPreferenceNotFound
		}
		if err != nil {
			return fmt.Errorf("get internship preference: %w", err)
		}

		internshipCase, err := service.findOwnedCaseForUpdate(tx, studentID)
		if err != nil {
			return err
		}
		if internshipCase.ID != preference.InternshipCaseID {
			return ErrPreferenceNotFound
		}
		if internshipCase.Status != model.InternshipCaseStatusDraft {
			return ErrCaseNotEditable
		}
		if err := tx.Delete(&preference).Error; err != nil {
			return fmt.Errorf("delete internship preference: %w", err)
		}
		return nil
	})
}

func (service *InternshipService) SubmitCase(studentID uint) (*model.InternshipCase, error) {
	var caseID uint
	err := service.db.Transaction(func(tx *gorm.DB) error {
		internshipCase, err := service.findOwnedCaseForUpdate(tx, studentID)
		if err != nil {
			return err
		}
		if internshipCase.Status != model.InternshipCaseStatusDraft {
			return ErrCaseNotEditable
		}

		var assignment model.ProfessorAssignment
		if err := tx.Where("student_id = ? AND professor_id = ?", studentID, internshipCase.ProfessorID).
			First(&assignment).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAssignmentNotFound
		} else if err != nil {
			return fmt.Errorf("validate professor assignment: %w", err)
		}

		var preferences []model.InternshipPreference
		if err := tx.Where("internship_case_id = ?", internshipCase.ID).Find(&preferences).Error; err != nil {
			return fmt.Errorf("list internship preferences: %w", err)
		}
		if internshipCase.PassedCredits == nil || internshipCase.Mobile == nil || strings.TrimSpace(*internshipCase.Mobile) == "" ||
			len(preferences) < model.MinPreferenceCount || len(preferences) > model.MaxPreferenceCount {
			return ErrInvalidApplication
		}
		priorities := make(map[int]bool, len(preferences))
		for _, preference := range preferences {
			if preference.Priority < 1 || preference.Priority > model.MaxPreferenceCount || priorities[preference.Priority] {
				return ErrInvalidApplication
			}
			priorities[preference.Priority] = true
			if preference.CompanyID == nil && (preference.ProposedCompanyName == nil || strings.TrimSpace(*preference.ProposedCompanyName) == "") {
				return ErrInvalidApplication
			}
		}

		now := time.Now()
		if err := tx.Model(internshipCase).Updates(map[string]any{
			"status": model.InternshipCaseStatusPendingUniversityApproval, "submitted_at": now,
		}).Error; err != nil {
			return fmt.Errorf("submit internship case: %w", err)
		}
		caseID = internshipCase.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return service.getCaseByID(caseID)
}

func (service *InternshipService) caseQuery(db *gorm.DB) *gorm.DB {
	return db.Preload("Student").Preload("Professor").
		Preload("Preferences", func(query *gorm.DB) *gorm.DB { return query.Order("priority ASC") }).
		Preload("Preferences.Company").Preload("SelectedPreference.Company").Preload("CompanySupervisor")
}

func (service *InternshipService) getCaseByID(caseID uint) (*model.InternshipCase, error) {
	var internshipCase model.InternshipCase
	if err := service.caseQuery(service.db).First(&internshipCase, caseID).Error; err != nil {
		return nil, fmt.Errorf("get internship case: %w", err)
	}
	return &internshipCase, nil
}

func (service *InternshipService) findOwnedCase(db *gorm.DB, studentID uint) (*model.InternshipCase, error) {
	var internshipCase model.InternshipCase
	err := db.Where("student_id = ? AND status IN ?", studentID, currentCaseStatuses()).
		Order("created_at DESC").First(&internshipCase).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCaseNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get owned internship case: %w", err)
	}
	return &internshipCase, nil
}

func (service *InternshipService) findOwnedCaseForUpdate(db *gorm.DB, studentID uint) (*model.InternshipCase, error) {
	var internshipCase model.InternshipCase
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("student_id = ? AND status IN ?", studentID, currentCaseStatuses()).
		Order("created_at DESC").First(&internshipCase).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCaseNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock owned internship case: %w", err)
	}
	return &internshipCase, nil
}

func (service *InternshipService) validatePreferenceInput(input PreferenceInput) error {
	if input.Priority < 1 || input.Priority > model.MaxPreferenceCount || strings.TrimSpace(input.City) == "" || strings.TrimSpace(input.WorkField) == "" {
		return ErrInvalidPreference
	}
	if input.CompanyID == nil {
		if input.ProposedCompanyName == nil || strings.TrimSpace(*input.ProposedCompanyName) == "" {
			return ErrInvalidPreference
		}
	} else if *input.CompanyID == 0 {
		return ErrInvalidPreference
	}
	return nil
}

func (service *InternshipService) ensureApprovedCompany(db *gorm.DB, companyID *uint) error {
	if companyID == nil {
		return nil
	}
	var count int64
	if err := db.Model(&model.Company{}).Where("id = ? AND is_approved = ?", *companyID, true).Count(&count).Error; err != nil {
		return fmt.Errorf("validate approved company: %w", err)
	}
	if count == 0 {
		return ErrInvalidPreference
	}
	return nil
}

func (service *InternshipService) ensurePriorityAvailable(db *gorm.DB, caseID uint, priority int, excludeID uint) error {
	query := db.Model(&model.InternshipPreference{}).
		Where("internship_case_id = ? AND priority = ?", caseID, priority)
	if excludeID != 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return fmt.Errorf("validate preference priority: %w", err)
	}
	if count > 0 {
		return ErrDuplicatePriority
	}
	return nil
}

func (service *InternshipService) getPreference(preferenceID uint) (*model.InternshipPreference, error) {
	var preference model.InternshipPreference
	if err := service.db.Preload("Company").First(&preference, preferenceID).Error; err != nil {
		return nil, fmt.Errorf("get internship preference: %w", err)
	}
	return &preference, nil
}

func preferenceFromInput(caseID uint, input PreferenceInput) model.InternshipPreference {
	preference := model.InternshipPreference{
		InternshipCaseID: caseID, Priority: input.Priority, CompanyID: input.CompanyID,
		City: strings.TrimSpace(input.City), WorkField: strings.TrimSpace(input.WorkField),
	}
	if input.CompanyID == nil {
		preference.ProposedCompanyName = trimmedPointer(input.ProposedCompanyName)
		preference.ProposedWebsite = trimmedPointer(input.ProposedWebsite)
		preference.ProposedPhone = trimmedPointer(input.ProposedPhone)
		preference.ProposedEmail = trimmedPointer(input.ProposedEmail)
		preference.ProposedSupervisorName = trimmedPointer(input.ProposedSupervisorName)
	}
	return preference
}

func trimmedPointer(value *string) *string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func currentCaseStatuses() []model.InternshipCaseStatus {
	return []model.InternshipCaseStatus{
		model.InternshipCaseStatusDraft,
		model.InternshipCaseStatusPendingUniversityApproval,
		model.InternshipCaseStatusPendingCompanyApproval,
		model.InternshipCaseStatusCompanyApproved,
		model.InternshipCaseStatusUniversityApproved,
		model.InternshipCaseStatusActive,
	}
}
