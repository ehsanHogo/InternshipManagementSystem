package model

import "time"

const (
	MinPreferenceCount = 1
	MaxPreferenceCount = 3
)

type InternshipCaseStatus string

const (
	InternshipCaseStatusDraft                   InternshipCaseStatus = "DRAFT"
	InternshipCaseStatusRevisionRequested       InternshipCaseStatus = "REVISION_REQUESTED"
	InternshipCaseStatusPendingUniversityReview InternshipCaseStatus = "PENDING_UNIVERSITY_REVIEW"
	InternshipCaseStatusPendingCompanyDetails   InternshipCaseStatus = "PENDING_COMPANY_DETAILS"
	InternshipCaseStatusPendingFinalApproval    InternshipCaseStatus = "PENDING_FINAL_APPROVAL"
	InternshipCaseStatusActive                  InternshipCaseStatus = "ACTIVE"
	InternshipCaseStatusPassed                  InternshipCaseStatus = "PASSED"
	InternshipCaseStatusFailed                  InternshipCaseStatus = "FAILED"
	InternshipCaseStatusCancelled               InternshipCaseStatus = "CANCELLED"
)

func (status InternshipCaseStatus) Valid() bool {
	switch status {
	case InternshipCaseStatusDraft,
		InternshipCaseStatusPendingUniversityReview,
		InternshipCaseStatusPendingCompanyDetails,
		InternshipCaseStatusPendingFinalApproval,
		InternshipCaseStatusRevisionRequested,
		InternshipCaseStatusActive,
		InternshipCaseStatusPassed,
		InternshipCaseStatusFailed,
		InternshipCaseStatusCancelled:
		return true
	default:
		return false
	}
}

type ProfessorFinalResult string

const (
	ProfessorFinalResultExcellent ProfessorFinalResult = "EXCELLENT"
	ProfessorFinalResultGood      ProfessorFinalResult = "GOOD"
	ProfessorFinalResultFailed    ProfessorFinalResult = "FAILED"
)

func (result ProfessorFinalResult) Valid() bool {
	switch result {
	case ProfessorFinalResultExcellent, ProfessorFinalResultGood, ProfessorFinalResultFailed:
		return true
	default:
		return false
	}
}

type Company struct {
	ID                          uint                      `gorm:"primaryKey" json:"id"`
	Name                        string                    `gorm:"size:250;uniqueIndex;not null" json:"name"`
	NationalID                  string                    `gorm:"size:50;not null;uniqueIndex;check:char_length(btrim(national_id)) > 0" json:"nationalId"`
	EconomicCode                string                    `gorm:"size:50;not null;uniqueIndex;check:char_length(btrim(economic_code)) > 0" json:"economicCode"`
	Website                     *string                   `gorm:"size:500" json:"website,omitempty"`
	Phone                       *string                   `gorm:"size:50" json:"phone,omitempty"`
	Email                       *string                   `gorm:"size:320" json:"email,omitempty"`
	Address                     *string                   `gorm:"size:1000" json:"address,omitempty"`
	RegistrationStatus          CompanyRegistrationStatus `gorm:"type:varchar(16);not null;default:PENDING;index;check:chk_company_registration_status,registration_status IN ('PENDING','APPROVED','REJECTED')" json:"registrationStatus"`
	RegistrationReviewedAt      *time.Time                `json:"registrationReviewedAt,omitempty"`
	RegistrationReviewedBy      *uint                     `gorm:"index" json:"registrationReviewedBy,omitempty"`
	RegistrationRejectionReason *string                   `gorm:"type:text" json:"registrationRejectionReason,omitempty"`
	IsApproved                  bool                      `gorm:"not null;default:false;index" json:"isApproved"`
	CreatedAt                   time.Time                 `json:"createdAt"`
	UpdatedAt                   time.Time                 `json:"updatedAt"`
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
	ID uint `gorm:"primaryKey"`
	// Nullable only for legacy cases. New cases receive the currently open term.
	TermID      *uint                `gorm:"index"`
	Term        *InternshipTerm      `gorm:"foreignKey:TermID;constraint:OnDelete:RESTRICT"`
	StudentID   uint                 `gorm:"not null;index"`
	ProfessorID uint                 `gorm:"not null;index"`
	Status      InternshipCaseStatus `gorm:"type:varchar(32);not null;index;check:chk_internship_case_status,status IN ('DRAFT','PENDING_UNIVERSITY_REVIEW','PENDING_COMPANY_DETAILS','PENDING_FINAL_APPROVAL','REVISION_REQUESTED','ACTIVE','PASSED','FAILED','CANCELLED')"`

	PassedCredits *int    `gorm:"check:passed_credits IS NULL OR passed_credits >= 0"`
	Mobile        *string `gorm:"size:30"`

	SelectedPreferenceID *uint
	// CompanySupervisorID snapshots the selected opportunity's supervisor.
	// It is never populated through arbitrary university selection.
	CompanySupervisorID *uint
	LetterNumber        *string `gorm:"size:100"`
	LetterDate          *time.Time
	InternshipSubject   *string `gorm:"size:500"`
	StartDate           *time.Time
	WorkplaceAddress    *string               `gorm:"size:1000"`
	WorkplacePhone      *string               `gorm:"size:50"`
	FinalResult         *ProfessorFinalResult `gorm:"type:varchar(16);check:final_result IS NULL OR final_result IN ('EXCELLENT','GOOD','FAILED')"`
	ProfessorComment    *string               `gorm:"size:2000"`
	CancellationComment *string               `gorm:"type:text"`
	// Latest preference revision request; retained after student resubmission.
	UniversityRevisionComment     *string `gorm:"type:text"`
	UniversityRevisionRequestedAt *time.Time
	UniversityRevisionRequestedBy *uint
	UniversityRevisionRequester   *User   `gorm:"foreignKey:UniversityRevisionRequestedBy"`
	CompanyDetailsRevisionComment *string `gorm:"type:text"`

	Student            User                     `gorm:"foreignKey:StudentID"`
	Professor          User                     `gorm:"foreignKey:ProfessorID"`
	Preferences        []InternshipPreference   `gorm:"foreignKey:InternshipCaseID"`
	SelectedPreference *InternshipPreference    `gorm:"foreignKey:SelectedPreferenceID;-:migration"`
	CompanySupervisor  *User                    `gorm:"foreignKey:CompanySupervisorID"`
	StudentRating      *StudentInternshipRating `gorm:"foreignKey:InternshipCaseID" json:"-"`
	FinalReport        *FinalReport             `gorm:"foreignKey:InternshipCaseID"`
	WeeklyReports      []WeeklyReport           `gorm:"foreignKey:InternshipCaseID"`
	CompanyEvaluation  *CompanyEvaluation       `gorm:"foreignKey:InternshipCaseID"`

	CreatedAt   time.Time
	UpdatedAt   time.Time
	SubmittedAt *time.Time
	ActivatedAt *time.Time
	// CompletedAt records final evaluation for both successful and failed outcomes.
	CompletedAt *time.Time
	CancelledAt *time.Time
}

type InternshipPreference struct {
	ID                       uint                   `gorm:"primaryKey"`
	InternshipCaseID         uint                   `gorm:"not null;uniqueIndex:idx_case_priority;uniqueIndex:idx_case_application"`
	OpportunityApplicationID uint                   `gorm:"not null;uniqueIndex:idx_case_application"`
	Priority                 int                    `gorm:"not null;uniqueIndex:idx_case_priority;check:priority >= 1 AND priority <= 3"`
	OpportunityApplication   OpportunityApplication `gorm:"foreignKey:OpportunityApplicationID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

func (status InternshipCaseStatus) Terminal() bool {
	return status == InternshipCaseStatusPassed || status == InternshipCaseStatusFailed || status == InternshipCaseStatusCancelled
}
