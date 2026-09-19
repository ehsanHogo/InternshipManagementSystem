package model

import "time"

type Role string

const (
	RoleStudent              Role = "STUDENT"
	RoleProfessor            Role = "PROFESSOR"
	RoleCompanySupervisor    Role = "COMPANY_SUPERVISOR"
	RoleUniversitySupervisor Role = "UNIVERSITY_SUPERVISOR"
	RoleAdmin                Role = "ADMIN"
)

func (role Role) Valid() bool {
	switch role {
	case RoleStudent, RoleProfessor, RoleCompanySupervisor, RoleUniversitySupervisor, RoleAdmin:
		return true
	default:
		return false
	}
}

type User struct {
	ID            uint    `gorm:"primaryKey"`
	FullName      string  `gorm:"size:200;not null"`
	Email         string  `gorm:"size:320;uniqueIndex;not null"`
	PasswordHash  string  `gorm:"not null" json:"-"`
	Role          Role    `gorm:"type:varchar(32);not null"`
	StudentNumber *string `gorm:"size:50"`
	Major         *string `gorm:"size:200"`
	CompanyID     *uint
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type PublicUser struct {
	ID            uint    `json:"id"`
	FullName      string  `json:"fullName"`
	Email         string  `json:"email"`
	Role          Role    `json:"role"`
	StudentNumber *string `json:"studentNumber,omitempty"`
	Major         *string `json:"major,omitempty"`
	CompanyID     *uint   `json:"companyId,omitempty"`
}

func (user User) Public() PublicUser {
	return PublicUser{
		ID:            user.ID,
		FullName:      user.FullName,
		Email:         user.Email,
		Role:          user.Role,
		StudentNumber: user.StudentNumber,
		Major:         user.Major,
		CompanyID:     user.CompanyID,
	}
}
