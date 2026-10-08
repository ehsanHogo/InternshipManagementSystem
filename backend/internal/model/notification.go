package model

import "time"

type NotificationType string

const (
	NotificationStudentStatus               NotificationType = "STUDENT_STATUS_CHANGED"
	NotificationAccountStatus               NotificationType = "ACCOUNT_STATUS_CHANGED"
	NotificationProfessorAction             NotificationType = "PROFESSOR_ACTION_REQUIRED"
	NotificationCompanyApplications         NotificationType = "COMPANY_APPLICATIONS_ACTION_REQUIRED"
	NotificationCompanyPlacement            NotificationType = "COMPANY_PLACEMENT_ACTION_REQUIRED"
	NotificationCompanyWeeklyReports        NotificationType = "COMPANY_WEEKLY_REPORTS_ACTION_REQUIRED"
	NotificationUniversityCaseReview        NotificationType = "UNIVERSITY_CASE_REVIEW_ACTION_REQUIRED"
	NotificationUniversityPlacementReview   NotificationType = "UNIVERSITY_PLACEMENT_REVIEW_ACTION_REQUIRED"
	NotificationAdminCompanyRegistration    NotificationType = "ADMIN_COMPANY_REGISTRATION_ACTION_REQUIRED"
	NotificationAdminUniversityVerification NotificationType = "ADMIN_UNIVERSITY_VERIFICATION_ACTION_REQUIRED"
)

// The partial unique index also prevents duplicate unread reviewer alerts when
// independent business transactions create work concurrently.
type Notification struct {
	ID         uint             `gorm:"primaryKey" json:"id"`
	UserID     uint             `gorm:"not null;index:idx_notifications_user_created,priority:1;index:idx_notifications_user_read,priority:1;uniqueIndex:idx_notifications_unread_action,where:is_read = false AND type LIKE '%ACTION_REQUIRED'" json:"-"`
	User       User             `gorm:"foreignKey:UserID;constraint:OnDelete:RESTRICT" json:"-"`
	Type       NotificationType `gorm:"type:varchar(64);not null;uniqueIndex:idx_notifications_unread_action" json:"type"`
	Title      string           `gorm:"size:200;not null" json:"title"`
	Message    string           `gorm:"type:text;not null" json:"message"`
	IsRead     bool             `gorm:"not null;default:false;index:idx_notifications_user_read,priority:2" json:"isRead"`
	CreatedAt  time.Time        `gorm:"not null;index:idx_notifications_user_created,priority:2,sort:desc" json:"createdAt"`
	ActionPath string           `gorm:"size:300" json:"actionPath,omitempty"`
}
