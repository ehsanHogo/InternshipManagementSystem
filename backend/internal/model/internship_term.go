package model

import "time"

type InternshipTermType string

const (
	InternshipTermTypeFirst  InternshipTermType = "FIRST"
	InternshipTermTypeSecond InternshipTermType = "SECOND"
	InternshipTermTypeSummer InternshipTermType = "SUMMER"
)

func (kind InternshipTermType) Valid() bool {
	return kind == InternshipTermTypeFirst || kind == InternshipTermTypeSecond || kind == InternshipTermTypeSummer
}

type InternshipTermStatus string

const (
	InternshipTermStatusOpen   InternshipTermStatus = "OPEN"
	InternshipTermStatusClosed InternshipTermStatus = "CLOSED"
)

type InternshipTerm struct {
	ID           uint               `gorm:"primaryKey" json:"id"`
	AcademicYear int                `gorm:"not null;uniqueIndex:idx_internship_term_identity;check:academic_year > 0 AND academic_year <= 9999" json:"academicYear"`
	TermType     InternshipTermType `gorm:"type:varchar(8);not null;uniqueIndex:idx_internship_term_identity;check:term_type IN ('FIRST','SECOND','SUMMER')" json:"termType"`
	// The partial unique index guarantees one OPEN term even across processes.
	Status    InternshipTermStatus `gorm:"type:varchar(8);not null;uniqueIndex:idx_internship_term_one_open,where:status = 'OPEN';check:status IN ('OPEN','CLOSED')" json:"status"`
	OpenedAt  time.Time            `gorm:"not null" json:"openedAt"`
	ClosedAt  *time.Time           `json:"closedAt"`
	CreatedBy uint                 `gorm:"not null" json:"createdBy"`
	Creator   User                 `gorm:"foreignKey:CreatedBy;constraint:OnDelete:RESTRICT" json:"-"`
	ClosedBy  *uint                `json:"closedBy,omitempty"`
	Closer    *User                `gorm:"foreignKey:ClosedBy;constraint:OnDelete:RESTRICT" json:"-"`
	CreatedAt time.Time            `json:"createdAt"`
	UpdatedAt time.Time            `json:"updatedAt"`
}
