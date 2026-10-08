package service

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"internship-management-system/backend/internal/model"
)

const NotificationListLimit = 100

var ErrInvalidNotificationIDs = errors.New("invalid notification IDs")

type NotificationService struct{ db *gorm.DB }

func NewNotificationService(db *gorm.DB) *NotificationService { return &NotificationService{db: db} }

func (s *NotificationService) List(userID uint) ([]model.Notification, error) {
	notifications := []model.Notification{}
	err := s.db.Where("user_id = ?", userID).Order("created_at DESC, id DESC").Limit(NotificationListLimit).Find(&notifications).Error
	return notifications, err
}
func (s *NotificationService) UnreadCount(userID uint) (int64, error) {
	var count int64
	err := s.db.Model(&model.Notification{}).Where("user_id = ? AND is_read = false", userID).Count(&count).Error
	return count, err
}

// Unknown and foreign IDs are ignored. Ownership is part of the UPDATE predicate;
// a newly arrived notification absent from this snapshot is never consumed.
func (s *NotificationService) MarkViewed(userID uint, ids []uint) error {
	if len(ids) > NotificationListLimit {
		return ErrInvalidNotificationIDs
	}
	for _, id := range ids {
		if id == 0 {
			return ErrInvalidNotificationIDs
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return s.db.Model(&model.Notification{}).Where("user_id = ? AND id IN ? AND is_read = false", userID, ids).Update("is_read", true).Error
}

// Call creation helpers with the current business transaction, so notification
// failures roll back the state change and rejected transitions produce no alert.
func (s *NotificationService) CreateStudentNotification(userID uint, title, message, path string) error {
	return s.personal(userID, model.NotificationStudentStatus, title, message, path)
}
func (s *NotificationService) personal(userID uint, kind model.NotificationType, title, message, path string) error {
	return s.db.Create(&model.Notification{UserID: userID, Type: kind, Title: title, Message: message, ActionPath: path}).Error
}
func (s *NotificationService) action(userID uint, kind model.NotificationType, title, message, path string) error {
	var count int64
	if err := s.db.Model(&model.Notification{}).Where("user_id = ? AND type = ? AND is_read = false", userID, kind).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	notification := model.Notification{UserID: userID, Type: kind, Title: title, Message: message, ActionPath: path}
	return s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&notification).Error
}
func (s *NotificationService) notifyUsers(query *gorm.DB, kind model.NotificationType, title, message, path string) error {
	var ids []uint
	if err := query.Model(&model.User{}).Where("is_active = true").Order("id").Pluck("id", &ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.action(id, kind, title, message, path); err != nil {
			return err
		}
	}
	return nil
}

// Match M17 operational eligibility at notification delivery time, including
// when an older assigned case still points to a now restricted company user.
func (s *NotificationService) companyRecipients() *gorm.DB {
	return s.db.Where("role = ?", model.RoleCompanySupervisor).
		Where("EXISTS (SELECT 1 FROM companies WHERE companies.id = users.company_id AND companies.registration_status = ?)", model.CompanyRegistrationStatusApproved)
}
func (s *NotificationService) NotifyCompanyApplications(companyID uint) error {
	return s.notifyUsers(s.companyRecipients().Where("company_id = ?", companyID), model.NotificationCompanyApplications,
		"درخواست جدید کارآموزی", "درخواست‌های جدیدی برای فرصت‌های کارآموزی شما ثبت شده است و نیاز به بررسی دارد.", "/company/opportunities")
}
func (s *NotificationService) NotifyCompanyPlacement(supervisorID uint, correction bool) error {
	title, message := "تکمیل اطلاعات کارآموزی", "پرونده‌های کارآموزی جدیدی نیازمند تکمیل اطلاعات محل کارآموزی هستند."
	if correction {
		title, message = "اصلاح اطلاعات کارآموزی", "برخی پرونده‌های کارآموزی برای اصلاح اطلاعات محل کارآموزی بازگردانده شده‌اند."
	}
	return s.notifyUsers(s.companyRecipients().Where("id = ?", supervisorID), model.NotificationCompanyPlacement,
		title, message, "/company/internships?status=PENDING_COMPANY_DETAILS")
}
func (s *NotificationService) NotifyCompanyWeeklyReports(supervisorID uint) error {
	return s.notifyUsers(s.companyRecipients().Where("id = ?", supervisorID), model.NotificationCompanyWeeklyReports,
		"گزارش هفتگی جدید", "گزارش‌های هفتگی جدیدی برای بررسی و تأیید شما وجود دارد.", "/company/internships?status=ACTIVE")
}
func (s *NotificationService) NotifyProfessorActionRequired(professorID uint) error {
	return s.notifyUsers(s.db.Where("id = ? AND role = ?", professorID, model.RoleProfessor), model.NotificationProfessorAction,
		"نیاز به بررسی", "گزارش‌های جدیدی برای بررسی شما وجود دارد.", "/professor/internships")
}
func (s *NotificationService) universityRecipients() *gorm.DB {
	return s.db.Where("role = ? AND verification_status = ?", model.RoleUniversitySupervisor, model.UserVerificationApproved)
}
func (s *NotificationService) NotifyUniversityCaseReview() error {
	return s.notifyUsers(s.universityRecipients(), model.NotificationUniversityCaseReview,
		"درخواست کارآموزی جدید", "درخواست‌های کارآموزی جدیدی برای بررسی و تعیین محل کارآموزی وجود دارد.", "/university/applications?status=PENDING_UNIVERSITY_REVIEW")
}
func (s *NotificationService) NotifyUniversityPlacementReview() error {
	return s.notifyUsers(s.universityRecipients(), model.NotificationUniversityPlacementReview,
		"نیاز به تأیید نهایی", "اطلاعات محل کارآموزی پرونده‌های جدیدی ثبت شده و نیازمند بررسی نهایی است.", "/university/applications?status=PENDING_FINAL_APPROVAL")
}
func (s *NotificationService) NotifyAdminCompanyRegistration() error {
	return s.notifyUsers(s.db.Where("role = ?", model.RoleAdmin), model.NotificationAdminCompanyRegistration,
		"ثبت شرکت جدید", "ثبت‌نام‌های جدید شرکت‌ها نیازمند بررسی است.", "/admin/company-registrations")
}
func (s *NotificationService) NotifyAdminUniversityVerification() error {
	return s.notifyUsers(s.db.Where("role = ?", model.RoleAdmin), model.NotificationAdminUniversityVerification,
		"درخواست احراز هویت جدید", "درخواست‌های جدید احراز هویت مسئول آموزش نیازمند بررسی است.", "/admin/user-verifications")
}
func (s *NotificationService) NotifyWeeklyReportSubmitted(caseID uint) error {
	var item model.InternshipCase
	if err := s.db.First(&item, caseID).Error; err != nil {
		return err
	}
	if item.CompanySupervisorID != nil {
		if err := s.NotifyCompanyWeeklyReports(*item.CompanySupervisorID); err != nil {
			return err
		}
	}
	return s.NotifyProfessorActionRequired(item.ProfessorID)
}
func (s *NotificationService) NotifyWeeklyReportReviewed(studentID uint, week int, role model.Role, approved bool) error {
	reviewer, result := "استاد", "نیازمند اصلاح است"
	if role == model.RoleCompanySupervisor {
		reviewer = "سرپرست شرکت"
	}
	if approved {
		result = "تأیید شد"
	}
	digits := []rune("۰۱۲۳۴۵۶۷۸۹")
	return s.CreateStudentNotification(studentID, "نتیجه بررسی گزارش هفتگی", fmt.Sprintf("گزارش هفتگی هفته %c توسط %s بررسی شد: %s.", digits[week], reviewer, result), "/student/weekly-reports")
}
func (s *NotificationService) NotifyCompanyRegistrationResult(companyID uint, approved bool) error {
	var ids []uint
	if err := s.db.Model(&model.User{}).Where("company_id = ? AND role = ?", companyID, model.RoleCompanySupervisor).Order("id").Pluck("id", &ids).Error; err != nil {
		return err
	}
	message := "ثبت‌نام شرکت شما رد شد. دلیل رد را در پروفایل شرکت مشاهده کنید."
	if approved {
		message = "ثبت‌نام شرکت شما تأیید شد."
	}
	for _, id := range ids {
		if err := s.personal(id, model.NotificationAccountStatus, "نتیجه بررسی ثبت شرکت", message, "/company/profile"); err != nil {
			return err
		}
	}
	return nil
}
func (s *NotificationService) NotifyVerificationResult(userID uint, approved bool) error {
	message := "درخواست احراز هویت شما رد شد. دلیل رد را در صفحه وضعیت مشاهده کنید."
	if approved {
		message = "احراز هویت شما تأیید شد."
	}
	return s.personal(userID, model.NotificationAccountStatus, "نتیجه بررسی احراز هویت", message, "/university-supervisor/verification")
}
