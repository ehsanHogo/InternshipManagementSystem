package model

import "time"

type ApplicationStatus string

const (
	ApplicationStatusPending  ApplicationStatus = "PENDING"
	ApplicationStatusAccepted ApplicationStatus = "ACCEPTED"
	ApplicationStatusRejected ApplicationStatus = "REJECTED"
)

func (status ApplicationStatus) Valid() bool {
	switch status {
	case ApplicationStatusPending, ApplicationStatusAccepted, ApplicationStatusRejected:
		return true
	default:
		return false
	}
}

type OpportunityApplication struct {
	ID             uint              `gorm:"primaryKey"`
	OpportunityID  uint              `gorm:"not null;uniqueIndex:idx_opportunity_student"`
	StudentID      uint              `gorm:"not null;uniqueIndex:idx_opportunity_student"`
	ResumeFileID   uint              `gorm:"not null;uniqueIndex"`
	Status         ApplicationStatus `gorm:"type:varchar(16);not null;index;check:status IN ('PENDING','ACCEPTED','REJECTED')"`
	CompanyComment *string           `gorm:"type:text"`
	AppliedAt      time.Time         `gorm:"not null"`
	ReviewedAt     *time.Time

	Opportunity InternshipOpportunity `gorm:"foreignKey:OpportunityID"`
	Student     User                  `gorm:"foreignKey:StudentID"`
	ResumeFile  File                  `gorm:"foreignKey:ResumeFileID"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
