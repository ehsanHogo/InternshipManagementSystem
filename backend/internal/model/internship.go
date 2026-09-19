package model

import "time"

const (
	MinPreferenceCount = 1
	MaxPreferenceCount = 3
)

type InternshipCaseStatus string

const (
	InternshipCaseStatusDraft                     InternshipCaseStatus = "DRAFT"
	InternshipCaseStatusPendingUniversityApproval InternshipCaseStatus = "PENDING_UNIVERSITY_APPROVAL"
	InternshipCaseStatusPendingCompanyApproval    InternshipCaseStatus = "PENDING_COMPANY_APPROVAL"
	InternshipCaseStatusCompanyApproved           InternshipCaseStatus = "COMPANY_APPROVED"
	InternshipCaseStatusUniversityApproved        InternshipCaseStatus = "UNIVERSITY_APPROVED"
	InternshipCaseStatusActive                    InternshipCaseStatus = "ACTIVE"
	InternshipCaseStatusCompleted                 InternshipCaseStatus = "COMPLETED"
)

type Company struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"size:250;uniqueIndex;not null" json:"name"`
	Website    *string   `gorm:"size:500" json:"website,omitempty"`
	Phone      *string   `gorm:"size:50" json:"phone,omitempty"`
	Email      *string   `gorm:"size:320" json:"email,omitempty"`
	Address    *string   `gorm:"size:1000" json:"address,omitempty"`
	IsApproved bool      `gorm:"not null;default:false;index" json:"isApproved"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type ProfessorAssignment struct {
	ID          uint      `gorm:"primaryKey"`
	StudentID   uint      `gorm:"uniqueIndex;not null"`
	ProfessorID uint      `gorm:"not null"`
	Student     User      `gorm:"foreignKey:StudentID"`
	Professor   User      `gorm:"foreignKey:ProfessorID"`
	AssignedAt  time.Time `gorm:"not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type InternshipCase struct {
	ID          uint                 `gorm:"primaryKey"`
	StudentID   uint                 `gorm:"not null;index"`
	ProfessorID uint                 `gorm:"not null;index"`
	Status      InternshipCaseStatus `gorm:"type:varchar(32);not null;index"`

	PassedCredits *int    `gorm:"check:passed_credits IS NULL OR passed_credits >= 0"`
	Mobile        *string `gorm:"size:30"`

	SelectedPreferenceID *uint
	CompanySupervisorID  *uint
	LetterNumber         *string `gorm:"size:100"`
	LetterDate           *time.Time
	InternshipSubject    *string `gorm:"size:500"`
	StartDate            *time.Time
	WorkplaceAddress     *string `gorm:"size:1000"`
	WorkplacePhone       *string `gorm:"size:50"`
	FinalReportFileID    *uint
	FinalGrade           *float64
	ProfessorComment     *string `gorm:"size:2000"`

	Student            User                   `gorm:"foreignKey:StudentID"`
	Professor          User                   `gorm:"foreignKey:ProfessorID"`
	Preferences        []InternshipPreference `gorm:"foreignKey:InternshipCaseID"`
	SelectedPreference *InternshipPreference  `gorm:"foreignKey:SelectedPreferenceID;-:migration"`
	CompanySupervisor  *User                  `gorm:"foreignKey:CompanySupervisorID"`

	CreatedAt            time.Time
	UpdatedAt            time.Time
	SubmittedAt          *time.Time
	CompanyConfirmedAt   *time.Time
	UniversityApprovedAt *time.Time
	ActivatedAt          *time.Time
	CompletedAt          *time.Time
}

type InternshipPreference struct {
	ID               uint `gorm:"primaryKey"`
	InternshipCaseID uint `gorm:"not null;uniqueIndex:idx_case_priority"`
	Priority         int  `gorm:"not null;uniqueIndex:idx_case_priority"`

	CompanyID              *uint
	Company                *Company `gorm:"foreignKey:CompanyID"`
	ProposedCompanyName    *string  `gorm:"size:250"`
	ProposedWebsite        *string  `gorm:"size:500"`
	ProposedPhone          *string  `gorm:"size:50"`
	ProposedEmail          *string  `gorm:"size:320"`
	ProposedSupervisorName *string  `gorm:"size:200"`

	City      string `gorm:"size:150;not null"`
	WorkField string `gorm:"size:300;not null"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
