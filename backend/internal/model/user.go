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

type UserVerificationStatus string

const (
	UserVerificationNotRequired UserVerificationStatus = "NOT_REQUIRED"
	UserVerificationPending     UserVerificationStatus = "PENDING"
	UserVerificationApproved    UserVerificationStatus = "APPROVED"
	UserVerificationRejected    UserVerificationStatus = "REJECTED"
)

func (status UserVerificationStatus) Reviewable() bool {
	return status == UserVerificationPending || status == UserVerificationApproved || status == UserVerificationRejected
}

type User struct {
	IsActive                    bool                   `gorm:"not null;default:true"`
	VerificationStatus          UserVerificationStatus `gorm:"type:varchar(16);not null;default:NOT_REQUIRED;check:chk_user_verification_status,(role = 'UNIVERSITY_SUPERVISOR' AND verification_status IN ('PENDING','APPROVED','REJECTED')) OR (role <> 'UNIVERSITY_SUPERVISOR' AND verification_status = 'NOT_REQUIRED')"`
	VerificationReviewedAt      *time.Time
	VerificationReviewedBy      *uint
	VerificationRejectionReason *string `gorm:"type:text"`
	VerificationResubmittedAt   *time.Time
	ID                          uint    `gorm:"primaryKey"`
	FullName                    string  `gorm:"size:200;not null"`
	Email                       string  `gorm:"size:320;uniqueIndex;not null"`
	PasswordHash                string  `gorm:"not null" json:"-"`
	Role                        Role    `gorm:"type:varchar(32);not null"`
	StudentNumber               *string `gorm:"size:50;uniqueIndex"`
	Major                       *string `gorm:"size:200"`
	Phone                       *string `gorm:"size:50"`
	JobTitle                    *string `gorm:"size:200"`
	CompanyID                   *uint
	Company                     *Company `gorm:"foreignKey:CompanyID" json:"-"`
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

type PublicUser struct {
	IsActive                    bool                       `json:"isActive"`
	CreatedAt                   time.Time                  `json:"createdAt"`
	VerificationStatus          UserVerificationStatus     `json:"verificationStatus"`
	VerificationReviewedAt      *time.Time                 `json:"verificationReviewedAt,omitempty"`
	VerificationRejectionReason *string                    `json:"verificationRejectionReason,omitempty"`
	VerificationResubmittedAt   *time.Time                 `json:"verificationResubmittedAt,omitempty"`
	ID                          uint                       `json:"id"`
	FullName                    string                     `json:"fullName"`
	Email                       string                     `json:"email"`
	Role                        Role                       `json:"role"`
	StudentNumber               *string                    `json:"studentNumber,omitempty"`
	Major                       *string                    `json:"major,omitempty"`
	Phone                       *string                    `json:"phone,omitempty"`
	JobTitle                    *string                    `json:"jobTitle,omitempty"`
	CompanyID                   *uint                      `json:"companyId,omitempty"`
	CompanyName                 *string                    `json:"companyName,omitempty"`
	CompanyRegistrationStatus   *CompanyRegistrationStatus `json:"companyRegistrationStatus,omitempty"`
}

func (user User) Public() PublicUser {
	var status *CompanyRegistrationStatus
	var companyName *string
	if user.Role == RoleCompanySupervisor && user.Company != nil {
		value := user.Company.RegistrationStatus
		status = &value
		name := user.Company.Name
		companyName = &name
	}
	return PublicUser{
		IsActive:                    user.IsActive,
		CreatedAt:                   user.CreatedAt,
		VerificationStatus:          user.VerificationStatus,
		VerificationReviewedAt:      user.VerificationReviewedAt,
		VerificationRejectionReason: user.VerificationRejectionReason,
		VerificationResubmittedAt:   user.VerificationResubmittedAt,
		CompanyRegistrationStatus:   status,
		CompanyName:                 companyName,
		ID:                          user.ID,
		FullName:                    user.FullName,
		Email:                       user.Email,
		Role:                        user.Role,
		StudentNumber:               user.StudentNumber,
		Major:                       user.Major,
		Phone:                       user.Phone,
		JobTitle:                    user.JobTitle,
		CompanyID:                   user.CompanyID,
	}
}
