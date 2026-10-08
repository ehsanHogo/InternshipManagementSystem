package main

import (
	"fmt"
	"testing"
	"time"

	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

func TestNotificationRegistrationAndVerificationEvents(t *testing.T) {
	db := registrationTestDB(t)
	admin := registrationDemoUser(t, db, "admin")
	suffix := time.Now().UnixNano()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	notifications := service.NewNotificationService(db)
	actionCount := func(kind model.NotificationType, want int64) {
		t.Helper()
		var count int64
		must(db.Model(&model.Notification{}).Where("user_id = ? AND type = ?", admin.ID, kind).Count(&count).Error)
		if count != want {
			t.Fatalf("admin action alerts %d want %d", count, want)
		}
	}
	companyService := service.NewCompanyAccountService(db)
	input := service.CompanyRegistrationInput{
		Company:    service.CompanyInput{Name: fmt.Sprintf("m23-%d", suffix), NationalID: fmt.Sprint(suffix), EconomicCode: fmt.Sprintf("e%d", suffix), Address: "Tehran", Phone: "02112345678", Email: fmt.Sprintf("office-%d@test.local", suffix)},
		Supervisor: service.CompanySupervisorRegistrationInput{FullName: "supervisor", Email: fmt.Sprintf("m23-company-%d@test.local", suffix), Phone: "09120000000", JobTitle: "manager", Password: "Secret123!"},
	}
	account, err := companyService.Register(input)
	must(err)
	actionCount(model.NotificationAdminCompanyRegistration, 1)
	access := service.NewUserAccessService(db)
	university, err := access.Register(service.UniversityRegistrationInput{FullName: "university", Email: fmt.Sprintf("m23-university-%d@test.local", suffix), Password: "Secret123!"})
	must(err)
	actionCount(model.NotificationAdminUniversityVerification, 1)
	registration := service.NewCompanyRegistrationService(db)
	_, err = registration.Review(admin.ID, account.Company.ID, model.CompanyRegistrationStatusRejected, "اصلاح اطلاعات")
	must(err)
	_, err = access.Review(admin.ID, university.ID, model.UserVerificationRejected, "اصلاح اطلاعات")
	must(err)
	for _, id := range []uint{account.Supervisor.ID, university.ID} {
		list, err := notifications.List(id)
		must(err)
		if len(list) != 1 || list[0].Type != model.NotificationAccountStatus || list[0].IsRead {
			t.Fatalf("missing personal rejection: %+v", list)
		}
	}
	list, err := notifications.List(admin.ID)
	must(err)
	ids := []uint{}
	for _, n := range list {
		ids = append(ids, n.ID)
	}
	must(notifications.MarkViewed(admin.ID, ids))
	_, err = companyService.Resubmit(account.Supervisor.ID)
	must(err)
	actionCount(model.NotificationAdminCompanyRegistration, 2)
	_, err = access.Resubmit(university.ID)
	must(err)
	actionCount(model.NotificationAdminUniversityVerification, 2)
	_, err = registration.Review(admin.ID, account.Company.ID, model.CompanyRegistrationStatusApproved, "")
	must(err)
	_, err = access.Review(admin.ID, university.ID, model.UserVerificationApproved, "")
	must(err)
	for _, id := range []uint{account.Supervisor.ID, university.ID} {
		list, err := notifications.List(id)
		must(err)
		if len(list) != 2 || list[0].Type != model.NotificationAccountStatus || list[0].IsRead {
			t.Fatalf("missing personal approval: %+v", list)
		}
	}
	// Repeated invalid submissions/reviews do not create any extra notification.
	_, err = companyService.Resubmit(account.Supervisor.ID)
	if err == nil {
		t.Fatal("approved company resubmitted")
	}
	_, err = access.Resubmit(university.ID)
	if err == nil {
		t.Fatal("approved university resubmitted")
	}
	_, err = registration.Review(admin.ID, account.Company.ID, model.CompanyRegistrationStatusApproved, "")
	if err == nil {
		t.Fatal("company reviewed twice")
	}
	_, err = access.Review(admin.ID, university.ID, model.UserVerificationApproved, "")
	if err == nil {
		t.Fatal("university reviewed twice")
	}
	actionCount(model.NotificationAdminCompanyRegistration, 2)
	actionCount(model.NotificationAdminUniversityVerification, 2)
}
