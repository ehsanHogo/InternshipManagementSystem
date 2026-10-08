package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

func TestNotificationOwnershipAndReadSnapshot(t *testing.T) {
	db := registrationTestDB(t)
	router := registrationRouter(t, db)
	owner := registrationDemoUser(t, db, "student")
	other := registrationDemoUser(t, db, "professor")
	notifications := service.NewNotificationService(db)
	add := func(user uint, message string) model.Notification {
		t.Helper()
		if err := notifications.CreateStudentNotification(user, "اعلان", message, "/student/application"); err != nil {
			t.Fatal(err)
		}
		var item model.Notification
		if err := db.Where("user_id = ?", user).Order("id DESC").First(&item).Error; err != nil {
			t.Fatal(err)
		}
		if item.IsRead {
			t.Fatal("new notification must be unread")
		}
		return item
	}
	first := add(owner.ID, "first")
	unseen := add(owner.ID, "unseen")
	foreign := add(other.ID, "private")
	token := registrationToken(t, owner)
	list := registrationDecode[[]model.Notification](t, registrationRequest(t, router, "GET", "/api/notifications", token, nil, 200))
	if len(list) != 2 || list[0].ID != unseen.ID || list[1].ID != first.ID {
		t.Fatalf("incorrect private newest-first list: %+v", list)
	}
	// GET is deliberately read-only; mark-viewed follows successful rendering.
	count := func(want int64) {
		t.Helper()
		result := registrationDecode[struct{ Count int64 }](t, registrationRequest(t, router, "GET", "/api/notifications/unread-count", token, nil, 200))
		if result.Count != want {
			t.Fatalf("count %d want %d", result.Count, want)
		}
	}
	count(2)
	late := add(owner.ID, "arrived after list")
	for i := 0; i < 2; i++ {
		registrationRequest(t, router, "POST", "/api/notifications/mark-viewed", token, map[string]any{"notificationIds": []uint{first.ID, foreign.ID, 999999999}}, 200)
		count(2)
	}
	for _, item := range []model.Notification{foreign, unseen, late} {
		var stored model.Notification
		if err := db.First(&stored, item.ID).Error; err != nil {
			t.Fatal(err)
		}
		if stored.IsRead {
			t.Fatalf("unsubmitted or foreign ID %d was consumed", item.ID)
		}
	}
	registrationRequest(t, router, "POST", "/api/notifications/mark-viewed", token, map[string]any{"notificationIds": []uint{unseen.ID}}, 200)
	count(1)
	for _, body := range []any{map[string]any{}, map[string]any{"notificationIds": []int{-1}}, map[string]any{"notificationIds": []int{0}}, map[string]any{"notificationIds": []uint{first.ID}, "userId": other.ID}, map[string]any{"notificationIds": make([]uint, 101)}} {
		registrationRequest(t, router, "POST", "/api/notifications/mark-viewed", token, body, 400)
	}
	registrationRequest(t, router, "POST", "/api/notifications/mark-viewed", token, map[string]any{"notificationIds": []uint{}}, 200)
	for _, path := range []string{"/api/notifications", "/api/notifications/unread-count", "/api/notifications/mark-viewed"} {
		method := "GET"
		if strings.HasSuffix(path, "mark-viewed") {
			method = "POST"
		}
		registrationRequest(t, router, method, path, "", nil, 401)
	}
	if err := db.Model(&owner).Update("is_active", false).Error; err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/notifications", "/api/notifications/unread-count", "/api/notifications/mark-viewed"} {
		method := "GET"
		if strings.HasSuffix(path, "mark-viewed") {
			method = "POST"
		}
		registrationRequest(t, router, method, path, token, map[string]any{"notificationIds": []uint{late.ID}}, 401)
	}
	if err := db.Model(&owner).Update("is_active", true).Error; err != nil {
		t.Fatal(err)
	}
	count(1)
	// Bounded responses still use the actual database unread count, not list length.
	rows := make([]model.Notification, 105)
	for i := range rows {
		rows[i] = model.Notification{UserID: owner.ID, Type: model.NotificationStudentStatus, Title: "title", Message: "message"}
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	list = registrationDecode[[]model.Notification](t, registrationRequest(t, router, "GET", "/api/notifications", token, nil, 200))
	if len(list) != service.NotificationListLimit {
		t.Fatalf("unbounded list: %d", len(list))
	}
	count(106)
}

func TestRestrictedNotificationAccessPreservesOperationalRestrictions(t *testing.T) {
	db := registrationTestDB(t)
	router := registrationRouter(t, db)
	company := registrationDemoUser(t, db, "company")
	university := registrationDemoUser(t, db, "university")
	for _, status := range []string{"PENDING", "REJECTED"} {
		t.Run(status, func(t *testing.T) {
			if err := db.Model(&model.Company{}).Where("id = ?", company.CompanyID).Update("registration_status", status).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&university).Update("verification_status", status).Error; err != nil {
				t.Fatal(err)
			}
			for _, user := range []model.User{company, university} {
				n := model.Notification{UserID: user.ID, Type: model.NotificationAccountStatus, Title: "وضعیت حساب", Message: "نتیجه بررسی"}
				if err := db.Create(&n).Error; err != nil {
					t.Fatal(err)
				}
				token := registrationToken(t, user)
				registrationRequest(t, router, "GET", "/api/notifications", token, nil, 200)
				registrationRequest(t, router, "GET", "/api/notifications/unread-count", token, nil, 200)
				registrationRequest(t, router, "POST", "/api/notifications/mark-viewed", token, map[string]any{"notificationIds": []uint{n.ID}}, 200)
			}
			registrationRequest(t, router, "GET", "/api/company/opportunities", registrationToken(t, company), nil, 403)
			registrationRequest(t, router, "GET", "/api/university/students", registrationToken(t, university), nil, 403)
		})
	}
	// Notifications expose only their small DTO, never company rating aggregates.
	body := registrationRequest(t, router, "GET", "/api/notifications", registrationToken(t, company), nil, 200).Body.String()
	for _, private := range []string{"rating", "Rating", "userId", "password"} {
		if strings.Contains(body, private) {
			t.Fatalf("notification response leaked %s", private)
		}
	}
}

func TestNotificationBusinessEvents(t *testing.T) {
	db := registrationTestDB(t)
	router := registrationRouter(t, db)
	suffix := time.Now().UnixNano()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	supervisor := registrationDemoUser(t, db, "company")
	professor := registrationDemoUser(t, db, "professor")
	university := registrationDemoUser(t, db, "university")
	workflow := service.NewInternshipService(db)
	notifications := service.NewNotificationService(db)
	applications := service.NewOpportunityApplicationService(db)
	total := func(id uint, kind model.NotificationType) int64 {
		t.Helper()
		var count int64
		must(db.Model(&model.Notification{}).Where("user_id = ? AND type = ?", id, kind).Count(&count).Error)
		return count
	}
	expect := func(id uint, kind model.NotificationType, want int64) {
		t.Helper()
		if got := total(id, kind); got != want {
			t.Fatalf("user %d type %s count %d want %d", id, kind, got, want)
		}
	}
	clear := func(id uint) {
		t.Helper()
		list, err := notifications.List(id)
		must(err)
		ids := []uint{}
		for _, n := range list {
			ids = append(ids, n.ID)
		}
		must(notifications.MarkViewed(id, ids))
	}
	student := func(label string) model.User {
		t.Helper()
		u := model.User{FullName: label, Email: fmt.Sprintf("m23-%s-%d@example.test", label, suffix), Role: model.RoleStudent, PasswordHash: "fixture"}
		must(db.Create(&u).Error)
		must(db.Create(&model.ProfessorAssignment{StudentID: u.ID, ProfessorID: professor.ID, AssignedAt: time.Now()}).Error)
		return u
	}
	unrelated := student("unrelated")
	opportunity, err := service.NewOpportunityService(db).Create(supervisor.ID, service.OpportunityInput{Title: "m23", Description: "description", WorkField: "software", Location: "Tehran"})
	must(err)
	resume := func() *model.File {
		return &model.File{OriginalName: "resume.pdf", StoredName: fmt.Sprintf("resume-%d.pdf", time.Now().UnixNano()), Path: "/tmp/m23-resume.pdf", MimeType: "application/pdf", SizeBytes: 20}
	}
	rejected := student("rejected")
	rejectedApp, err := applications.Apply(rejected.ID, opportunity.ID, resume())
	must(err)
	expect(supervisor.ID, model.NotificationCompanyApplications, 1)
	_, err = applications.Review(supervisor.ID, rejectedApp.ID, model.ApplicationStatusRejected, nil)
	must(err)
	expect(rejected.ID, model.NotificationStudentStatus, 1)
	// Both result mappings traverse all actual event hooks, including resubmission.
	for i, result := range []model.ProfessorFinalResult{model.ProfessorFinalResultGood, model.ProfessorFinalResultFailed} {
		t.Run(string(result), func(t *testing.T) {
			st := student(fmt.Sprintf("workflow-%d", i))
			app, err := applications.Apply(st.ID, opportunity.ID, resume())
			must(err)
			if i == 0 {
				expect(supervisor.ID, model.NotificationCompanyApplications, 1)
			} // no per-student flood
			_, err = applications.Review(supervisor.ID, app.ID, model.ApplicationStatusAccepted, nil)
			must(err)
			expect(st.ID, model.NotificationStudentStatus, 1)
			_, err = applications.Review(supervisor.ID, app.ID, model.ApplicationStatusAccepted, nil)
			if err == nil {
				t.Fatal("repeated transition accepted")
			}
			expect(st.ID, model.NotificationStudentStatus, 1)
			if i == 0 {
				_, err = service.NewInternshipTermService(db).Create(university.ID, 1405, model.InternshipTermTypeSummer)
				must(err)
			}
			item, _, err := workflow.CreateOrGetCase(st.ID)
			must(err)
			_, err = workflow.SubmitCase(st.ID)
			if err == nil {
				t.Fatal("invalid case submitted")
			}
			expect(st.ID, model.NotificationStudentStatus, 1)
			credits, mobile := 100, "09120000000"
			_, err = workflow.UpdateCase(st.ID, &credits, &mobile)
			must(err)
			pref, err := workflow.AddPreference(st.ID, service.PreferenceInput{OpportunityApplicationID: app.ID, Priority: 1})
			must(err)
			before := total(university.ID, model.NotificationUniversityCaseReview)
			_, err = workflow.SubmitCase(st.ID)
			must(err)
			if i == 0 {
				expect(university.ID, model.NotificationUniversityCaseReview, before+1)
			}
			_, err = workflow.RequestUniversityRevision(item.ID, university.ID, "اصلاح اولویت")
			must(err)
			expect(st.ID, model.NotificationStudentStatus, 2)
			clear(university.ID)
			before = total(university.ID, model.NotificationUniversityCaseReview)
			_, err = workflow.SubmitCase(st.ID)
			must(err)
			expect(university.ID, model.NotificationUniversityCaseReview, before+1)
			expect(st.ID, model.NotificationStudentStatus, 2) // own submission is silent
			clear(supervisor.ID)
			before = total(supervisor.ID, model.NotificationCompanyPlacement)
			_, err = workflow.ApproveUniversityPlacement(item.ID, service.UniversityPlacementApprovalInput{PreferenceID: pref.ID, LetterNumber: "m23", LetterDate: time.Now()})
			must(err)
			expect(st.ID, model.NotificationStudentStatus, 3)
			expect(supervisor.ID, model.NotificationCompanyPlacement, before+1)
			clear(university.ID)
			before = total(university.ID, model.NotificationUniversityPlacementReview)
			details := service.PlacementDetailsInput{InternshipSubject: "software", StartDate: time.Now(), WorkplaceAddress: "Tehran", WorkplacePhone: "02112345678"}
			_, err = workflow.SubmitPlacementDetails(supervisor.ID, item.ID, details)
			must(err)
			expect(university.ID, model.NotificationUniversityPlacementReview, before+1)
			clear(supervisor.ID)
			before = total(supervisor.ID, model.NotificationCompanyPlacement)
			_, err = workflow.RequestPlacementCorrection(item.ID, "اصلاح نشانی")
			must(err)
			expect(supervisor.ID, model.NotificationCompanyPlacement, before+1)
			_, err = workflow.SubmitPlacementDetails(supervisor.ID, item.ID, details)
			must(err)
			// University has not read its previous alert; no duplicate on correction.
			_, err = workflow.ApprovePlacementDetails(item.ID)
			must(err)
			expect(st.ID, model.NotificationStudentStatus, 4)
			clear(supervisor.ID)
			clear(professor.ID)
			companyBefore, profBefore := total(supervisor.ID, model.NotificationCompanyWeeklyReports), total(professor.ID, model.NotificationProfessorAction)
			for week := 1; week <= 8; week++ {
				report, err := workflow.CreateWeeklyReport(st.ID, service.WeeklyReportInput{WeekNumber: week, StartDate: time.Now(), EndDate: time.Now().Add(time.Hour), ActivityDescription: "کارهای هفته"})
				must(err)
				_, err = workflow.SubmitWeeklyReport(st.ID, report.ID)
				must(err)
				expect(supervisor.ID, model.NotificationCompanyWeeklyReports, companyBefore+1)
				expect(professor.ID, model.NotificationProfessorAction, profBefore+1)
				if week == 1 {
					comment := "اصلاح گزارش"
					_, err = workflow.ReviewWeeklyReportByCompany(supervisor.ID, item.ID, report.ID, model.WeeklyReviewRevisionRequested, &comment)
					must(err)
					_, err = workflow.SubmitWeeklyReport(st.ID, report.ID)
					must(err)
					_, err = workflow.ReviewWeeklyReportByProfessor(professor.ID, item.ID, report.ID, model.WeeklyReviewRevisionRequested, &comment)
					must(err)
					_, err = workflow.SubmitWeeklyReport(st.ID, report.ID)
					must(err)
					expect(st.ID, model.NotificationStudentStatus, 6)
				}
				_, err = workflow.ReviewWeeklyReportByCompany(supervisor.ID, item.ID, report.ID, model.WeeklyReviewApproved, nil)
				must(err)
				_, err = workflow.ReviewWeeklyReportByProfessor(professor.ID, item.ID, report.ID, model.WeeklyReviewApproved, nil)
				must(err)
			}
			expect(st.ID, model.NotificationStudentStatus, 22)
			before = total(professor.ID, model.NotificationProfessorAction)
			clear(professor.ID)
			_, _, err = workflow.AttachFinalReport(st.ID, resume())
			must(err)
			expect(professor.ID, model.NotificationProfessorAction, before+1)
			comment := "اصلاح گزارش نهایی"
			_, err = workflow.ReviewFinalReport(professor.ID, item.ID, model.FinalReportRevisionRequested, &comment)
			must(err)
			expect(st.ID, model.NotificationStudentStatus, 23)
			_, _, err = workflow.AttachFinalReport(st.ID, resume())
			must(err)
			expect(professor.ID, model.NotificationProfessorAction, before+1)
			_, err = workflow.ReviewFinalReport(professor.ID, item.ID, model.FinalReportApproved, nil)
			must(err)
			expect(st.ID, model.NotificationStudentStatus, 24)
			evaluation := model.CompanyEvaluation{InternshipCaseID: item.ID, CompanySupervisorID: supervisor.ID, AttendanceRating: model.EvaluationRatingGood, ParticipationRating: model.EvaluationRatingGood, LearningRating: model.EvaluationRatingGood, InterestRating: model.EvaluationRatingGood, PersistenceRating: model.EvaluationRatingGood, SuggestionRating: model.EvaluationRatingGood, ResourceUsageRating: model.EvaluationRatingGood, ReportQualityRating: model.EvaluationRatingGood, ProjectPerformanceRating: model.EvaluationRatingGood, SubmittedAt: time.Now()}
			must(db.Create(&evaluation).Error)
			_, err = workflow.CompleteProfessorCase(professor.ID, item.ID, service.ProfessorCompletionInput{Result: result})
			must(err)
			expect(st.ID, model.NotificationStudentStatus, 25)
			list, err := notifications.List(st.ID)
			must(err)
			word := "قبول"
			if result == model.ProfessorFinalResultFailed {
				word = "مردود"
			}
			if !strings.Contains(list[0].Message, word) {
				t.Fatalf("incorrect outcome: %s", list[0].Message)
			}
			// Production authorization fails before the business action or notification.
			countBefore := total(st.ID, model.NotificationStudentStatus)
			registrationRequest(t, router, "POST", fmt.Sprintf("/api/professor/internship-cases/%d/complete", item.ID), registrationToken(t, unrelated), map[string]any{"result": "GOOD"}, 403)
			expect(st.ID, model.NotificationStudentStatus, countBefore)
			_, err = workflow.CompleteProfessorCase(professor.ID, item.ID, service.ProfessorCompletionInput{Result: result})
			if err == nil {
				t.Fatal("finalized twice")
			}
			expect(st.ID, model.NotificationStudentStatus, countBefore)
		})
	}
	expect(unrelated.ID, model.NotificationStudentStatus, 0)
}
