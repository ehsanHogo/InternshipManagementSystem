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
	ErrApprovalCompanyNotFound       = errors.New("company not found")
	ErrCompanyAlreadyApproved        = errors.New("company already approved")
	ErrCompanyNotEligibleForApproval = errors.New("company not eligible for approval")
)

type CompanyApprovalService struct{ db *gorm.DB }

type ApprovalCompanyView struct {
	model.Company
	PassedInternshipCount int64 `json:"passedInternshipCount"`
}

type SuccessfulInternshipView struct {
	CaseID           uint                        `json:"caseId"`
	StudentName      string                      `json:"studentName"`
	OpportunityTitle string                      `json:"opportunityTitle"`
	FinalResult      *model.ProfessorFinalResult `json:"finalResult"`
	CompletedAt      *time.Time                  `json:"completedAt"`
}

type ApprovalCompanyDetail struct {
	CompanyRating
	ApprovalCompanyView
	SuccessfulInternships []SuccessfulInternshipView `json:"successfulInternships"`
}

// PublicApprovedCompany deliberately excludes company identifiers and student evidence.
type PublicApprovedCompany struct {
	ID      uint    `json:"id"`
	Name    string  `json:"name"`
	Website *string `json:"website"`
	Phone   *string `json:"phone"`
	Email   *string `json:"email"`
	Address *string `json:"address"`
}

func NewCompanyApprovalService(db *gorm.DB) *CompanyApprovalService {
	return &CompanyApprovalService{db: db}
}

// All approval evidence follows the university-selected placement, never an
// arbitrary preference, accepted application, or cached company supervisor.
func selectedPassedInternships(db *gorm.DB) *gorm.DB {
	return db.Table("internship_cases AS cases").
		Joins("JOIN internship_preferences AS preference ON preference.id = cases.selected_preference_id AND preference.internship_case_id = cases.id").
		Joins("JOIN opportunity_applications AS application ON application.id = preference.opportunity_application_id").
		Joins("JOIN internship_opportunities AS opportunity ON opportunity.id = application.opportunity_id").
		Where("cases.status = ?", model.InternshipCaseStatusPassed)
}

func (service *CompanyApprovalService) ListEligibleCompanies() ([]ApprovalCompanyView, error) {
	counts := selectedPassedInternships(service.db).
		Select("opportunity.company_id, COUNT(DISTINCT cases.id) AS passed_internship_count").Group("opportunity.company_id")
	companies := make([]ApprovalCompanyView, 0)
	err := service.db.Model(&model.Company{}).
		Select("companies.*, evidence.passed_internship_count").
		Joins("JOIN (?) AS evidence ON evidence.company_id = companies.id", counts).
		Where("companies.is_approved = ?", false).Order("companies.name ASC, companies.id ASC").Scan(&companies).Error
	if err != nil {
		return nil, fmt.Errorf("list eligible companies: %w", err)
	}
	return companies, nil
}

func (service *CompanyApprovalService) ListApprovedCompanies() ([]model.Company, error) {
	companies := make([]model.Company, 0)
	if err := service.db.Where("is_approved = ?", true).Order("name ASC, id ASC").Find(&companies).Error; err != nil {
		return nil, fmt.Errorf("list approved companies: %w", err)
	}
	return companies, nil
}

func (service *CompanyApprovalService) ListStudentApprovedCompanies() ([]PublicApprovedCompany, error) {
	companies := make([]PublicApprovedCompany, 0)
	err := service.db.Model(&model.Company{}).Select("id, name, website, phone, email, address").
		Where("is_approved = ?", true).Order("name ASC, id ASC").Scan(&companies).Error
	if err != nil {
		return nil, fmt.Errorf("list student approved companies: %w", err)
	}
	return companies, nil
}

func (service *CompanyApprovalService) GetCompany(id uint) (*ApprovalCompanyDetail, error) {
	var detail ApprovalCompanyDetail
	if err := service.db.First(&detail.Company, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrApprovalCompanyNotFound
	} else if err != nil {
		return nil, fmt.Errorf("get approval company: %w", err)
	}
	ratings, err := companyRatings(service.db, []uint{id})
	if err != nil {
		return nil, err
	}
	detail.CompanyRating = ratings[id]
	detail.SuccessfulInternships = make([]SuccessfulInternshipView, 0)
	query := selectedPassedInternships(service.db).Where("opportunity.company_id = ?", id)
	if err := query.Distinct("cases.id").Count(&detail.PassedInternshipCount).Error; err != nil {
		return nil, fmt.Errorf("count successful internships: %w", err)
	}
	// Keep the approval view concise; the count above still includes all passed cases.
	err = selectedPassedInternships(service.db).Where("opportunity.company_id = ?", id).
		Joins("JOIN users AS student ON student.id = cases.student_id").
		Select("cases.id AS case_id, student.full_name AS student_name, opportunity.title AS opportunity_title, cases.final_result, cases.completed_at").
		Order("cases.completed_at DESC NULLS LAST, cases.id DESC").Limit(10).Scan(&detail.SuccessfulInternships).Error
	if err != nil {
		return nil, fmt.Errorf("get successful internship evidence: %w", err)
	}
	return &detail, nil
}

func (service *CompanyApprovalService) ApproveCompany(id uint) (*model.Company, error) {
	var company model.Company
	err := service.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&company, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrApprovalCompanyNotFound
		} else if err != nil {
			return fmt.Errorf("lock approval company: %w", err)
		}
		if company.IsApproved {
			return ErrCompanyAlreadyApproved
		}
		var count int64
		if err := selectedPassedInternships(tx).Where("opportunity.company_id = ?", id).Distinct("cases.id").Count(&count).Error; err != nil {
			return fmt.Errorf("revalidate company approval eligibility: %w", err)
		}
		if count == 0 {
			return ErrCompanyNotEligibleForApproval
		}
		// UpdateColumn intentionally avoids touching UpdatedAt or any associations.
		if err := tx.Model(&company).UpdateColumn("is_approved", true).Error; err != nil {
			return fmt.Errorf("approve company: %w", err)
		}
		company.IsApproved = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &company, nil
}
