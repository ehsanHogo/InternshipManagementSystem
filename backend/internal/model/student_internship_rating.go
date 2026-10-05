package model

import "time"

// StudentInternshipRating snapshots the company resolved from a successful case.
// It is final: the application exposes creation only, with no update/delete API.
type StudentInternshipRating struct {
	ID               uint           `gorm:"primaryKey"`
	InternshipCaseID uint           `gorm:"not null;uniqueIndex"`
	StudentID        uint           `gorm:"not null;index"`
	CompanyID        uint           `gorm:"not null;index"`
	Rating           int            `gorm:"not null;check:chk_student_internship_rating,rating >= 1 AND rating <= 5"`
	CreatedAt        time.Time      `gorm:"not null"`
	InternshipCase   InternshipCase `gorm:"foreignKey:InternshipCaseID;constraint:OnDelete:RESTRICT"`
	Student          User           `gorm:"foreignKey:StudentID;constraint:OnDelete:RESTRICT"`
	Company          Company        `gorm:"foreignKey:CompanyID;constraint:OnDelete:RESTRICT"`
}
