package model

import (
	"encoding/json"
	"time"
)

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

type WeeklyReviewStatus string

const (
	WeeklyReviewPending           WeeklyReviewStatus = "PENDING"
	WeeklyReviewApproved          WeeklyReviewStatus = "APPROVED"
	WeeklyReviewRevisionRequested WeeklyReviewStatus = "REVISION_REQUESTED"
)

type WeeklyReportStatus string

const (
	WeeklyReportDraft             WeeklyReportStatus = "DRAFT"
	WeeklyReportSubmitted         WeeklyReportStatus = "SUBMITTED"
	WeeklyReportRevisionRequested WeeklyReportStatus = "REVISION_REQUESTED"
	WeeklyReportApproved          WeeklyReportStatus = "APPROVED"
)

type WeeklyReport struct {
	ID                     uint               `gorm:"primaryKey" json:"id"`
	InternshipCaseID       uint               `gorm:"not null;uniqueIndex:idx_weekly_report_case_week" json:"internshipCaseId"`
	WeekNumber             int                `gorm:"not null;uniqueIndex:idx_weekly_report_case_week;check:week_number >= 1 AND week_number <= 8" json:"weekNumber"`
	StartDate              time.Time          `gorm:"type:date;not null" json:"startDate"`
	EndDate                time.Time          `gorm:"type:date;not null;check:end_date >= start_date" json:"endDate"`
	ActivityDescription    string             `gorm:"type:text;not null" json:"activityDescription"`
	SubmittedAt            *time.Time         `json:"submittedAt"`
	CompanyReviewStatus    WeeklyReviewStatus `gorm:"type:varchar(24);not null;default:PENDING;check:company_review_status IN ('PENDING','APPROVED','REVISION_REQUESTED')" json:"companyReviewStatus"`
	CompanyReviewComment   *string            `gorm:"type:text" json:"companyReviewComment"`
	CompanyReviewedAt      *time.Time         `json:"companyReviewedAt"`
	ProfessorReviewStatus  WeeklyReviewStatus `gorm:"type:varchar(24);not null;default:PENDING;check:professor_review_status IN ('PENDING','APPROVED','REVISION_REQUESTED')" json:"professorReviewStatus"`
	ProfessorReviewComment *string            `gorm:"type:text" json:"professorReviewComment"`
	ProfessorReviewedAt    *time.Time         `json:"professorReviewedAt"`
	CreatedAt              time.Time          `json:"createdAt"`
	UpdatedAt              time.Time          `json:"updatedAt"`
}

func (report WeeklyReport) Status() WeeklyReportStatus {
	if report.SubmittedAt == nil {
		return WeeklyReportDraft
	}
	if report.CompanyReviewStatus == WeeklyReviewRevisionRequested || report.ProfessorReviewStatus == WeeklyReviewRevisionRequested {
		return WeeklyReportRevisionRequested
	}
	if report.CompanyReviewStatus == WeeklyReviewApproved && report.ProfessorReviewStatus == WeeklyReviewApproved {
		return WeeklyReportApproved
	}
	return WeeklyReportSubmitted
}

// Status is computed for every response, including reports nested in case responses.
func (report WeeklyReport) MarshalJSON() ([]byte, error) {
	type fields WeeklyReport
	return json.Marshal(struct {
		fields
		Status WeeklyReportStatus `json:"status"`
	}{fields: fields(report), Status: report.Status()})
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

// WeeklyReportsReadyForFinalEvaluation requires every distinct week 1..8 and
// approvals from both reviewers on submitted content.
func WeeklyReportsReadyForFinalEvaluation(reports []WeeklyReport) bool {
	if len(reports) != 8 {
		return false
	}
	var seen [8]bool
	for _, report := range reports {
		if report.WeekNumber < 1 || report.WeekNumber > 8 || seen[report.WeekNumber-1] || report.SubmittedAt == nil || report.CompanyReviewStatus != WeeklyReviewApproved || report.ProfessorReviewStatus != WeeklyReviewApproved {
			return false
		}
		seen[report.WeekNumber-1] = true
	}
	return true
}
