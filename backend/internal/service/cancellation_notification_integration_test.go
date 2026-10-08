package service

import (
	"errors"
	"testing"

	"gorm.io/gorm"
	"internship-management-system/backend/internal/model"
)

func TestCancellationNotificationFailureRollsBack(t *testing.T) {
	db := termTestDatabase(t)
	var student, professor, university model.User
	for role, target := range map[model.Role]*model.User{
		model.RoleStudent: &student, model.RoleProfessor: &professor, model.RoleUniversitySupervisor: &university,
	} {
		if err := db.Where("role = ?", role).First(target).Error; err != nil {
			t.Fatal(err)
		}
	}
	terms := NewInternshipTermService(db)
	term, err := terms.Create(university.ID, 1405, model.InternshipTermTypeSummer)
	if err != nil {
		t.Fatal(err)
	}
	item := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, TermID: &term.ID, Status: model.InternshipCaseStatusPendingUniversityReview}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	failure := errors.New("injected cancellation notification failure")
	const callback = "m24_cancellation_notification_failure"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "notifications" {
			tx.AddError(failure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Callback().Create().Remove(callback) })
	if _, err := NewInternshipService(db).CancelUniversityReview(item.ID, "لغو"); !errors.Is(err, failure) {
		t.Fatalf("manual cancellation should fail: %v", err)
	}
	if _, err := terms.Close(university.ID, term.ID); !errors.Is(err, failure) {
		t.Fatalf("term closure should fail: %v", err)
	}
	var stored model.InternshipCase
	if err := db.First(&stored, item.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != item.Status || stored.CancelledAt != nil || stored.CancellationComment != nil {
		t.Fatal("failed notification left a cancelled case")
	}
	var storedTerm model.InternshipTerm
	if err := db.First(&storedTerm, term.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedTerm.Status != model.InternshipTermStatusOpen || storedTerm.ClosedAt != nil || storedTerm.ClosedBy != nil {
		t.Fatal("failed notification left a closed term")
	}
	var count int64
	if err := db.Model(&model.Notification{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("rolled-back cancellation left a notification")
	}
}
