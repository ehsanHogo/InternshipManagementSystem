package service

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"internship-management-system/backend/internal/model"
)

var (
	ErrInvalidOpportunity        = errors.New("invalid opportunity")
	ErrOpportunityNotFound       = errors.New("opportunity not found")
	ErrOpportunityAlreadyClosed  = errors.New("opportunity already closed")
	ErrOpportunityCompanyMissing = errors.New("opportunity company missing")
)

type OpportunityInput struct {
	Title       string
	Description string
	WorkField   string
	Location    string
}

type OpportunityService struct {
	db *gorm.DB
}

func NewOpportunityService(db *gorm.DB) *OpportunityService {
	return &OpportunityService{db: db}
}

func (service *OpportunityService) Create(supervisorID uint, input OpportunityInput) (*model.InternshipOpportunity, error) {
	input, err := normalizeOpportunityInput(input)
	if err != nil {
		return nil, err
	}
	companyID, err := service.supervisorCompanyID(supervisorID)
	if err != nil {
		return nil, err
	}
	opportunity := model.InternshipOpportunity{
		CompanyID: companyID, CreatedBy: supervisorID, Title: input.Title,
		Description: input.Description, WorkField: input.WorkField,
		Location: input.Location, Status: model.OpportunityStatusOpen,
	}
	if err := service.db.Create(&opportunity).Error; err != nil {
		return nil, fmt.Errorf("create internship opportunity: %w", err)
	}
	return &opportunity, nil
}

func (service *OpportunityService) ListCompany(supervisorID uint) ([]model.InternshipOpportunity, error) {
	companyID, err := service.supervisorCompanyID(supervisorID)
	if err != nil {
		return nil, err
	}
	var opportunities []model.InternshipOpportunity
	if err := service.db.Where("company_id = ?", companyID).Order("created_at DESC").Find(&opportunities).Error; err != nil {
		return nil, fmt.Errorf("list company opportunities: %w", err)
	}
	return opportunities, nil
}

func (service *OpportunityService) GetCompany(supervisorID, opportunityID uint) (*model.InternshipOpportunity, error) {
	companyID, err := service.supervisorCompanyID(supervisorID)
	if err != nil {
		return nil, err
	}
	return service.findCompanyOpportunity(companyID, opportunityID)
}

func (service *OpportunityService) Update(supervisorID, opportunityID uint, input OpportunityInput) (*model.InternshipOpportunity, error) {
	input, err := normalizeOpportunityInput(input)
	if err != nil {
		return nil, err
	}
	companyID, err := service.supervisorCompanyID(supervisorID)
	if err != nil {
		return nil, err
	}
	opportunity, err := service.findCompanyOpportunity(companyID, opportunityID)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{
		"title": input.Title, "description": input.Description,
		"work_field": input.WorkField, "location": input.Location,
	}
	if err := service.db.Model(opportunity).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update internship opportunity: %w", err)
	}
	return service.findCompanyOpportunity(companyID, opportunityID)
}

func (service *OpportunityService) Close(supervisorID, opportunityID uint) (*model.InternshipOpportunity, error) {
	companyID, err := service.supervisorCompanyID(supervisorID)
	if err != nil {
		return nil, err
	}
	opportunity, err := service.findCompanyOpportunity(companyID, opportunityID)
	if err != nil {
		return nil, err
	}
	if opportunity.Status == model.OpportunityStatusClosed {
		return nil, ErrOpportunityAlreadyClosed
	}
	result := service.db.Model(&model.InternshipOpportunity{}).
		Where("id = ? AND company_id = ? AND status = ?", opportunityID, companyID, model.OpportunityStatusOpen).
		Update("status", model.OpportunityStatusClosed)
	if result.Error != nil {
		return nil, fmt.Errorf("close internship opportunity: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrOpportunityAlreadyClosed
	}
	return service.findCompanyOpportunity(companyID, opportunityID)
}

func (service *OpportunityService) ListStudent() ([]model.InternshipOpportunity, error) {
	var opportunities []model.InternshipOpportunity
	if err := service.db.Preload("Company").Where("status = ?", model.OpportunityStatusOpen).
		Order("created_at DESC").Find(&opportunities).Error; err != nil {
		return nil, fmt.Errorf("list student opportunities: %w", err)
	}
	return opportunities, nil
}

func (service *OpportunityService) GetStudent(opportunityID uint) (*model.InternshipOpportunity, error) {
	var opportunity model.InternshipOpportunity
	result := service.db.Preload("Company").Where("id = ? AND status = ?", opportunityID, model.OpportunityStatusOpen).First(&opportunity)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, ErrOpportunityNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("find student opportunity: %w", result.Error)
	}
	return &opportunity, nil
}

func (service *OpportunityService) supervisorCompanyID(supervisorID uint) (uint, error) {
	var supervisor model.User
	result := service.db.Select("id", "company_id").Where("id = ? AND role = ?", supervisorID, model.RoleCompanySupervisor).First(&supervisor)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return 0, ErrOpportunityCompanyMissing
	}
	if result.Error != nil {
		return 0, fmt.Errorf("find opportunity supervisor: %w", result.Error)
	}
	if supervisor.CompanyID == nil || *supervisor.CompanyID == 0 {
		return 0, ErrOpportunityCompanyMissing
	}
	var count int64
	if err := service.db.Model(&model.Company{}).Where("id = ?", *supervisor.CompanyID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("validate opportunity company: %w", err)
	}
	if count == 0 {
		return 0, ErrOpportunityCompanyMissing
	}
	return *supervisor.CompanyID, nil
}

func (service *OpportunityService) findCompanyOpportunity(companyID, opportunityID uint) (*model.InternshipOpportunity, error) {
	var opportunity model.InternshipOpportunity
	result := service.db.Where("id = ? AND company_id = ?", opportunityID, companyID).First(&opportunity)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, ErrOpportunityNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("find company opportunity: %w", result.Error)
	}
	return &opportunity, nil
}

func normalizeOpportunityInput(input OpportunityInput) (OpportunityInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	input.WorkField = strings.TrimSpace(input.WorkField)
	input.Location = strings.TrimSpace(input.Location)
	if input.Title == "" || input.Description == "" || input.WorkField == "" || input.Location == "" ||
		len([]rune(input.Title)) > 250 || len([]rune(input.Description)) > 10000 ||
		len([]rune(input.WorkField)) > 200 || len([]rune(input.Location)) > 500 {
		return OpportunityInput{}, ErrInvalidOpportunity
	}
	return input, nil
}
