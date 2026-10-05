package service

import (
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"internship-management-system/backend/internal/auth"
	"internship-management-system/backend/internal/model"
)

var (
	ErrInvalidProfile       = errors.New("invalid profile fields")
	ErrInvalidNewPassword   = errors.New("invalid new password")
	ErrWrongCurrentPassword = errors.New("incorrect current password")
	ErrPasswordUnchanged    = errors.New("new password equals current password")
)

type ProfileService struct{ db *gorm.DB }

// ProfileInput contains personal fields only. Nil JobTitle preserves the value.
type ProfileInput struct {
	FullName string
	Phone    string
	JobTitle *string
}

func NewProfileService(db *gorm.DB) *ProfileService { return &ProfileService{db: db} }

func (service *ProfileService) Update(userID uint, input ProfileInput) (*model.PublicUser, error) {
	input.FullName = strings.TrimSpace(input.FullName)
	input.Phone = strings.TrimSpace(input.Phone)
	if input.FullName == "" || len([]rune(input.FullName)) > 200 || len([]rune(input.Phone)) > 50 {
		return nil, ErrInvalidProfile
	}
	var user model.User
	err := service.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
			return err
		}
		updates := map[string]any{"full_name": input.FullName, "phone": optionalString(input.Phone)}
		if input.JobTitle != nil {
			jobTitle := strings.TrimSpace(*input.JobTitle)
			if user.Role != model.RoleCompanySupervisor || len([]rune(jobTitle)) > 200 {
				return ErrInvalidProfile
			}
			updates["job_title"] = optionalString(jobTitle)
		}
		// An explicit column allowlist avoids mass assignment and association writes.
		if err := tx.Model(&model.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Preload("Company").First(&user, user.ID).Error
	})
	if err != nil {
		return nil, err
	}
	public := user.Public()
	return &public, nil
}

func (service *ProfileService) ChangePassword(userID uint, currentPassword, newPassword string) error {
	// Match company registration: nonblank and at most bcrypt's 72-byte limit.
	// Do not trim the actual password; spaces can be intentional.
	if strings.TrimSpace(newPassword) == "" || len([]byte(newPassword)) > 72 {
		return ErrInvalidNewPassword
	}
	var user model.User
	if err := service.db.First(&user, userID).Error; err != nil {
		return err
	}
	if !auth.CheckPassword(user.PasswordHash, currentPassword) {
		return ErrWrongCurrentPassword
	}
	if newPassword == currentPassword {
		return ErrPasswordUnchanged
	}
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	// Compare-and-swap prevents two concurrent requests using the old password
	// from both succeeding. No other account fields or sessions are changed.
	result := service.db.Model(&model.User{}).Where("id = ? AND password_hash = ?", user.ID, user.PasswordHash).Update("password_hash", hash)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrWrongCurrentPassword
	}
	return nil
}
