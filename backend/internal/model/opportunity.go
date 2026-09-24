package model

import "time"

type OpportunityStatus string

const (
	OpportunityStatusOpen   OpportunityStatus = "OPEN"
	OpportunityStatusClosed OpportunityStatus = "CLOSED"
)

func (status OpportunityStatus) Valid() bool {
	return status == OpportunityStatusOpen || status == OpportunityStatusClosed
}

type InternshipOpportunity struct {
	ID          uint              `gorm:"primaryKey"`
	CompanyID   uint              `gorm:"not null;index"`
	CreatedBy   uint              `gorm:"not null;index"`
	Title       string            `gorm:"size:250;not null"`
	Description string            `gorm:"type:text;not null"`
	WorkField   string            `gorm:"size:200;not null"`
	Location    string            `gorm:"size:500;not null"`
	Status      OpportunityStatus `gorm:"type:varchar(16);not null;index;check:status IN ('OPEN','CLOSED')"`
	Company     Company           `gorm:"foreignKey:CompanyID"`
	Creator     User              `gorm:"foreignKey:CreatedBy"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
