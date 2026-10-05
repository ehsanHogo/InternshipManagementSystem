package service

import (
	"errors"
	"sync"
	"testing"

	"internship-management-system/backend/internal/model"
)

func TestConcurrentAdminDisableAndVerificationReview(t *testing.T) {
	db := termTestDatabase(t)
	var first model.User
	if err := db.Where("role = ?", model.RoleAdmin).First(&first).Error; err != nil {
		t.Fatal(err)
	}
	second := model.User{FullName: "Other Admin", Email: "m22-other-admin@example.test", Role: model.RoleAdmin, PasswordHash: "fixture"}
	if err := db.Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	s := NewUserAccessService(db)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, pair := range [][2]uint{{first.ID, second.ID}, {second.ID, first.ID}} {
		wg.Add(1)
		go func(actor, target uint) {
			defer wg.Done()
			<-start
			_, err := s.SetActive(actor, target, false)
			results <- err
		}(pair[0], pair[1])
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrUserAccessForbidden) && !errors.Is(err, ErrLastActiveAdmin) {
			t.Fatal(err)
		}
	}
	var count int64
	if err := db.Model(&model.User{}).Where("role = ? AND is_active = true", model.RoleAdmin).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 || successes != 1 {
		t.Fatalf("concurrent disables left %d active admins, %d successes", count, successes)
	}
	// Defense in depth: a transaction snapshot still forbids disabling the sole
	// Admin even if a new request raced with an earlier disable after authentication.
	var active model.User
	if err := db.Where("role = ? AND is_active = true", model.RoleAdmin).First(&active).Error; err != nil {
		t.Fatal(err)
	}
	// Self-disable has a dedicated error and also covers the single-Admin case.
	if _, err := s.SetActive(active.ID, active.ID, false); !errors.Is(err, ErrAdminSelfDisable) {
		t.Fatalf("self-disable: %v", err)
	}
	pending, err := s.Register(UniversityRegistrationInput{FullName: "Pending", Email: "m22-review@example.test", Password: "Secret123!"})
	if err != nil {
		t.Fatal(err)
	}
	start = make(chan struct{})
	results = make(chan error, 2)
	for _, status := range []model.UserVerificationStatus{model.UserVerificationApproved, model.UserVerificationRejected} {
		wg.Add(1)
		go func(status model.UserVerificationStatus) {
			defer wg.Done()
			<-start
			_, err := s.Review(active.ID, pending.ID, status, "Reason")
			results <- err
		}(status)
	}
	close(start)
	wg.Wait()
	close(results)
	successes = 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrVerificationTransition) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent review succeeded %d times", successes)
	}
}
