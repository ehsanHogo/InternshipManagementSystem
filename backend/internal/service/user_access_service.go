package service

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"internship-management-system/backend/internal/auth"
	"internship-management-system/backend/internal/model"
)

var (
	ErrInvalidUserAccess      = errors.New("invalid user access input")
	ErrVerificationTransition = errors.New("invalid user verification transition")
	ErrVerificationReason     = errors.New("verification rejection reason required")
	ErrAdminSelfDisable       = errors.New("admin cannot disable self")
	ErrLastActiveAdmin        = errors.New("another active admin required")
	ErrUserAccessForbidden    = errors.New("user access action forbidden")
)

type UserAccessService struct{ db *gorm.DB }
type UniversityRegistrationInput struct {
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}
type AdminUserDetail struct {
	model.PublicUser
	VerificationReviewedBy *uint `json:"verificationReviewedBy,omitempty"`
}

func NewUserAccessService(db *gorm.DB) *UserAccessService { return &UserAccessService{db: db} }

func (s *UserAccessService) Register(input UniversityRegistrationInput) (*model.PublicUser, error) {
	input.FullName = strings.TrimSpace(input.FullName)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Phone = strings.TrimSpace(input.Phone)
	if input.FullName == "" || len([]rune(input.FullName)) > 200 || !validEmail(input.Email) || len(input.Email) > 320 || len([]rune(input.Phone)) > 50 || strings.TrimSpace(input.Password) == "" || len([]byte(input.Password)) > 72 {
		return nil, ErrInvalidUserAccess
	}
	var count int64
	if err := s.db.Model(&model.User{}).Where("email = ?", input.Email).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrEmailAlreadyExists
	}
	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		return nil, err
	}
	user := model.User{FullName: input.FullName, Email: input.Email, Phone: optionalString(input.Phone), PasswordHash: hash, Role: model.RoleUniversitySupervisor, IsActive: true, VerificationStatus: model.UserVerificationPending}
	// Failed inserts must not write credential hashes into SQL error logs.
	if err := s.db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Create(&user).Error; err != nil {
		return nil, classifyRegistrationConstraint(err)
	}
	public := user.Public()
	return &public, nil
}

func (s *UserAccessService) List(verificationOnly bool, status model.UserVerificationStatus, search string, role model.Role, active *bool) ([]model.PublicUser, error) {
	if (status != "" && !status.Reviewable()) || (role != "" && !role.Valid()) {
		return nil, ErrInvalidUserAccess
	}
	query := s.db.Preload("Company").Order("created_at DESC, id DESC")
	if verificationOnly {
		query = query.Where("role = ?", model.RoleUniversitySupervisor)
	}
	if status != "" {
		query = query.Where("verification_status = ?", status)
	}
	if role != "" {
		query = query.Where("role = ?", role)
	}
	if active != nil {
		query = query.Where("is_active = ?", *active)
	}
	if search = strings.TrimSpace(search); search != "" {
		// Treat wildcard characters literally in user-entered searches.
		pattern := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(search) + "%"
		query = query.Where("full_name ILIKE ? OR email ILIKE ?", pattern, pattern)
	}
	var users []model.User
	if err := query.Find(&users).Error; err != nil {
		return nil, err
	}
	result := make([]model.PublicUser, 0, len(users))
	for _, user := range users {
		result = append(result, user.Public())
	}
	return result, nil
}

func (s *UserAccessService) Detail(id uint, verificationOnly bool) (*AdminUserDetail, error) {
	var user model.User
	query := s.db.Preload("Company")
	if verificationOnly {
		query = query.Where("role = ?", model.RoleUniversitySupervisor)
	}
	if err := query.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &AdminUserDetail{PublicUser: user.Public(), VerificationReviewedBy: user.VerificationReviewedBy}, nil
}

func requireActiveAdmin(tx *gorm.DB, id uint) error {
	var admin model.User
	if err := tx.First(&admin, id).Error; err != nil {
		return err
	}
	if admin.Role != model.RoleAdmin || !admin.IsActive {
		return ErrUserAccessForbidden
	}
	return nil
}

func (s *UserAccessService) Review(adminID, id uint, status model.UserVerificationStatus, reason string) (*AdminUserDetail, error) {
	reason = strings.TrimSpace(reason)
	if status != model.UserVerificationApproved && status != model.UserVerificationRejected {
		return nil, ErrVerificationTransition
	}
	if status == model.UserVerificationRejected && (reason == "" || len([]rune(reason)) > 5000) {
		return nil, ErrVerificationReason
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := requireActiveAdmin(tx, adminID); err != nil {
			return err
		}
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("role = ?", model.RoleUniversitySupervisor).First(&user, id).Error; err != nil {
			return err
		}
		if user.VerificationStatus != model.UserVerificationPending {
			return ErrVerificationTransition
		}
		var rejectionReason *string
		if status == model.UserVerificationRejected {
			rejectionReason = &reason
		}
		return tx.Model(&user).Updates(map[string]any{"verification_status": status, "verification_reviewed_at": time.Now(), "verification_reviewed_by": adminID, "verification_rejection_reason": rejectionReason}).Error
	})
	if err != nil {
		return nil, err
	}
	return s.Detail(id, true)
}

func (s *UserAccessService) Resubmit(id uint) (*model.PublicUser, error) {
	var user model.User
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, id).Error; err != nil {
			return err
		}
		if user.Role != model.RoleUniversitySupervisor || !user.IsActive {
			return ErrUserAccessForbidden
		}
		if user.VerificationStatus != model.UserVerificationRejected {
			return ErrVerificationTransition
		}
		// Clear the previous decision; the next review will populate latest metadata.
		if err := tx.Model(&user).Updates(map[string]any{"verification_status": model.UserVerificationPending, "verification_resubmitted_at": time.Now(), "verification_reviewed_at": nil, "verification_reviewed_by": nil, "verification_rejection_reason": nil}).Error; err != nil {
			return err
		}
		return tx.First(&user, id).Error
	})
	if err != nil {
		return nil, err
	}
	public := user.Public()
	return &public, nil
}

func (s *UserAccessService) SetActive(adminID, id uint, active bool) (*AdminUserDetail, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// All activation changes share this transaction lock. Concurrent Admin
		// disables cannot both count the other account as the last active Admin.
		if err := tx.Exec("SELECT pg_advisory_xact_lock(220022)").Error; err != nil {
			return err
		}
		if err := requireActiveAdmin(tx, adminID); err != nil {
			return err
		}
		if !active && adminID == id {
			return ErrAdminSelfDisable
		}
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, id).Error; err != nil {
			return err
		}
		if !active && user.Role == model.RoleAdmin && user.IsActive {
			var count int64
			if err := tx.Model(&model.User{}).Where("role = ? AND is_active = true AND id <> ?", model.RoleAdmin, id).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ErrLastActiveAdmin
			}
		}
		return tx.Model(&user).Update("is_active", active).Error
	})
	if err != nil {
		return nil, err
	}
	return s.Detail(id, false)
}
