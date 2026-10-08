package service

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"internship-management-system/backend/internal/model"
)

var (
	ErrCaseNotAssignedToCompany     = fmt.Errorf("%w: case is not assigned to company", ErrCaseAccessDenied)
	ErrCaseNotPendingCompanyDetails = errors.New("internship case is not pending company details")
	ErrInvalidCompanyPlacement      = errors.New("invalid selected placement or introduction letter")
	ErrInternshipSubjectRequired    = errors.New("internship subject is required")
	ErrStartDateRequired            = errors.New("start date is required")
	ErrWorkplaceAddressRequired     = errors.New("workplace address is required")
	ErrWorkplacePhoneRequired       = errors.New("workplace phone is required")
	ErrInvalidStartDate             = errors.New("invalid start date")
	ErrPlacementDetailsTooLong      = errors.New("placement details exceed column limits")
)

type PlacementDetailsInput struct {
	InternshipSubject string
	StartDate         time.Time
	WorkplaceAddress  string
	WorkplacePhone    string
}

// companyCaseQuery binds the supervisor snapshot AND the selected placement to
// the supervisor's current company. Other accepted preferences grant no access.
func (service *InternshipService) companyCaseQuery(db *gorm.DB, supervisorID uint) *gorm.DB {
	return service.caseQuery(db).Where("internship_cases.company_supervisor_id = ?", supervisorID).
		Where(`EXISTS (
			SELECT 1 FROM internship_preferences p
			JOIN opportunity_applications a ON a.id = p.opportunity_application_id
			JOIN internship_opportunities o ON o.id = a.opportunity_id
			JOIN users u ON u.id = internship_cases.company_supervisor_id
			WHERE p.id = internship_cases.selected_preference_id
			AND p.internship_case_id = internship_cases.id
			AND a.student_id = internship_cases.student_id AND a.status = ?
			AND u.role = ? AND u.company_id = o.company_id
		)`, model.ApplicationStatusAccepted, model.RoleCompanySupervisor)
}

func (service *InternshipService) ListCompanyCases(supervisorID uint) ([]model.InternshipCase, error) {
	return service.listCompanyCases(supervisorID, companyVisibleStatuses())
}

func (service *InternshipService) ListPendingCompanyDetailsCases(supervisorID uint) ([]model.InternshipCase, error) {
	return service.listCompanyCases(supervisorID, []model.InternshipCaseStatus{model.InternshipCaseStatusPendingCompanyDetails})
}

func (service *InternshipService) listCompanyCases(supervisorID uint, statuses []model.InternshipCaseStatus) ([]model.InternshipCase, error) {
	if err := requireCompanyCaseAccess(service.db, supervisorID); err != nil {
		return nil, err
	}
	var cases []model.InternshipCase
	if err := service.companyCaseQuery(service.db, supervisorID).
		Where("internship_cases.status <> ? OR (internship_cases.term_id IS NOT NULL AND internship_cases.cancellation_comment = ?)", model.InternshipCaseStatusCancelled, TermClosureCancellationComment).
		Where("internship_cases.status IN ?", statuses).Order("updated_at DESC").Find(&cases).Error; err != nil {
		return nil, fmt.Errorf("list company internship cases: %w", err)
	}
	return cases, nil
}

func (service *InternshipService) GetCompanyCase(supervisorID, caseID uint) (*model.InternshipCase, error) {
	var internshipCase model.InternshipCase
	err := service.companyCaseQuery(service.db, supervisorID).
		Where("internship_cases.status <> ? OR (internship_cases.term_id IS NOT NULL AND internship_cases.cancellation_comment = ?)", model.InternshipCaseStatusCancelled, TermClosureCancellationComment).
		Where("internship_cases.status IN ?", companyVisibleStatuses()).First(&internshipCase, caseID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCaseNotAssignedToCompany
	}
	if err != nil {
		return nil, fmt.Errorf("get company internship case: %w", err)
	}
	if !internshipCase.Status.Terminal() {
		if err := requireCompanyCaseAccess(service.db, supervisorID); err != nil {
			return nil, err
		}
	}
	return &internshipCase, nil
}

// SubmitPlacementDetails is the only company placement action. Recruitment has
// already accepted the student. Locking serializes submissions and future edits.
func (service *InternshipService) SubmitPlacementDetails(supervisorID, caseID uint, input PlacementDetailsInput) (*model.InternshipCase, error) {
	if err := requireCompanyCaseAccess(service.db, supervisorID); err != nil {
		return nil, err
	}
	var resultCase *model.InternshipCase
	err := service.db.Transaction(func(tx *gorm.DB) error {
		var internshipCase model.InternshipCase
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&internshipCase, caseID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCaseNotFound
		}
		if err != nil {
			return fmt.Errorf("lock company placement case: %w", err)
		}
		if internshipCase.CompanySupervisorID == nil || *internshipCase.CompanySupervisorID != supervisorID {
			return ErrCaseNotAssignedToCompany
		}
		var supervisor model.User
		if err := tx.First(&supervisor, supervisorID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCaseNotAssignedToCompany
			}
			return fmt.Errorf("load placement supervisor: %w", err)
		}
		if supervisor.Role != model.RoleCompanySupervisor || supervisor.CompanyID == nil {
			return ErrCaseNotAssignedToCompany
		}
		if internshipCase.SelectedPreferenceID == nil {
			return ErrInvalidCompanyPlacement
		}
		var preference model.InternshipPreference
		err = tx.Preload("OpportunityApplication.Opportunity.Company").First(&preference, *internshipCase.SelectedPreferenceID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalidCompanyPlacement
		}
		if err != nil {
			return fmt.Errorf("load selected company placement: %w", err)
		}
		application := preference.OpportunityApplication
		opportunity := application.Opportunity
		if preference.InternshipCaseID != internshipCase.ID || opportunity.CompanyID != *supervisor.CompanyID {
			return ErrCaseNotAssignedToCompany
		}
		if internshipCase.Status != model.InternshipCaseStatusPendingCompanyDetails {
			return ErrCaseNotPendingCompanyDetails
		}
		if application.ID == 0 || application.StudentID != internshipCase.StudentID || application.Status != model.ApplicationStatusAccepted ||
			opportunity.ID == 0 || opportunity.Company.ID == 0 || internshipCase.LetterNumber == nil ||
			strings.TrimSpace(*internshipCase.LetterNumber) == "" || internshipCase.LetterDate == nil || internshipCase.LetterDate.IsZero() {
			return ErrInvalidCompanyPlacement
		}
		input.InternshipSubject = strings.TrimSpace(input.InternshipSubject)
		input.WorkplaceAddress = strings.TrimSpace(input.WorkplaceAddress)
		input.WorkplacePhone = strings.TrimSpace(input.WorkplacePhone)
		if input.InternshipSubject == "" {
			return ErrInternshipSubjectRequired
		}
		if input.StartDate.IsZero() {
			return ErrStartDateRequired
		}
		if input.StartDate.Year() < 1 || input.StartDate.Year() > 9999 {
			return ErrInvalidStartDate
		}
		if input.WorkplaceAddress == "" {
			return ErrWorkplaceAddressRequired
		}
		if input.WorkplacePhone == "" {
			return ErrWorkplacePhoneRequired
		}
		if utf8.RuneCountInString(input.InternshipSubject) > 500 || utf8.RuneCountInString(input.WorkplaceAddress) > 1000 || utf8.RuneCountInString(input.WorkplacePhone) > 50 {
			return ErrPlacementDetailsTooLong
		}
		startDate := time.Date(input.StartDate.Year(), input.StartDate.Month(), input.StartDate.Day(), 0, 0, 0, 0, time.UTC)
		result := tx.Model(&model.InternshipCase{}).Where("id = ? AND status = ?", caseID, model.InternshipCaseStatusPendingCompanyDetails).
			Updates(map[string]any{
				"internship_subject": input.InternshipSubject, "start_date": startDate,
				"workplace_address": input.WorkplaceAddress, "workplace_phone": input.WorkplacePhone,
				"company_details_revision_comment": nil, "status": model.InternshipCaseStatusPendingFinalApproval,
			})
		if result.Error != nil {
			return fmt.Errorf("submit company placement details: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrCaseNotPendingCompanyDetails
		}
		// Build the response inside the transaction so a failed reload rolls back too.
		var updated model.InternshipCase
		if err := service.caseQuery(tx).First(&updated, caseID).Error; err != nil {
			return fmt.Errorf("reload submitted placement: %w", err)
		}
		resultCase = &updated
		return NewNotificationService(tx).NotifyUniversityPlacementReview()
	})
	return resultCase, err
}

// Historical reads retain the original placement and supervisor ownership checks.
func (service *InternshipService) ListCompanyHistoricalCases(supervisorID uint) ([]model.InternshipCase, error) {
	cases := []model.InternshipCase{}
	err := service.companyCaseQuery(service.db, supervisorID).
		Where("internship_cases.status IN ?", []model.InternshipCaseStatus{model.InternshipCaseStatusPassed, model.InternshipCaseStatusFailed, model.InternshipCaseStatusCancelled}).
		Order("updated_at DESC").Find(&cases).Error
	return cases, err
}
func (service *InternshipService) GetCompanyHistoricalCase(supervisorID, caseID uint) (*model.InternshipCase, error) {
	var item model.InternshipCase
	err := service.companyCaseQuery(service.db, supervisorID).
		Where("internship_cases.status IN ?", []model.InternshipCaseStatus{model.InternshipCaseStatusPassed, model.InternshipCaseStatusFailed, model.InternshipCaseStatusCancelled}).First(&item, caseID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCaseNotAssignedToCompany
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}
