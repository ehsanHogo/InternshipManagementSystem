package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"internship-management-system/backend/internal/model"
)

// Cancellation is a personal workflow change, including automatic term closure.
// Exercise the production routes, not just notification helper functions.
func TestAcceptanceCancellationNotifications(t *testing.T) {
	db := registrationTestDB(t)
	router := registrationRouter(t, db)
	university := registrationDemoUser(t, db, "university")
	professor := registrationDemoUser(t, db, "professor")
	token := registrationToken(t, university)
	term := registrationDecode[model.InternshipTerm](t, registrationRequest(t, router, "POST", "/api/university/internship-terms", token,
		map[string]any{"academicYear": 1405, "termType": "SUMMER"}, 201))
	newCase := func(status model.InternshipCaseStatus) (model.User, model.InternshipCase) {
		t.Helper()
		student := model.User{FullName: "دانشجوی آزمون لغو", Email: fmt.Sprintf("m24-cancel-%d@example.test", time.Now().UnixNano()), Role: model.RoleStudent, PasswordHash: "fixture"}
		if err := db.Create(&student).Error; err != nil {
			t.Fatal(err)
		}
		item := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, TermID: &term.ID, Status: status}
		if err := db.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
		return student, item
	}
	assertNotification := func(student model.User, want int, message string) {
		t.Helper()
		list := registrationDecode[[]model.Notification](t, registrationRequest(t, router, "GET", "/api/notifications", registrationToken(t, student), nil, 200))
		if len(list) != want {
			t.Fatalf("cancellation notifications=%d want %d", len(list), want)
		}
		if want > 0 && (list[0].Type != model.NotificationStudentStatus || list[0].ActionPath != "/student/application" || !strings.Contains(list[0].Message, message)) {
			t.Fatalf("incorrect cancellation notification: %+v", list[0])
		}
	}

	student, item := newCase(model.InternshipCaseStatusPendingUniversityReview)
	path := fmt.Sprintf("/api/university/internship-cases/%d/cancel", item.ID)
	registrationRequest(t, router, "POST", path, token, map[string]any{"comment": " "}, 400)
	assertNotification(student, 0, "")
	registrationRequest(t, router, "POST", path, token, map[string]any{"comment": "لغو به درخواست دانشجو"}, 200)
	assertNotification(student, 1, "لغو")
	registrationRequest(t, router, "POST", path, token, map[string]any{"comment": "تکرار"}, 409)
	assertNotification(student, 1, "لغو")

	statuses := []model.InternshipCaseStatus{model.InternshipCaseStatusDraft, model.InternshipCaseStatusPendingUniversityReview,
		model.InternshipCaseStatusRevisionRequested, model.InternshipCaseStatusPendingCompanyDetails,
		model.InternshipCaseStatusPendingFinalApproval, model.InternshipCaseStatusActive,
		model.InternshipCaseStatusPassed, model.InternshipCaseStatusFailed, model.InternshipCaseStatusCancelled}
	students := make([]model.User, len(statuses))
	for i, status := range statuses {
		students[i], _ = newCase(status)
	}
	closePath := fmt.Sprintf("/api/university/internship-terms/%d/close", term.ID)
	registrationRequest(t, router, "POST", closePath, token, nil, 200)
	for i, status := range statuses {
		want := 1
		if status.Terminal() {
			want = 0
		}
		assertNotification(students[i], want, "بسته شدن ترم")
	}
	// Previously cancelled cases and rejected repeated closes produce no new alert.
	assertNotification(student, 1, "لغو")
	registrationRequest(t, router, "POST", closePath, token, nil, 409)
	for i, status := range statuses {
		want := 1
		if status.Terminal() {
			want = 0
		}
		assertNotification(students[i], want, "بسته شدن ترم")
	}
}
