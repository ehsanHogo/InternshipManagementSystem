package model

import "time"

type FinalReportStatus string

const (
	FinalReportSubmitted         FinalReportStatus = "SUBMITTED"
	FinalReportRevisionRequested FinalReportStatus = "REVISION_REQUESTED"
	FinalReportApproved          FinalReportStatus = "APPROVED"
)

type FinalReport struct {
	ID               uint              `gorm:"primaryKey" json:"id"`
	InternshipCaseID uint              `gorm:"not null;uniqueIndex" json:"internshipCaseId"`
	CurrentFileID    uint              `gorm:"not null" json:"currentFileId"`
	Status           FinalReportStatus `gorm:"type:varchar(24);not null;check:status IN ('SUBMITTED','REVISION_REQUESTED','APPROVED')" json:"status"`
	ReviewComment    *string           `gorm:"type:text" json:"reviewComment"`
	SubmittedAt      time.Time         `gorm:"not null" json:"submittedAt"`
	ReviewedAt       *time.Time        `json:"reviewedAt"`
	CurrentFile      File              `gorm:"foreignKey:CurrentFileID;constraint:OnDelete:RESTRICT" json:"currentFile"`
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
}
