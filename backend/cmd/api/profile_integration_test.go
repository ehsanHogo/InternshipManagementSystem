package main

import (
	"encoding/json"
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

var profileTestRoles = []string{"student", "company", "professor", "university", "admin"}

func profileUser(t *testing.T, db *gorm.DB, id uint) model.User {
	t.Helper()
	var user model.User
	if err := db.First(&user, id).Error; err != nil {
		t.Fatal(err)
	}
	return user
}

func TestProfileReadAndUpdateAllRoles(t *testing.T) {
	db := registrationTestDB(t)
	router := registrationRouter(t, db)
	registrationRequest(t, router, "GET", "/api/auth/me", "", nil, http.StatusUnauthorized)
	registrationRequest(t, router, "PUT", "/api/profile", "", map[string]string{"fullName": "نام"}, http.StatusUnauthorized)
	registrationRequest(t, router, "POST", "/api/profile/change-password", "", map[string]string{}, http.StatusUnauthorized)
	for _, role := range profileTestRoles {
		t.Run(role, func(t *testing.T) {
			owner := registrationDemoUser(t, db, role)
			other := registrationDemoUser(t, db, "admin")
			if owner.ID == other.ID {
				other = registrationDemoUser(t, db, "student")
			}
			token := registrationToken(t, owner)
			path := fmt.Sprintf("/api/auth/me?userId=%d", other.ID)
			read := registrationRequest(t, router, "GET", path, token, nil, http.StatusOK)
			public := registrationDecode[model.PublicUser](t, read)
			if public.ID != owner.ID || public.FullName != owner.FullName || public.Email != owner.Email || public.Role != owner.Role {
				t.Fatal("profile did not read authenticated account")
			}
			var fields map[string]any
			if err := json.Unmarshal(read.Body.Bytes(), &fields); err != nil {
				t.Fatal(err)
			}
			allowed := map[string]bool{"isActive": true, "createdAt": true, "verificationStatus": true, "verificationReviewedAt": true, "verificationRejectionReason": true, "verificationResubmittedAt": true, "id": true, "fullName": true, "email": true, "role": true, "phone": true, "studentNumber": true, "major": true, "jobTitle": true, "companyId": true, "companyName": true, "companyRegistrationStatus": true}
			for key := range fields {
				if !allowed[key] {
					t.Fatalf("unsafe profile field: %s", key)
				}
			}
			if strings.Contains(read.Body.String(), owner.PasswordHash) || strings.Contains(read.Body.String(), "$2a$") {
				t.Fatal("hash leaked")
			}
			if owner.Role == model.RoleStudent && (!reflect.DeepEqual(public.StudentNumber, owner.StudentNumber) || !reflect.DeepEqual(public.Major, owner.Major)) {
				t.Fatal("missing academic identity")
			}
			if owner.Role == model.RoleCompanySupervisor && (public.CompanyName == nil || public.CompanyRegistrationStatus == nil) {
				t.Fatal("missing safe company affiliation")
			}
			payload := map[string]any{"fullName": "  نام جدید " + role + "  ", "phone": "  تماس آزاد  "}
			if owner.Role == model.RoleCompanySupervisor {
				payload["jobTitle"] = "  عنوان جدید  "
			}
			updated := registrationDecode[model.PublicUser](t, registrationRequest(t, router, "PUT", fmt.Sprintf("/api/profile?userId=%d", other.ID), token, payload, http.StatusOK))
			if updated.ID != owner.ID || updated.FullName != "نام جدید "+role || updated.Phone == nil || *updated.Phone != "تماس آزاد" {
				t.Fatal("personal fields not updated/normalized")
			}
			stored := profileUser(t, db, owner.ID)
			if stored.FullName != updated.FullName || !reflect.DeepEqual(stored.Phone, updated.Phone) {
				t.Fatal("update not persisted")
			}
			if stored.Email != owner.Email || stored.Role != owner.Role || !reflect.DeepEqual(stored.CompanyID, owner.CompanyID) || !reflect.DeepEqual(stored.StudentNumber, owner.StudentNumber) || !reflect.DeepEqual(stored.Major, owner.Major) || stored.PasswordHash != owner.PasswordHash || stored.CreatedAt != owner.CreatedAt {
				t.Fatal("protected field changed")
			}
			if owner.Role == model.RoleCompanySupervisor && (stored.JobTitle == nil || *stored.JobTitle != "عنوان جدید") {
				t.Fatal("job title not updated")
			}
			if !reflect.DeepEqual(other, profileUser(t, db, other.ID)) {
				t.Fatal("another user's row changed")
			}
			refreshed := registrationDecode[model.PublicUser](t, registrationRequest(t, router, "GET", "/api/auth/me", token, nil, http.StatusOK))
			if refreshed.FullName != updated.FullName {
				t.Fatal("auth/me did not refresh name")
			}
			registrationRequest(t, router, "PUT", "/api/profile", token, map[string]any{"fullName": updated.FullName, "phone": "  "}, http.StatusOK)
			if profileUser(t, db, owner.ID).Phone != nil {
				t.Fatal("optional phone was not cleared")
			}
		})
	}
}

func TestProfileRejectsProtectedFieldsAndInvalidInput(t *testing.T) {
	db := registrationTestDB(t)
	router := registrationRouter(t, db)
	for _, role := range profileTestRoles {
		owner := registrationDemoUser(t, db, role)
		token := registrationToken(t, owner)
		protected := map[string]any{
			"id": 999, "userId": 999, "email": "changed@example.test", "role": "ADMIN", "companyId": 999,
			"studentNumber": "new-number", "major": "new-major", "passwordHash": "plaintext", "password": "plaintext",
			"registrationStatus": "APPROVED", "isApproved": true, "nationalId": "legal-id", "economicCode": "legal-code",
			"company": map[string]any{"name": "company", "isApproved": true}, "companyName": "company",
			"companyEmail": "office@example.test", "companyAddress": "address", "createdAt": time.Now(), "updatedAt": time.Now(),
		}
		for key, value := range protected {
			t.Run(role+"/"+key, func(t *testing.T) {
				registrationRequest(t, router, "PUT", "/api/profile", token, map[string]any{"fullName": "Should not persist", "phone": "new", key: value}, http.StatusBadRequest)
				if !reflect.DeepEqual(owner, profileUser(t, db, owner.ID)) {
					t.Fatal("rejected mass assignment changed the user")
				}
			})
		}
		if owner.Role != model.RoleCompanySupervisor {
			registrationRequest(t, router, "PUT", "/api/profile", token, map[string]any{"fullName": "نام", "jobTitle": "سمت"}, http.StatusBadRequest)
		}
		for _, input := range []map[string]any{
			{"fullName": ""}, {"fullName": " \t\n "}, {"phone": "phone"}, {"fullName": strings.Repeat("ن", 201)},
			{"fullName": "نام", "phone": strings.Repeat("ن", 51)}, {"fullName": 42},
		} {
			registrationRequest(t, router, "PUT", "/api/profile", token, input, http.StatusBadRequest)
		}
		if !reflect.DeepEqual(owner, profileUser(t, db, owner.ID)) {
			t.Fatal("invalid profile input changed account")
		}
	}
	supervisor := registrationDemoUser(t, db, "company")
	registrationRequest(t, router, "PUT", "/api/profile", registrationToken(t, supervisor), map[string]any{"fullName": "نام", "jobTitle": strings.Repeat("ن", 201)}, http.StatusBadRequest)
}

func TestProfileStudentPhoneDoesNotChangeCases(t *testing.T) {
	db := registrationTestDB(t)
	router := registrationRouter(t, db)
	student := registrationDemoUser(t, db, "student")
	professor := registrationDemoUser(t, db, "professor")
	university := registrationDemoUser(t, db, "university")
	now := time.Now()
	closed := model.InternshipTerm{AcademicYear: 1404, TermType: model.InternshipTermTypeSummer, Status: model.InternshipTermStatusClosed, OpenedAt: now, ClosedAt: &now, CreatedBy: university.ID}
	open := model.InternshipTerm{AcademicYear: 1405, TermType: model.InternshipTermTypeSummer, Status: model.InternshipTermStatusOpen, OpenedAt: now, CreatedBy: university.ID}
	for _, term := range []*model.InternshipTerm{&closed, &open} {
		if err := db.Create(term).Error; err != nil {
			t.Fatal(err)
		}
	}
	mobile := "09120000000"
	cases := []model.InternshipCase{
		{StudentID: student.ID, ProfessorID: professor.ID, TermID: &closed.ID, Status: model.InternshipCaseStatusPassed, Mobile: &mobile},
		{StudentID: student.ID, ProfessorID: professor.ID, TermID: &open.ID, Status: model.InternshipCaseStatusActive, Mobile: &mobile},
	}
	for i := range cases {
		if err := db.Create(&cases[i]).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.First(&cases[i], cases[i].ID).Error; err != nil {
			t.Fatal(err)
		}
	}
	registrationRequest(t, router, "PUT", "/api/profile", registrationToken(t, student), map[string]any{"fullName": "دانشجو", "phone": "09129999999"}, http.StatusOK)
	for _, before := range cases {
		var after model.InternshipCase
		if err := db.First(&after, before.ID).Error; err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("case %d changed after account phone update", before.ID)
		}
	}
}

func TestProfileRestrictedCompanyRemainsRestricted(t *testing.T) {
	db := registrationTestDB(t)
	router := registrationRouter(t, db)
	for _, status := range []model.CompanyRegistrationStatus{model.CompanyRegistrationStatusPending, model.CompanyRegistrationStatusRejected, model.CompanyRegistrationStatusApproved} {
		t.Run(string(status), func(t *testing.T) {
			input, company, public, token := registrationFixture(t, router, "personal-"+string(status))
			if err := db.Model(&company).Update("registration_status", status).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.First(&company, company.ID).Error; err != nil {
				t.Fatal(err)
			}
			registrationRequest(t, router, "GET", "/api/auth/me", token, nil, http.StatusOK)
			registrationRequest(t, router, "PUT", "/api/profile", token, map[string]any{"fullName": "نام شخصی", "phone": "شخصی", "jobTitle": "سمت شخصی"}, http.StatusOK)
			registrationRequest(t, router, "POST", "/api/profile/change-password", token, map[string]string{"currentPassword": input.Supervisor.Password, "newPassword": "Personal123!"}, http.StatusOK)
			if status != model.CompanyRegistrationStatusApproved {
				for _, path := range []string{"/api/company/opportunities", "/api/company/opportunities/1/applications", "/api/company/opportunity-applications/1", "/api/company/internship-cases", "/api/company/internship-cases/1", "/api/companies"} {
					registrationRequest(t, router, "GET", path, token, nil, http.StatusForbidden)
				}
				registrationRequest(t, router, "POST", "/api/company/opportunities", token, map[string]any{}, http.StatusForbidden)
				registrationRequest(t, router, "POST", "/api/company/opportunity-applications/1/accept", token, map[string]any{}, http.StatusForbidden)
			}
			registrationRequest(t, router, "GET", "/api/company/profile", token, nil, http.StatusOK)
			var after model.Company
			if err := db.First(&after, company.ID).Error; err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(company, after) {
				t.Fatal("personal profile/password altered company registration data")
			}
			owner := profileUser(t, db, public.ID)
			if owner.CompanyID == nil || *owner.CompanyID != company.ID {
				t.Fatal("company link changed")
			}
		})
	}
}

func TestProfilePasswordChangeAllRoles(t *testing.T) {
	db := registrationTestDB(t)
	router := registrationRouter(t, db)
	for _, role := range profileTestRoles {
		t.Run(role, func(t *testing.T) {
			owner := registrationDemoUser(t, db, role)
			other := registrationDemoUser(t, db, "admin")
			if owner.ID == other.ID {
				other = registrationDemoUser(t, db, "student")
			}
			token := registrationToken(t, owner)
			change := func(current, next string, status int) {
				t.Helper()
				response := registrationRequest(t, router, "POST", "/api/profile/change-password", token, map[string]string{"currentPassword": current, "newPassword": next}, status)
				if (next != "" && strings.Contains(response.Body.String(), next)) || strings.Contains(response.Body.String(), owner.PasswordHash) {
					t.Fatal("password information leaked")
				}
			}
			change("wrong", "Different123!", http.StatusBadRequest)
			for _, invalid := range []string{"", " \t ", strings.Repeat("x", 73), strings.Repeat("ن", 37)} {
				change("Demo123!", invalid, http.StatusBadRequest)
			}
			change("Demo123!", "Demo123!", http.StatusBadRequest)
			for _, protected := range []string{"userId", "id", "email", "role", "passwordHash", "confirmNewPassword"} {
				registrationRequest(t, router, "POST", "/api/profile/change-password", token, map[string]any{"currentPassword": "Demo123!", "newPassword": "Different123!", protected: other.ID}, http.StatusBadRequest)
			}
			if !reflect.DeepEqual(owner, profileUser(t, db, owner.ID)) {
				t.Fatal("failed password change mutated account")
			}
			newPassword := " NewPassword123! "
			registrationRequest(t, router, "POST", fmt.Sprintf("/api/profile/change-password?userId=%d", other.ID), token, map[string]string{"currentPassword": "Demo123!", "newPassword": newPassword}, http.StatusOK)
			stored := profileUser(t, db, owner.ID)
			if stored.PasswordHash == newPassword || stored.PasswordHash == owner.PasswordHash || !auth.CheckPassword(stored.PasswordHash, newPassword) || auth.CheckPassword(stored.PasswordHash, "Demo123!") {
				t.Fatal("new password not securely stored")
			}
			stored.PasswordHash = owner.PasswordHash
			stored.UpdatedAt = owner.UpdatedAt
			if !reflect.DeepEqual(stored, owner) {
				t.Fatal("password change modified profile fields")
			}
			if !reflect.DeepEqual(other, profileUser(t, db, other.ID)) {
				t.Fatal("another user's password/profile changed")
			}
			registrationRequest(t, router, "POST", "/api/auth/login", "", map[string]string{"email": owner.Email, "password": "Demo123!"}, http.StatusUnauthorized)
			registrationRequest(t, router, "POST", "/api/auth/login", "", map[string]string{"email": owner.Email, "password": newPassword}, http.StatusOK)
			// Existing JWTs remain valid under the unchanged session architecture.
			registrationRequest(t, router, "GET", "/api/auth/me", token, nil, http.StatusOK)
			change("Demo123!", "ThirdPassword123!", http.StatusBadRequest)
		})
	}
}

func TestProfileGeneratedPasswordWorks(t *testing.T) {
	db := registrationTestDB(t)
	router := registrationRouter(t, db)
	management := service.NewUniversityManagementService(db)
	for _, role := range []model.Role{model.RoleStudent, model.RoleProfessor} {
		user, password, err := management.CreateManagedUser(role, service.ManagedUserInput{FullName: "کاربر جدید", Email: fmt.Sprintf("profile-%s@example.test", role), StudentNumber: "m21-generated", Major: "کامپیوتر"})
		if err != nil {
			t.Fatal(err)
		}
		registrationRequest(t, router, "POST", "/api/profile/change-password", registrationToken(t, *user), map[string]string{"currentPassword": password, "newPassword": "OwnPassword123!"}, http.StatusOK)
		registrationRequest(t, router, "POST", "/api/auth/login", "", map[string]string{"email": user.Email, "password": password}, http.StatusUnauthorized)
		registrationRequest(t, router, "POST", "/api/auth/login", "", map[string]string{"email": user.Email, "password": "OwnPassword123!"}, http.StatusOK)
	}
}
