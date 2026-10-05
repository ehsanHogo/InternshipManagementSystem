package main

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"internship-management-system/backend/internal/auth"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

func universityRegistration(t *testing.T, db *gorm.DB, router http.Handler, label string) (model.User, string) {
	t.Helper()
	response := registrationRequest(t, router, "POST", "/api/auth/university-supervisor-register", "", map[string]string{"fullName": "مسئول آموزش " + label, "email": label + "@example.test", "phone": "09120000000", "password": "University123!"}, http.StatusCreated)
	public := registrationDecode[struct{ User model.PublicUser }](t, response).User
	user := profileUser(t, db, public.ID)
	if user.Role != model.RoleUniversitySupervisor || user.VerificationStatus != model.UserVerificationPending || !user.IsActive || user.PasswordHash == "University123!" || !auth.CheckPassword(user.PasswordHash, "University123!") {
		t.Fatal("incorrect registration state or password hashing")
	}
	login := registrationDecode[struct{ Token string }](t, registrationRequest(t, router, "POST", "/api/auth/login", "", map[string]string{"email": user.Email, "password": "University123!"}, http.StatusOK))
	return user, login.Token
}

func TestUniversityIdentityLifecycle(t *testing.T) {
	db := registrationTestDB(t)
	router := registrationRouter(t, db)
	admin := registrationDemoUser(t, db, "admin")
	adminToken := registrationToken(t, admin)
	user, token := universityRegistration(t, db, router, "m22-identity")
	path := fmt.Sprintf("/api/admin/user-verifications/%d", user.ID)
	for _, payload := range []map[string]any{
		{"fullName": "Name", "email": "other@example.test", "password": "Password!", "role": "ADMIN"},
		{"fullName": "Name", "email": "other@example.test", "password": "Password!", "isActive": false},
		{"fullName": "Name", "email": "other@example.test", "password": "Password!", "verificationStatus": "APPROVED"},
		{"fullName": " ", "email": "other@example.test", "password": "Password!"},
		{"fullName": "Name", "email": "invalid", "password": "Password!"},
		{"fullName": "Name", "email": "other@example.test", "password": strings.Repeat("a", 73)},
		{"fullName": "Name", "email": "other@example.test", "password": " "},
	} {
		registrationRequest(t, router, "POST", "/api/auth/university-supervisor-register", "", payload, http.StatusBadRequest)
	}
	registrationRequest(t, router, "POST", "/api/auth/university-supervisor-register", "", map[string]string{"fullName": "Name", "email": strings.ToUpper(user.Email), "password": "Password!"}, http.StatusConflict)
	for _, role := range []string{"student", "professor", "company", "university"} {
		other := registrationDemoUser(t, db, role)
		otherToken := registrationToken(t, other)
		for _, endpoint := range []string{"/api/admin/user-verifications", "/api/admin/users", path} {
			registrationRequest(t, router, "GET", endpoint, otherToken, nil, http.StatusForbidden)
		}
		registrationRequest(t, router, "POST", path+"/approve", otherToken, nil, http.StatusForbidden)
		registrationRequest(t, router, "POST", path+"/reject", otherToken, map[string]string{"reason": "No"}, http.StatusForbidden)
		if other.Role != model.RoleUniversitySupervisor {
			registrationRequest(t, router, "GET", fmt.Sprintf("/api/admin/user-verifications/%d", other.ID), adminToken, nil, http.StatusNotFound)
		}
	}
	registrationRequest(t, router, "GET", fmt.Sprintf("/api/admin/user-verifications/%d", admin.ID), adminToken, nil, http.StatusNotFound)
	queue := registrationDecode[[]model.PublicUser](t, registrationRequest(t, router, "GET", "/api/admin/user-verifications?status=PENDING&search=m22-identity", adminToken, nil, http.StatusOK))
	if len(queue) != 1 || queue[0].ID != user.ID {
		t.Fatal("verification filter/search failed")
	}
	all := registrationDecode[[]model.PublicUser](t, registrationRequest(t, router, "GET", "/api/admin/user-verifications", adminToken, nil, http.StatusOK))
	for _, queued := range all {
		if queued.Role != model.RoleUniversitySupervisor {
			t.Fatal("unrelated role in verification queue")
		}
	}
	registrationRequest(t, router, "GET", path, adminToken, nil, http.StatusOK)
	// Broad business-route coverage also catches shared-route authorization leaks.
	blocked := func() {
		t.Helper()
		for _, endpoint := range []string{"/api/university/internship-terms", "/api/university/students", "/api/university/professors", "/api/university/professor-assignments", "/api/university/internship-cases", "/api/university/companies", "/api/university/company-supervisors", "/api/companies", "/api/files/1/download"} {
			registrationRequest(t, router, "GET", endpoint, token, nil, http.StatusForbidden)
		}
		for _, endpoint := range []string{"/api/university/students", "/api/university/students/import", "/api/university/professors", "/api/university/professors/import", "/api/university/professor-assignments", "/api/university/internship-cases/1/approve-placement"} {
			registrationRequest(t, router, "POST", endpoint, token, map[string]string{}, http.StatusForbidden)
		}
	}
	blocked()
	registrationRequest(t, router, "GET", "/api/auth/me", token, nil, http.StatusOK)
	registrationRequest(t, router, "PUT", "/api/profile", token, map[string]string{"fullName": "Pending name", "phone": "phone"}, http.StatusOK)
	registrationRequest(t, router, "POST", "/api/profile/change-password", token, map[string]string{"currentPassword": "University123!", "newPassword": "Pending123!"}, http.StatusOK)
	registrationRequest(t, router, "POST", "/api/university-supervisor/verification/resubmit", token, nil, http.StatusConflict)
	registrationRequest(t, router, "POST", path+"/reject", adminToken, map[string]string{"reason": "  "}, http.StatusBadRequest)
	rejected := registrationDecode[model.PublicUser](t, registrationRequest(t, router, "POST", path+"/reject", adminToken, map[string]string{"reason": " مدارک اطلاعات ناقص "}, http.StatusOK))
	stored := profileUser(t, db, user.ID)
	if rejected.VerificationStatus != model.UserVerificationRejected || stored.VerificationReviewedBy == nil || *stored.VerificationReviewedBy != admin.ID || stored.VerificationReviewedAt == nil || rejected.VerificationRejectionReason == nil || *rejected.VerificationRejectionReason != "مدارک اطلاعات ناقص" || stored.Role != user.Role || !stored.IsActive {
		t.Fatal("rejection audit/state incorrect")
	}
	blocked()
	registrationRequest(t, router, "POST", "/api/auth/login", "", map[string]string{"email": user.Email, "password": "Pending123!"}, http.StatusOK)
	me := registrationDecode[model.PublicUser](t, registrationRequest(t, router, "GET", "/api/auth/me", token, nil, http.StatusOK))
	if me.VerificationRejectionReason == nil || *me.VerificationRejectionReason != "مدارک اطلاعات ناقص" {
		t.Fatal("owner cannot see rejection reason")
	}
	registrationRequest(t, router, "PUT", "/api/profile", token, map[string]string{"fullName": "Corrected name", "phone": "updated"}, http.StatusOK)
	if profileUser(t, db, user.ID).VerificationStatus != model.UserVerificationRejected {
		t.Fatal("profile implicitly resubmitted verification")
	}
	registrationRequest(t, router, "POST", "/api/profile/change-password", token, map[string]string{"currentPassword": "Pending123!", "newPassword": "Rejected123!"}, http.StatusOK)
	for _, protected := range []string{"isActive", "verificationStatus", "verificationReviewedBy", "verificationRejectionReason"} {
		registrationRequest(t, router, "PUT", "/api/profile", token, map[string]any{"fullName": "Invalid", protected: "APPROVED"}, http.StatusBadRequest)
	}
	registrationRequest(t, router, "POST", path+"/approve", adminToken, nil, http.StatusConflict)
	resubmitted := registrationDecode[model.PublicUser](t, registrationRequest(t, router, "POST", "/api/university-supervisor/verification/resubmit?userId=1", token, nil, http.StatusOK))
	if resubmitted.ID != user.ID || resubmitted.Role != user.Role || !resubmitted.IsActive || resubmitted.VerificationStatus != model.UserVerificationPending || resubmitted.VerificationResubmittedAt == nil || resubmitted.VerificationRejectionReason != nil {
		t.Fatal("invalid resubmission")
	}
	blocked()
	// Approve a disabled account: identity review must never enable access.
	registrationRequest(t, router, "POST", fmt.Sprintf("/api/admin/users/%d/disable", user.ID), adminToken, nil, http.StatusOK)
	approved := registrationDecode[model.PublicUser](t, registrationRequest(t, router, "POST", path+"/approve", adminToken, nil, http.StatusOK))
	stored = profileUser(t, db, user.ID)
	if approved.IsActive || approved.Role != user.Role || approved.VerificationStatus != model.UserVerificationApproved || stored.VerificationReviewedAt == nil || stored.VerificationReviewedBy == nil || *stored.VerificationReviewedBy != admin.ID {
		t.Fatal("approval modified account access or missed audit")
	}
	registrationRequest(t, router, "GET", "/api/university/students", token, nil, http.StatusUnauthorized)
	registrationRequest(t, router, "POST", fmt.Sprintf("/api/admin/users/%d/enable", user.ID), adminToken, nil, http.StatusOK)
	registrationRequest(t, router, "GET", "/api/university/students", token, nil, http.StatusOK)
	registrationRequest(t, router, "GET", "/api/university/professors", token, nil, http.StatusOK)
	for _, action := range []string{"approve", "reject"} {
		registrationRequest(t, router, "POST", path+"/"+action, adminToken, map[string]string{"reason": "invalid"}, http.StatusConflict)
	}
	registrationRequest(t, router, "POST", "/api/university-supervisor/verification/resubmit", token, nil, http.StatusConflict)
}

func TestAccountActivationAllRolesAndProvisioning(t *testing.T) {
	db := registrationTestDB(t)
	router := registrationRouter(t, db)
	admin := registrationDemoUser(t, db, "admin")
	adminToken := registrationToken(t, admin)
	university := registrationDemoUser(t, db, "university")
	universityToken := registrationToken(t, university)
	// Existing trusted university provisioning produces normal, active accounts.
	for _, kind := range []string{"students", "professors"} {
		payload := map[string]string{"fullName": "M22 managed " + kind, "email": "m22-" + kind + "@example.test", "studentNumber": "m22-123", "major": "Computer"}
		created := registrationDecode[struct {
			User              model.PublicUser
			TemporaryPassword string
		}](t, registrationRequest(t, router, "POST", "/api/university/"+kind, universityToken, payload, http.StatusCreated))
		if !created.User.IsActive || created.User.VerificationStatus != model.UserVerificationNotRequired {
			t.Fatal("managed account needs verification or is inactive")
		}
		registrationRequest(t, router, "POST", "/api/auth/login", "", map[string]string{"email": created.User.Email, "password": created.TemporaryPassword}, http.StatusOK)
	}
	professor := registrationDemoUser(t, db, "professor")
	student := registrationDemoUser(t, db, "student")
	registrationRequest(t, router, "POST", "/api/university/professor-assignments", universityToken, map[string]uint{"studentId": student.ID, "professorId": professor.ID}, http.StatusOK)
	var beforeAssignment model.ProfessorAssignment
	if err := db.Where("student_id = ?", student.ID).First(&beforeAssignment).Error; err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"student", "professor", "company", "university"} {
		user := registrationDemoUser(t, db, role)
		token := registrationToken(t, user)
		path := fmt.Sprintf("/api/admin/users/%d", user.ID)
		before := profileUser(t, db, user.ID)
		var companyBefore model.Company
		if user.CompanyID != nil {
			if err := db.First(&companyBefore, *user.CompanyID).Error; err != nil {
				t.Fatal(err)
			}
		}
		registrationRequest(t, router, "POST", path+"/disable", adminToken, nil, http.StatusOK)
		for _, endpoint := range []string{"/api/auth/me", "/api/protected", "/api/companies"} {
			registrationRequest(t, router, "GET", endpoint, token, nil, http.StatusUnauthorized)
		}
		registrationRequest(t, router, "PUT", "/api/profile", token, map[string]string{"fullName": "No"}, http.StatusUnauthorized)
		registrationRequest(t, router, "POST", "/api/profile/change-password", token, map[string]string{}, http.StatusUnauthorized)
		registrationRequest(t, router, "POST", "/api/auth/login", "", map[string]string{"email": user.Email, "password": "Demo123!"}, http.StatusUnauthorized)
		disabled := profileUser(t, db, user.ID)
		if disabled.IsActive {
			t.Fatal("disable failed")
		}
		disabled.IsActive = before.IsActive
		disabled.UpdatedAt = before.UpdatedAt
		if !reflect.DeepEqual(disabled, before) {
			t.Fatal("disable modified identity/verification/business affiliation")
		}
		if role == "professor" {
			registrationRequest(t, router, "POST", "/api/university/professor-assignments", universityToken, map[string]uint{"studentId": student.ID, "professorId": professor.ID}, http.StatusNotFound)
			registrationRequest(t, router, "GET", "/api/university/professor-assignments", universityToken, nil, http.StatusOK)
		}
		registrationRequest(t, router, "GET", path, adminToken, nil, http.StatusOK)
		result := registrationDecode[[]model.PublicUser](t, registrationRequest(t, router, "GET", "/api/admin/users?isActive=false&role="+string(user.Role)+"&search="+user.Email, adminToken, nil, http.StatusOK))
		if len(result) != 1 || result[0].ID != user.ID {
			t.Fatal("user search/filters failed")
		}
		registrationRequest(t, router, "POST", path+"/enable", adminToken, nil, http.StatusOK)
		after := profileUser(t, db, user.ID)
		after.UpdatedAt = before.UpdatedAt
		if !reflect.DeepEqual(after, before) {
			t.Fatal("enable modified identity or verification")
		}
		registrationRequest(t, router, "GET", "/api/auth/me", token, nil, http.StatusOK)
		if user.CompanyID != nil {
			var afterCompany model.Company
			if err := db.First(&afterCompany, *user.CompanyID).Error; err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(companyBefore, afterCompany) {
				t.Fatal("activation mutated company")
			}
		}
	}
	var afterAssignment model.ProfessorAssignment
	if err := db.First(&afterAssignment, beforeAssignment.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(beforeAssignment, afterAssignment) {
		t.Fatal("assignment history changed")
	}
	registrationRequest(t, router, "GET", "/api/professor/internship-cases", registrationToken(t, professor), nil, http.StatusOK)
	// Re-enabling rejected university/company users must preserve restrictions.
	rejected, token := universityRegistration(t, db, router, "m22-reenable")
	registrationRequest(t, router, "POST", fmt.Sprintf("/api/admin/user-verifications/%d/reject", rejected.ID), adminToken, map[string]string{"reason": "Reason"}, http.StatusOK)
	for _, action := range []string{"disable", "enable"} {
		registrationRequest(t, router, "POST", fmt.Sprintf("/api/admin/users/%d/%s", rejected.ID, action), adminToken, nil, http.StatusOK)
	}
	registrationRequest(t, router, "GET", "/api/university/students", token, nil, http.StatusForbidden)
	for _, status := range []model.CompanyRegistrationStatus{model.CompanyRegistrationStatusPending, model.CompanyRegistrationStatusRejected, model.CompanyRegistrationStatusApproved} {
		_, company, owner, companyToken := registrationFixture(t, router, "m22-activation-"+string(status))
		if err := db.Model(&company).Updates(map[string]any{"registration_status": status, "is_approved": false}).Error; err != nil {
			t.Fatal(err)
		}
		for _, action := range []string{"disable", "enable"} {
			registrationRequest(t, router, "POST", fmt.Sprintf("/api/admin/users/%d/%s", owner.ID, action), adminToken, nil, http.StatusOK)
		}
		var current model.Company
		if err := db.First(&current, company.ID).Error; err != nil {
			t.Fatal(err)
		}
		if current.RegistrationStatus != status || current.IsApproved {
			t.Fatal("enable approved company")
		}
		want := http.StatusForbidden
		if status == model.CompanyRegistrationStatusApproved {
			want = http.StatusOK
		}
		registrationRequest(t, router, "GET", "/api/company/opportunities", companyToken, nil, want)
	}
	for _, route := range router.Routes() {
		if strings.Contains(strings.ToLower(route.Path), "role") {
			t.Fatalf("unexpected role API %s", route.Path)
		}
	}
	for _, path := range []string{"/api/admin/users?role=BAD", "/api/admin/users?isActive=invalid", "/api/admin/user-verifications?status=NOT_REQUIRED"} {
		registrationRequest(t, router, "GET", path, adminToken, nil, http.StatusBadRequest)
	}
}

func TestAdminActivationSafety(t *testing.T) {
	db := registrationTestDB(t)
	router := registrationRouter(t, db)
	admin := registrationDemoUser(t, db, "admin")
	token := registrationToken(t, admin)
	registrationRequest(t, router, "POST", fmt.Sprintf("/api/admin/users/%d/disable", admin.ID), token, nil, http.StatusConflict)
	other := model.User{FullName: "Second admin", Email: "m22-admin@example.test", Role: model.RoleAdmin, PasswordHash: admin.PasswordHash}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	otherToken := registrationToken(t, other)
	registrationRequest(t, router, "POST", fmt.Sprintf("/api/admin/users/%d/disable", other.ID), token, nil, http.StatusOK)
	registrationRequest(t, router, "GET", "/api/admin/users", otherToken, nil, http.StatusUnauthorized)
	registrationRequest(t, router, "POST", "/api/auth/login", "", map[string]string{"email": other.Email, "password": "Demo123!"}, http.StatusUnauthorized)
	registrationRequest(t, router, "POST", fmt.Sprintf("/api/admin/users/%d/enable", other.ID), token, nil, http.StatusOK)
	registrationRequest(t, router, "POST", fmt.Sprintf("/api/admin/users/%d/disable", admin.ID), otherToken, nil, http.StatusOK)
	// The service independently enforces active Admin authority.
	_, err := service.NewUserAccessService(db).SetActive(admin.ID, other.ID, false)
	if err != service.ErrUserAccessForbidden {
		t.Fatalf("disabled admin service access: %v", err)
	}
	if !profileUser(t, db, other.ID).IsActive {
		t.Fatal("last active admin lost access")
	}
}

func TestCurrentDatabaseRoleAndVerificationReasonLimits(t *testing.T) {
	db := registrationTestDB(t)
	router := registrationRouter(t, db)
	admin := registrationDemoUser(t, db, "admin")
	adminToken := registrationToken(t, admin)
	student := registrationDemoUser(t, db, "student")
	// A valid signature must not grant a token's stale role over the current row.
	stale := student
	stale.Role = model.RoleAdmin
	token := registrationToken(t, stale)
	registrationRequest(t, router, "GET", "/api/admin/users", token, nil, http.StatusForbidden)
	registrationRequest(t, router, "POST", "/api/university-supervisor/verification/resubmit", token, nil, http.StatusForbidden)
	// Missing current users cannot keep using their JWT.
	missingToken, err := auth.CreateToken(model.User{ID: 4294967295, Role: model.RoleAdmin}, registrationTestSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	registrationRequest(t, router, "GET", "/api/auth/me", missingToken, nil, http.StatusUnauthorized)
	user, _ := universityRegistration(t, db, router, "m22-reason-limit")
	path := fmt.Sprintf("/api/admin/user-verifications/%d/reject", user.ID)
	registrationRequest(t, router, "POST", path, adminToken, map[string]string{"reason": strings.Repeat("ن", 5001)}, http.StatusBadRequest)
	// UTF-8 reasons up to the documented character limit fit the strict decoder.
	reason := strings.Repeat("ن", 5000)
	rejected := registrationDecode[model.PublicUser](t, registrationRequest(t, router, "POST", path, adminToken, map[string]string{"reason": reason}, http.StatusOK))
	if rejected.VerificationRejectionReason == nil || *rejected.VerificationRejectionReason != reason {
		t.Fatal("valid long reason was not persisted")
	}
}

func TestActivationPreservesBusinessHistory(t *testing.T) {
	db := registrationTestDB(t)
	router := registrationRouter(t, db)
	student := registrationDemoUser(t, db, "student")
	professor := registrationDemoUser(t, db, "professor")
	company := registrationDemoUser(t, db, "company")
	university := registrationDemoUser(t, db, "university")
	admin := registrationDemoUser(t, db, "admin")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	comment := "preserved review and audit"
	opportunity := model.InternshipOpportunity{CompanyID: *company.CompanyID, CreatedBy: company.ID, Title: "History", Description: "History", WorkField: "Software", Location: "Tehran", Status: model.OpportunityStatusClosed}
	must(db.Create(&opportunity).Error)
	resume := model.File{OriginalName: "resume.pdf", StoredName: "m22-resume", Path: "fixture", MimeType: "application/pdf", SizeBytes: 20, UploadedBy: student.ID, UploadedAt: now}
	finalFile := model.File{OriginalName: "final.pdf", StoredName: "m22-final", Path: "fixture", MimeType: "application/pdf", SizeBytes: 20, UploadedBy: student.ID, UploadedAt: now}
	must(db.Create(&resume).Error)
	must(db.Create(&finalFile).Error)
	application := model.OpportunityApplication{OpportunityID: opportunity.ID, StudentID: student.ID, ResumeFileID: resume.ID, Status: model.ApplicationStatusAccepted, CompanyComment: &comment, AppliedAt: now, ReviewedAt: &now}
	must(db.Create(&application).Error)
	internshipCase := model.InternshipCase{StudentID: student.ID, ProfessorID: professor.ID, CompanySupervisorID: &company.ID, Status: model.InternshipCaseStatusPassed, ActivatedAt: &now, CompletedAt: &now, UniversityRevisionRequestedBy: &university.ID, UniversityRevisionComment: &comment, ProfessorComment: &comment}
	must(db.Create(&internshipCase).Error)
	preference := model.InternshipPreference{InternshipCaseID: internshipCase.ID, OpportunityApplicationID: application.ID, Priority: 1}
	must(db.Create(&preference).Error)
	must(db.Model(&internshipCase).Update("selected_preference_id", preference.ID).Error)
	weekly := model.WeeklyReport{InternshipCaseID: internshipCase.ID, WeekNumber: 1, StartDate: now, EndDate: now, ActivityDescription: "Work", SubmittedAt: &now, CompanyReviewStatus: model.WeeklyReviewApproved, CompanyReviewComment: &comment, CompanyReviewedAt: &now, ProfessorReviewStatus: model.WeeklyReviewApproved, ProfessorReviewComment: &comment, ProfessorReviewedAt: &now}
	final := model.FinalReport{InternshipCaseID: internshipCase.ID, CurrentFileID: finalFile.ID, Status: model.FinalReportApproved, SubmittedAt: now, ReviewedAt: &now, ReviewComment: &comment}
	rating := model.StudentInternshipRating{InternshipCaseID: internshipCase.ID, StudentID: student.ID, CompanyID: *company.CompanyID, Rating: 5}
	must(db.Create(&weekly).Error)
	must(db.Create(&final).Error)
	must(db.Create(&rating).Error)
	snapshot := func() map[string][]string {
		t.Helper()
		result := map[string][]string{}
		for _, table := range []string{"companies", "professor_assignments", "internship_cases", "internship_preferences", "internship_opportunities", "opportunity_applications", "files", "weekly_reports", "final_reports", "student_internship_ratings"} {
			var records []string
			must(db.Raw("SELECT row_to_json(record)::text FROM " + table + " AS record ORDER BY id").Scan(&records).Error)
			result[table] = records
		}
		return result
	}
	before := snapshot()
	token := registrationToken(t, admin)
	for _, user := range []model.User{student, professor, company, university} {
		for _, action := range []string{"disable", "enable"} {
			registrationRequest(t, router, "POST", fmt.Sprintf("/api/admin/users/%d/%s", user.ID, action), token, nil, http.StatusOK)
			if !reflect.DeepEqual(before, snapshot()) {
				t.Fatalf("%s for %s rewrote business history or audit references", action, user.Role)
			}
		}
	}
}
