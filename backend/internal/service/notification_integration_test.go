package service

import (
	"errors"
	"sync"
	"testing"

	"gorm.io/gorm"
	"internship-management-system/backend/internal/model"
)

func TestActionNotificationsDeduplicateConcurrentlyAndAfterRead(t *testing.T) {
	db := termTestDatabase(t)
	s := NewNotificationService(db)
	user := func(role model.Role) model.User {
		t.Helper()
		var u model.User
		if err := db.Where("role = ?", role).First(&u).Error; err != nil {
			t.Fatal(err)
		}
		return u
	}
	company, professor, university, admin := user(model.RoleCompanySupervisor), user(model.RoleProfessor), user(model.RoleUniversitySupervisor), user(model.RoleAdmin)
	for _, test := range []struct {
		name      string
		recipient uint
		kind      model.NotificationType
		path      string
		notify    func(*NotificationService) error
	}{
		{"company-applications", company.ID, model.NotificationCompanyApplications, "/company/opportunities", func(s *NotificationService) error { return s.NotifyCompanyApplications(*company.CompanyID) }},
		{"company-placement", company.ID, model.NotificationCompanyPlacement, "/company/internships?status=PENDING_COMPANY_DETAILS", func(s *NotificationService) error { return s.NotifyCompanyPlacement(company.ID, false) }},
		{"company-weekly", company.ID, model.NotificationCompanyWeeklyReports, "/company/internships?status=ACTIVE", func(s *NotificationService) error { return s.NotifyCompanyWeeklyReports(company.ID) }},
		{"professor", professor.ID, model.NotificationProfessorAction, "/professor/internships", func(s *NotificationService) error { return s.NotifyProfessorActionRequired(professor.ID) }},
		{"university-case", university.ID, model.NotificationUniversityCaseReview, "/university/applications?status=PENDING_UNIVERSITY_REVIEW", func(s *NotificationService) error { return s.NotifyUniversityCaseReview() }},
		{"university-placement", university.ID, model.NotificationUniversityPlacementReview, "/university/applications?status=PENDING_FINAL_APPROVAL", func(s *NotificationService) error { return s.NotifyUniversityPlacementReview() }},
		{"admin-company", admin.ID, model.NotificationAdminCompanyRegistration, "/admin/company-registrations", func(s *NotificationService) error { return s.NotifyAdminCompanyRegistration() }},
		{"admin-verification", admin.ID, model.NotificationAdminUniversityVerification, "/admin/user-verifications", func(s *NotificationService) error { return s.NotifyAdminUniversityVerification() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			categoryNotifications := func() ([]model.Notification, error) {
				list := []model.Notification{}
				err := db.Where("user_id = ? AND type = ?", test.recipient, test.kind).Order("id DESC").Find(&list).Error
				return list, err
			}
			var wg sync.WaitGroup
			start := make(chan struct{})
			results := make(chan error, 12)
			for i := 0; i < 12; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					results <- db.Transaction(func(tx *gorm.DB) error { return test.notify(NewNotificationService(tx)) })
				}()
			}
			close(start)
			wg.Wait()
			close(results)
			for err := range results {
				if err != nil {
					t.Fatal(err)
				}
			}
			list, err := categoryNotifications()
			if err != nil {
				t.Fatal(err)
			}
			if len(list) != 1 || list[0].Type != test.kind || list[0].IsRead || list[0].ActionPath != test.path {
				t.Fatalf("duplicate/wrong notification: %+v", list)
			}
			if err := test.notify(s); err != nil {
				t.Fatal(err)
			}
			if err := s.MarkViewed(test.recipient, []uint{list[0].ID}); err != nil {
				t.Fatal(err)
			}
			if err := test.notify(s); err != nil {
				t.Fatal(err)
			}
			list, err = categoryNotifications()
			if err != nil {
				t.Fatal(err)
			}
			if len(list) != 2 || list[0].IsRead || !list[1].IsRead {
				t.Fatalf("post-view new event: %+v", list)
			}
		})
	}
	// Different unread action categories coexist for the same reviewer.
	for _, test := range []struct {
		userID uint
		want   int64
	}{{company.ID, 3}, {university.ID, 2}, {admin.ID, 2}} {
		count, err := s.UnreadCount(test.userID)
		if err != nil || count != test.want {
			t.Fatalf("category coexistence: count %d want %d err %v", count, test.want, err)
		}
	}
	// Corrections use the same placement slot even when an applicant/report alert exists.
	if err := s.NotifyCompanyPlacement(company.ID, true); err != nil {
		t.Fatal(err)
	}
	countUnread, err := s.UnreadCount(company.ID)
	if err != nil || countUnread != 3 {
		t.Fatalf("correction duplicated placement: %d, %v", countUnread, err)
	}
	// Personal notifications are never aggregated, even when both remain unread.
	if err := s.CreateStudentNotification(company.ID, "personal", "one", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateStudentNotification(company.ID, "personal", "two", ""); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.Notification{}).Where("user_id = ? AND type = ?", company.ID, model.NotificationStudentStatus).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("personal notifications aggregated: %d", count)
	}
}

func TestActionNotificationRecipientEligibility(t *testing.T) {
	db := termTestDatabase(t)
	var users []model.User
	for i, status := range []model.UserVerificationStatus{model.UserVerificationPending, model.UserVerificationRejected, model.UserVerificationApproved, model.UserVerificationApproved} {
		u := model.User{FullName: "reviewer", Email: []string{"pending@test.local", "rejected@test.local", "approved@test.local", "disabled@test.local"}[i], Role: model.RoleUniversitySupervisor, PasswordHash: "fixture", VerificationStatus: status}
		if err := db.Create(&u).Error; err != nil {
			t.Fatal(err)
		}
		if i == 3 {
			if err := db.Model(&u).Update("is_active", false).Error; err != nil {
				t.Fatal(err)
			}
		}
		users = append(users, u)
	}
	admin := model.User{FullName: "disabled admin", Email: "disabled-admin@test.local", Role: model.RoleAdmin, PasswordHash: "fixture"}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&admin).Update("is_active", false).Error; err != nil {
		t.Fatal(err)
	}
	s := NewNotificationService(db)
	if err := s.NotifyUniversityCaseReview(); err != nil {
		t.Fatal(err)
	}
	if err := s.NotifyAdminCompanyRegistration(); err != nil {
		t.Fatal(err)
	}
	if err := s.NotifyUniversityPlacementReview(); err != nil {
		t.Fatal(err)
	}
	if err := s.NotifyAdminUniversityVerification(); err != nil {
		t.Fatal(err)
	}
	for i, u := range append(users, admin) {
		count, err := s.UnreadCount(u.ID)
		if err != nil {
			t.Fatal(err)
		}
		want := int64(0)
		if i == 2 {
			want = 2
		}
		if count != want {
			t.Fatalf("recipient %d got %d want %d", i, count, want)
		}
	}
}

func TestNotificationFailureRollsBackBusinessOperation(t *testing.T) {
	db := termTestDatabase(t)
	var student, company model.User
	if err := db.Where("role = ?", model.RoleStudent).First(&student).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("role = ?", model.RoleCompanySupervisor).First(&company).Error; err != nil {
		t.Fatal(err)
	}
	opportunity, err := NewOpportunityService(db).Create(company.ID, OpportunityInput{Title: "test", Description: "test", Location: "test", WorkField: "test"})
	if err != nil {
		t.Fatal(err)
	}
	fail := errors.New("notification insert failed")
	callback := "m23_notification_failure"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "notifications" {
			tx.AddError(fail)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Callback().Create().Remove(callback) })
	applications := NewOpportunityApplicationService(db)
	resume := &model.File{OriginalName: "resume.pdf", StoredName: "test.pdf", Path: "/tmp/test.pdf", MimeType: "application/pdf", SizeBytes: 20}
	_, err = applications.Apply(student.ID, opportunity.ID, resume)
	if !errors.Is(err, fail) {
		t.Fatalf("missing injected failure: %v", err)
	}
	for _, table := range []any{&model.OpportunityApplication{}, &model.Notification{}} {
		var count int64
		if err := db.Model(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("failed operation left %T rows: %d", table, count)
		}
	}
	var count int64
	if err := db.Model(&model.File{}).Where("id = ?", resume.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("resume metadata committed despite rollback")
	}
	db.Callback().Create().Remove(callback)
	// A broader enclosing transaction can also roll back both after notification creation.
	err = db.Transaction(func(tx *gorm.DB) error {
		resume.ID = 0
		if _, err := NewOpportunityApplicationService(tx).Apply(student.ID, opportunity.ID, resume); err != nil {
			return err
		}
		return fail
	})
	if !errors.Is(err, fail) {
		t.Fatal(err)
	}
	if err := db.Model(&model.Notification{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("outer rollback left notification")
	}
}

func TestCompanyActionNotificationsRequireOperationalAccess(t *testing.T) {
	db := termTestDatabase(t)
	s := NewNotificationService(db)
	for _, status := range []model.CompanyRegistrationStatus{model.CompanyRegistrationStatusPending, model.CompanyRegistrationStatusRejected, model.CompanyRegistrationStatusApproved} {
		t.Run(string(status), func(t *testing.T) {
			company := model.Company{Name: string(status), NationalID: string(status), EconomicCode: string(status), RegistrationStatus: status}
			if err := db.Create(&company).Error; err != nil {
				t.Fatal(err)
			}
			user := model.User{FullName: string(status), Email: string(status) + "@test.local", Role: model.RoleCompanySupervisor, CompanyID: &company.ID, PasswordHash: "fixture"}
			if err := db.Create(&user).Error; err != nil {
				t.Fatal(err)
			}
			for _, notify := range []func() error{
				func() error { return s.NotifyCompanyApplications(company.ID) },
				func() error { return s.NotifyCompanyPlacement(user.ID, false) },
				func() error { return s.NotifyCompanyPlacement(user.ID, true) },
				func() error { return s.NotifyCompanyWeeklyReports(user.ID) },
			} {
				if err := notify(); err != nil {
					t.Fatal(err)
				}
			}
			count, err := s.UnreadCount(user.ID)
			if err != nil {
				t.Fatal(err)
			}
			want := int64(0)
			if status == model.CompanyRegistrationStatusApproved {
				want = 3
			}
			if count != want {
				t.Fatalf("company %s got %d want %d", status, count, want)
			}
			if status == model.CompanyRegistrationStatusApproved {
				// Disabling or losing membership prevents all later operational alerts.
				list, err := s.List(user.ID)
				if err != nil {
					t.Fatal(err)
				}
				ids := []uint{}
				for _, n := range list {
					ids = append(ids, n.ID)
				}
				if err := s.MarkViewed(user.ID, ids); err != nil {
					t.Fatal(err)
				}
				for _, updates := range []map[string]any{{"is_active": false}, {"is_active": true, "company_id": nil}} {
					if err := db.Model(&user).Updates(updates).Error; err != nil {
						t.Fatal(err)
					}
					if err := s.NotifyCompanyApplications(company.ID); err != nil {
						t.Fatal(err)
					}
					if err := s.NotifyCompanyPlacement(user.ID, true); err != nil {
						t.Fatal(err)
					}
					if err := s.NotifyCompanyWeeklyReports(user.ID); err != nil {
						t.Fatal(err)
					}
					count, err := s.UnreadCount(user.ID)
					if err != nil || count != 0 {
						t.Fatalf("ineligible supervisor notified: %d %v", count, err)
					}
				}
			}
		})
	}
	var unrelated model.User
	if err := db.Where("email = ?", "company@demo.local").First(&unrelated).Error; err != nil {
		t.Fatal(err)
	}
	count, err := s.UnreadCount(unrelated.ID)
	if err != nil || count != 0 {
		t.Fatalf("unrelated company notified: %d %v", count, err)
	}
}
