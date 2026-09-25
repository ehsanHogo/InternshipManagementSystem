package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"internship-management-system/backend/internal/model"
)

var (
	ErrApplicationAlreadyExists        = errors.New("opportunity application already exists")
	ErrOpportunityNotOpen              = errors.New("opportunity is not open")
	ErrApplicationNotPending           = errors.New("opportunity application is not pending")
	ErrResumeRequired                  = errors.New("resume is required")
	ErrInvalidResumeFile               = errors.New("invalid resume file")
	ErrResumeTooLarge                  = errors.New("resume is too large")
	ErrInternshipCaseAlreadyInProgress = errors.New("internship case already in progress")
	ErrInternshipAlreadyCompleted      = errors.New("internship already completed")
	ErrOpportunityApplicationNotFound  = errors.New("opportunity application not found")
)

const (
	ApplicationRestrictionAlreadyExists = "APPLICATION_ALREADY_EXISTS"
	ApplicationRestrictionNotOpen       = "OPPORTUNITY_NOT_OPEN"
	ApplicationRestrictionCaseActive    = "INTERNSHIP_CASE_ALREADY_IN_PROGRESS"
	ApplicationRestrictionCompleted     = "INTERNSHIP_ALREADY_COMPLETED"
)

type ApplicationEligibility struct {
	CanApply              bool
	RestrictionCode       string
	ExistingApplicationID *uint
	ExistingStatus        *model.ApplicationStatus
}

type OpportunityApplicationService struct {
	db *gorm.DB
}

func NewOpportunityApplicationService(db *gorm.DB) *OpportunityApplicationService {
	return &OpportunityApplicationService{db: db}
}

func (service *OpportunityApplicationService) CheckEligibility(studentID, opportunityID uint) (ApplicationEligibility, error) {
	eligibility, err := checkStudentOpportunityApplicationEligibility(service.db, studentID)
	if err != nil {
		return ApplicationEligibility{}, err
	}
	var opportunity model.InternshipOpportunity
	result := service.db.Select("id", "status").First(&opportunity, opportunityID)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return ApplicationEligibility{}, ErrOpportunityNotFound
	}
	if result.Error != nil {
		return ApplicationEligibility{}, fmt.Errorf("find opportunity for eligibility: %w", result.Error)
	}
	if opportunity.Status != model.OpportunityStatusOpen && eligibility.RestrictionCode == "" {
		eligibility.CanApply = false
		eligibility.RestrictionCode = ApplicationRestrictionNotOpen
	}
	var application model.OpportunityApplication
	result = service.db.Select("id", "status").Where("opportunity_id = ? AND student_id = ?", opportunityID, studentID).First(&application)
	if result.Error == nil {
		eligibility.CanApply = false
		eligibility.ExistingApplicationID = &application.ID
		eligibility.ExistingStatus = &application.Status
		if eligibility.RestrictionCode == "" {
			eligibility.RestrictionCode = ApplicationRestrictionAlreadyExists
		}
	} else if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return ApplicationEligibility{}, fmt.Errorf("find existing opportunity application: %w", result.Error)
	}
	return eligibility, nil
}

func (service *OpportunityApplicationService) EnsureCanApply(studentID, opportunityID uint) error {
	eligibility, err := service.CheckEligibility(studentID, opportunityID)
	if err != nil {
		return err
	}
	return eligibilityError(eligibility.RestrictionCode)
}

func (service *OpportunityApplicationService) Apply(studentID, opportunityID uint, resume *model.File) (*model.OpportunityApplication, error) {
	if resume == nil || resume.OriginalName == "" || resume.StoredName == "" || resume.Path == "" ||
		resume.MimeType != "application/pdf" || resume.SizeBytes <= 0 {
		return nil, ErrInvalidResumeFile
	}
	var applicationID uint
	err := service.db.Transaction(func(tx *gorm.DB) error {
		var student model.User
		result := tx.Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND role = ?", studentID, model.RoleStudent).First(&student)
		if result.Error != nil {
			return fmt.Errorf("lock application student: %w", result.Error)
		}
		eligibility, err := checkStudentOpportunityApplicationEligibility(tx, studentID)
		if err != nil {
			return err
		}
		if !eligibility.CanApply {
			return eligibilityError(eligibility.RestrictionCode)
		}
		var opportunity model.InternshipOpportunity
		result = tx.Select("id", "status").Clauses(clause.Locking{Strength: "UPDATE"}).First(&opportunity, opportunityID)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return ErrOpportunityNotFound
		}
		if result.Error != nil {
			return fmt.Errorf("lock opportunity for application: %w", result.Error)
		}
		if opportunity.Status != model.OpportunityStatusOpen {
			return ErrOpportunityNotOpen
		}
		var count int64
		if err := tx.Model(&model.OpportunityApplication{}).
			Where("opportunity_id = ? AND student_id = ?", opportunityID, studentID).Count(&count).Error; err != nil {
			return fmt.Errorf("check duplicate opportunity application: %w", err)
		}
		if count > 0 {
			return ErrApplicationAlreadyExists
		}
		resume.UploadedBy = studentID
		if err := tx.Create(resume).Error; err != nil {
			return fmt.Errorf("save resume metadata: %w", err)
		}
		now := time.Now()
		application := model.OpportunityApplication{
			OpportunityID: opportunityID, StudentID: studentID, ResumeFileID: resume.ID,
			Status: model.ApplicationStatusPending, AppliedAt: now,
		}
		if err := tx.Create(&application).Error; err != nil {
			if isUniqueViolation(err) {
				return ErrApplicationAlreadyExists
			}
			return fmt.Errorf("create opportunity application: %w", err)
		}
		applicationID = application.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return service.getByID(applicationID)
}

func (service *OpportunityApplicationService) ListStudent(studentID uint) ([]model.OpportunityApplication, error) {
	var applications []model.OpportunityApplication
	if err := applicationQuery(service.db).Where("student_id = ?", studentID).
		Order("applied_at DESC").Find(&applications).Error; err != nil {
		return nil, fmt.Errorf("list student opportunity applications: %w", err)
	}
	return applications, nil
}

func (service *OpportunityApplicationService) GetStudent(studentID, applicationID uint) (*model.OpportunityApplication, error) {
	var application model.OpportunityApplication
	result := applicationQuery(service.db).Where("opportunity_applications.id = ? AND student_id = ?", applicationID, studentID).First(&application)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, ErrOpportunityApplicationNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("get student opportunity application: %w", result.Error)
	}
	return &application, nil
}

func (service *OpportunityApplicationService) ListCompany(supervisorID, opportunityID uint) ([]model.OpportunityApplication, error) {
	companyID, err := service.supervisorCompanyID(supervisorID)
	if err != nil {
		return nil, err
	}
	var opportunityCount int64
	if err := service.db.Model(&model.InternshipOpportunity{}).
		Where("id = ? AND company_id = ?", opportunityID, companyID).Count(&opportunityCount).Error; err != nil {
		return nil, fmt.Errorf("authorize opportunity applicant list: %w", err)
	}
	if opportunityCount == 0 {
		return nil, ErrOpportunityNotFound
	}
	var applications []model.OpportunityApplication
	if err := applicationQuery(service.db).Where("opportunity_id = ?", opportunityID).
		Order("applied_at DESC").Find(&applications).Error; err != nil {
		return nil, fmt.Errorf("list company opportunity applications: %w", err)
	}
	return applications, nil
}

func (service *OpportunityApplicationService) GetCompany(supervisorID, applicationID uint) (*model.OpportunityApplication, error) {
	companyID, err := service.supervisorCompanyID(supervisorID)
	if err != nil {
		return nil, err
	}
	return service.getCompanyApplication(companyID, applicationID)
}

func (service *OpportunityApplicationService) Review(supervisorID, applicationID uint, status model.ApplicationStatus, comment *string) (*model.OpportunityApplication, error) {
	if status != model.ApplicationStatusAccepted && status != model.ApplicationStatusRejected {
		return nil, ErrApplicationNotPending
	}
	companyID, err := service.supervisorCompanyID(supervisorID)
	if err != nil {
		return nil, err
	}
	comment = normalizeCompanyComment(comment)
	err = service.db.Transaction(func(tx *gorm.DB) error {
		var application model.OpportunityApplication
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Joins("JOIN internship_opportunities ON internship_opportunities.id = opportunity_applications.opportunity_id").
			Where("opportunity_applications.id = ? AND internship_opportunities.company_id = ?", applicationID, companyID).
			First(&application)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return ErrOpportunityApplicationNotFound
		}
		if result.Error != nil {
			return fmt.Errorf("lock company opportunity application: %w", result.Error)
		}
		if application.Status != model.ApplicationStatusPending {
			return ErrApplicationNotPending
		}
		now := time.Now()
		result = tx.Model(&model.OpportunityApplication{}).
			Where("id = ? AND status = ?", applicationID, model.ApplicationStatusPending).
			Updates(map[string]any{"status": status, "company_comment": comment, "reviewed_at": now})
		if result.Error != nil {
			return fmt.Errorf("review opportunity application: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrApplicationNotPending
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return service.getCompanyApplication(companyID, applicationID)
}

func checkStudentOpportunityApplicationEligibility(db *gorm.DB, studentID uint) (ApplicationEligibility, error) {
	var completedCount int64
	if err := db.Model(&model.InternshipCase{}).
		Where("student_id = ? AND status = ?", studentID, model.InternshipCaseStatusCompleted).
		Count(&completedCount).Error; err != nil {
		return ApplicationEligibility{}, fmt.Errorf("check completed internship case: %w", err)
	}
	if completedCount > 0 {
		return ApplicationEligibility{CanApply: false, RestrictionCode: ApplicationRestrictionCompleted}, nil
	}
	blocking := []model.InternshipCaseStatus{
		model.InternshipCaseStatusPendingUniversityReview,
		model.InternshipCaseStatusPendingCompanyDetails,
		model.InternshipCaseStatusPendingFinalApproval,
		model.InternshipCaseStatusReadyToStart,
		model.InternshipCaseStatusActive,
	}
	var activeCount int64
	if err := db.Model(&model.InternshipCase{}).
		Where("student_id = ? AND status IN ?", studentID, blocking).Count(&activeCount).Error; err != nil {
		return ApplicationEligibility{}, fmt.Errorf("check in-progress internship case: %w", err)
	}
	if activeCount > 0 {
		return ApplicationEligibility{CanApply: false, RestrictionCode: ApplicationRestrictionCaseActive}, nil
	}
	return ApplicationEligibility{CanApply: true}, nil
}

func eligibilityError(code string) error {
	switch code {
	case "":
		return nil
	case ApplicationRestrictionAlreadyExists:
		return ErrApplicationAlreadyExists
	case ApplicationRestrictionNotOpen:
		return ErrOpportunityNotOpen
	case ApplicationRestrictionCaseActive:
		return ErrInternshipCaseAlreadyInProgress
	case ApplicationRestrictionCompleted:
		return ErrInternshipAlreadyCompleted
	default:
		return ErrInvalidApplication
	}
}

func applicationQuery(db *gorm.DB) *gorm.DB {
	return db.Preload("Student").Preload("ResumeFile").
		Preload("Opportunity").Preload("Opportunity.Company")
}

func (service *OpportunityApplicationService) getByID(applicationID uint) (*model.OpportunityApplication, error) {
	var application model.OpportunityApplication
	if err := applicationQuery(service.db).First(&application, applicationID).Error; err != nil {
		return nil, fmt.Errorf("reload opportunity application: %w", err)
	}
	return &application, nil
}

func (service *OpportunityApplicationService) getCompanyApplication(companyID, applicationID uint) (*model.OpportunityApplication, error) {
	var application model.OpportunityApplication
	result := applicationQuery(service.db).
		Joins("JOIN internship_opportunities AS owned_opportunities ON owned_opportunities.id = opportunity_applications.opportunity_id").
		Where("opportunity_applications.id = ? AND owned_opportunities.company_id = ?", applicationID, companyID).
		First(&application)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, ErrOpportunityApplicationNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("get company opportunity application: %w", result.Error)
	}
	return &application, nil
}

func (service *OpportunityApplicationService) supervisorCompanyID(supervisorID uint) (uint, error) {
	var supervisor model.User
	result := service.db.Select("id", "company_id").Where("id = ? AND role = ?", supervisorID, model.RoleCompanySupervisor).First(&supervisor)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return 0, ErrOpportunityCompanyMissing
	}
	if result.Error != nil {
		return 0, fmt.Errorf("find application supervisor: %w", result.Error)
	}
	if supervisor.CompanyID == nil || *supervisor.CompanyID == 0 {
		return 0, ErrOpportunityCompanyMissing
	}
	return *supervisor.CompanyID, nil
}

func normalizeCompanyComment(comment *string) *string {
	if comment == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*comment)
	if trimmed == "" {
		return nil
	}
	if len([]rune(trimmed)) > 5000 {
		trimmed = string([]rune(trimmed)[:5000])
	}
	return &trimmed
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}
