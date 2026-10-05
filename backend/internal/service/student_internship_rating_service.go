package service

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"internship-management-system/backend/internal/model"
)

var (
	ErrInvalidStudentRating = errors.New("rating must be an integer from 1 to 5")
	ErrCaseNotRateable      = errors.New("only a passed case with a selected company can be rated")
	ErrStudentRatingExists  = errors.New("this case has already been rated")
)

// CompanyRating is an explicit read-only DTO, never part of the Company model.
// Company and admin responses therefore do not acquire rating fields implicitly.
type CompanyRating struct {
	CompanyAverageRating *float64 `json:"companyAverageRating"`
	CompanyRatingCount   int64    `json:"companyRatingCount"`
}

func companyRatings(db *gorm.DB, companyIDs []uint) (map[uint]CompanyRating, error) {
	result := make(map[uint]CompanyRating)
	uniqueIDs := make([]uint, 0, len(companyIDs))
	for _, id := range companyIDs {
		if id == 0 {
			continue
		}
		if _, exists := result[id]; !exists {
			result[id] = CompanyRating{}
			uniqueIDs = append(uniqueIDs, id)
		}
	}
	companyIDs = uniqueIDs
	if len(companyIDs) == 0 {
		return result, nil
	}
	var rows []struct {
		CompanyID uint
		CompanyRating
	}
	// One grouped query for the request, irrespective of opportunity count.
	err := db.Model(&model.StudentInternshipRating{}).
		Select("company_id, AVG(rating) AS company_average_rating, COUNT(id) AS company_rating_count").
		Where("company_id IN ?", companyIDs).Group("company_id").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("aggregate company ratings: %w", err)
	}
	for _, row := range rows {
		result[row.CompanyID] = row.CompanyRating
	}
	return result, nil
}

func (service *OpportunityService) CompanyRatings(ids []uint) (map[uint]CompanyRating, error) {
	return companyRatings(service.db, ids)
}

func (service *InternshipService) CompanyRatings(ids []uint) (map[uint]CompanyRating, error) {
	return companyRatings(service.db, ids)
}

func (service *InternshipService) CreateStudentRating(studentID, caseID uint, rating int) (*model.StudentInternshipRating, error) {
	if rating < 1 || rating > 5 {
		return nil, ErrInvalidStudentRating
	}
	var value model.StudentInternshipRating
	err := service.db.Transaction(func(tx *gorm.DB) error {
		var student model.User
		if err := tx.First(&student, studentID).Error; err != nil {
			return err
		}
		if student.Role != model.RoleStudent {
			return ErrCaseAccessDenied
		}
		var item model.InternshipCase
		// Serialize submissions on this case; the unique index is a second defense.
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND student_id = ?", caseID, studentID).First(&item).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCaseNotFound
		}
		if err != nil {
			return err
		}
		if item.Status != model.InternshipCaseStatusPassed {
			return ErrCaseNotRateable
		}
		var companyID uint
		err = selectedPassedInternships(tx).
			Where("cases.id = ? AND application.student_id = ?", caseID, studentID).
			Select("opportunity.company_id").Scan(&companyID).Error
		if err != nil {
			return err
		}
		if companyID == 0 {
			return ErrCaseNotRateable
		}
		value = model.StudentInternshipRating{InternshipCaseID: caseID, StudentID: studentID, CompanyID: companyID, Rating: rating}
		result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "internship_case_id"}}, DoNothing: true}).Create(&value)
		if result.Error != nil {
			return fmt.Errorf("create student internship rating: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrStudentRatingExists
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &value, nil
}
