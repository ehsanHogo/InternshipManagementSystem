package model

import "time"

type EvaluationRating string

const (
	EvaluationRatingExcellent EvaluationRating = "EXCELLENT"
	EvaluationRatingGood      EvaluationRating = "GOOD"
	EvaluationRatingAverage   EvaluationRating = "AVERAGE"
	EvaluationRatingWeak      EvaluationRating = "WEAK"
	EvaluationRatingFailed    EvaluationRating = "FAILED"
)

func (rating EvaluationRating) Valid() bool {
	switch rating {
	case EvaluationRatingExcellent, EvaluationRatingGood, EvaluationRatingAverage,
		EvaluationRatingWeak, EvaluationRatingFailed:
		return true
	default:
		return false
	}
}

type WeeklyReport struct {
	ID                  uint       `gorm:"primaryKey" json:"id"`
	InternshipCaseID    uint       `gorm:"not null;uniqueIndex:idx_weekly_report_case_week" json:"internshipCaseId"`
	WeekNumber          int        `gorm:"not null;uniqueIndex:idx_weekly_report_case_week;check:week_number >= 1 AND week_number <= 8" json:"weekNumber"`
	StartDate           time.Time  `gorm:"type:date;not null" json:"startDate"`
	EndDate             time.Time  `gorm:"type:date;not null" json:"endDate"`
	ActivityDescription string     `gorm:"type:text;not null" json:"activityDescription"`
	SubmittedAt         time.Time  `gorm:"not null" json:"submittedAt"`
	IsConfirmed         bool       `gorm:"not null;default:false" json:"isConfirmed"`
	SupervisorComment   *string    `gorm:"type:text" json:"supervisorComment"`
	ConfirmedAt         *time.Time `json:"confirmedAt"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

type CompanyEvaluation struct {
	ID                       uint             `gorm:"primaryKey" json:"id"`
	InternshipCaseID         uint             `gorm:"not null;uniqueIndex" json:"internshipCaseId"`
	CompanySupervisorID      uint             `gorm:"not null;index" json:"companySupervisorId"`
	AttendanceRating         EvaluationRating `gorm:"type:varchar(16);not null" json:"attendanceRating"`
	ParticipationRating      EvaluationRating `gorm:"type:varchar(16);not null" json:"participationRating"`
	LearningRating           EvaluationRating `gorm:"type:varchar(16);not null" json:"learningRating"`
	InterestRating           EvaluationRating `gorm:"type:varchar(16);not null" json:"interestRating"`
	PersistenceRating        EvaluationRating `gorm:"type:varchar(16);not null" json:"persistenceRating"`
	SuggestionRating         EvaluationRating `gorm:"type:varchar(16);not null" json:"suggestionRating"`
	ResourceUsageRating      EvaluationRating `gorm:"type:varchar(16);not null" json:"resourceUsageRating"`
	ReportQualityRating      EvaluationRating `gorm:"type:varchar(16);not null" json:"reportQualityRating"`
	ProjectPerformanceRating EvaluationRating `gorm:"type:varchar(16);not null" json:"projectPerformanceRating"`
	LeaveDays                int              `gorm:"not null;check:leave_days >= 0" json:"leaveDays"`
	AbsenceDays              int              `gorm:"not null;check:absence_days >= 0" json:"absenceDays"`
	Suggestions              *string          `gorm:"type:text" json:"suggestions"`
	SubmittedAt              time.Time        `gorm:"not null" json:"submittedAt"`
	CreatedAt                time.Time        `json:"createdAt"`
	UpdatedAt                time.Time        `json:"updatedAt"`
}

type File struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	OriginalName string    `gorm:"size:500;not null" json:"originalName"`
	StoredName   string    `gorm:"size:255;uniqueIndex;not null" json:"-"`
	Path         string    `gorm:"size:1000;not null" json:"-"`
	MimeType     string    `gorm:"size:150;not null" json:"mimeType"`
	SizeBytes    int64     `gorm:"not null" json:"sizeBytes"`
	UploadedBy   uint      `gorm:"not null;index" json:"uploadedBy"`
	UploadedAt   time.Time `gorm:"not null" json:"uploadedAt"`
}
