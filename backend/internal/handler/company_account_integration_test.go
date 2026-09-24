package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"internship-management-system/backend/internal/auth"
	appmiddleware "internship-management-system/backend/internal/middleware"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

func TestCompanySelfRegistrationEndToEnd(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect to integration database: %v", err)
	}
	if err := db.AutoMigrate(&model.Company{}, &model.User{}); err != nil {
		t.Fatalf("migrate integration database: %v", err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin test transaction: %v", tx.Error)
	}
	defer tx.Rollback()

	const jwtSecret = "company-registration-integration-secret"
	companyAccounts := service.NewCompanyAccountService(tx)
	companyHandler := NewCompanyAccountHandler(companyAccounts)
	authHandler := NewAuthHandler(tx, jwtSecret, time.Hour)
	router := gin.New()
	router.POST("/api/auth/company-register", companyHandler.Register)
	router.POST("/api/auth/login", authHandler.Login)
	company := router.Group("/api/company")
	company.Use(appmiddleware.RequireAuth(jwtSecret), appmiddleware.RequireRole(model.RoleCompanySupervisor))
	company.GET("/profile", companyHandler.Profile)

	suffix := time.Now().UnixNano()
	request := companyRegistrationFixture(suffix, "success")
	registration := performJSONRequest(t, router, http.MethodPost, "/api/auth/company-register", request, "")
	if registration.Code != http.StatusCreated {
		t.Fatalf("registration status = %d, body = %s", registration.Code, registration.Body.String())
	}
	assertNoPasswordData(t, registration.Body.String())

	var registeredCompany model.Company
	if err := tx.Where("national_id = ?", strings.TrimSpace(request.Company.NationalID)).First(&registeredCompany).Error; err != nil {
		t.Fatalf("load registered company: %v", err)
	}
	if registeredCompany.IsApproved {
		t.Fatal("self-registered company must not be faculty-approved")
	}
	if registeredCompany.EconomicCode != strings.TrimSpace(request.Company.EconomicCode) || registeredCompany.Phone == nil ||
		*registeredCompany.Phone != strings.TrimSpace(request.Company.Phone) {
		t.Fatalf("registered company fields were not persisted: %+v", registeredCompany)
	}

	var registeredSupervisor model.User
	if err := tx.Where("email = ?", strings.ToLower(request.Supervisor.Email)).First(&registeredSupervisor).Error; err != nil {
		t.Fatalf("load registered supervisor: %v", err)
	}
	if registeredSupervisor.Role != model.RoleCompanySupervisor || registeredSupervisor.CompanyID == nil || *registeredSupervisor.CompanyID != registeredCompany.ID {
		t.Fatalf("registered supervisor relationship is invalid: %+v", registeredSupervisor)
	}
	if registeredSupervisor.PasswordHash == request.Supervisor.Password || !auth.CheckPassword(registeredSupervisor.PasswordHash, request.Supervisor.Password) {
		t.Fatal("registered password was not stored as a matching bcrypt hash")
	}
	if registeredSupervisor.Phone == nil || *registeredSupervisor.Phone != strings.TrimSpace(request.Supervisor.Phone) || registeredSupervisor.JobTitle == nil ||
		*registeredSupervisor.JobTitle != strings.TrimSpace(request.Supervisor.JobTitle) {
		t.Fatalf("registered supervisor fields were not persisted: %+v", registeredSupervisor)
	}

	login := performJSONRequest(t, router, http.MethodPost, "/api/auth/login", map[string]string{
		"email": request.Supervisor.Email, "password": request.Supervisor.Password,
	}, "")
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", login.Code, login.Body.String())
	}
	assertNoPasswordData(t, login.Body.String())
	var loginResponse struct {
		Token string           `json:"token"`
		User  model.PublicUser `json:"user"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &loginResponse); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if loginResponse.Token == "" || loginResponse.User.Role != model.RoleCompanySupervisor || loginResponse.User.CompanyID == nil || *loginResponse.User.CompanyID != registeredCompany.ID {
		t.Fatalf("login response does not contain linked company supervisor: %+v", loginResponse)
	}

	profile := performJSONRequest(t, router, http.MethodGet, "/api/company/profile", nil, loginResponse.Token)
	if profile.Code != http.StatusOK {
		t.Fatalf("unapproved company profile status = %d, body = %s", profile.Code, profile.Body.String())
	}
	assertNoPasswordData(t, profile.Body.String())
	var profileResponse companyAccountResponse
	if err := json.Unmarshal(profile.Body.Bytes(), &profileResponse); err != nil {
		t.Fatalf("decode company profile: %v", err)
	}
	if profileResponse.Company.ID != registeredCompany.ID || profileResponse.Company.IsApproved || profileResponse.Supervisor.ID != registeredSupervisor.ID {
		t.Fatalf("unexpected company profile response: %+v", profileResponse)
	}

	t.Run("duplicate supervisor email rolls back company", func(t *testing.T) {
		duplicate := companyRegistrationFixture(suffix, "duplicate-email")
		duplicate.Supervisor.Email = request.Supervisor.Email
		response := performJSONRequest(t, router, http.MethodPost, "/api/auth/company-register", duplicate, "")
		assertAPIError(t, response, http.StatusConflict, "EMAIL_ALREADY_EXISTS")
		assertCompanyDoesNotExist(t, tx, duplicate.Company.NationalID)
	})

	t.Run("duplicate national id does not persist supervisor", func(t *testing.T) {
		duplicate := companyRegistrationFixture(suffix, "duplicate-national")
		duplicate.Company.NationalID = request.Company.NationalID
		response := performJSONRequest(t, router, http.MethodPost, "/api/auth/company-register", duplicate, "")
		assertAPIError(t, response, http.StatusConflict, "NATIONAL_ID_ALREADY_EXISTS")
		assertUserDoesNotExist(t, tx, duplicate.Supervisor.Email)
	})

	t.Run("duplicate economic code does not persist supervisor", func(t *testing.T) {
		duplicate := companyRegistrationFixture(suffix, "duplicate-economic")
		duplicate.Company.EconomicCode = request.Company.EconomicCode
		response := performJSONRequest(t, router, http.MethodPost, "/api/auth/company-register", duplicate, "")
		assertAPIError(t, response, http.StatusConflict, "ECONOMIC_CODE_ALREADY_EXISTS")
		assertUserDoesNotExist(t, tx, duplicate.Supervisor.Email)
	})

	t.Run("missing required field persists neither record", func(t *testing.T) {
		invalid := companyRegistrationFixture(suffix, "missing-field")
		invalid.Supervisor.JobTitle = "  "
		response := performJSONRequest(t, router, http.MethodPost, "/api/auth/company-register", invalid, "")
		assertAPIError(t, response, http.StatusBadRequest, "INVALID_REGISTRATION")
		assertCompanyDoesNotExist(t, tx, invalid.Company.NationalID)
		assertUserDoesNotExist(t, tx, invalid.Supervisor.Email)
	})
}

func TestCompanyAccountErrorsDoNotExposeInternalDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	writeCompanyAccountError(ctx, fmt.Errorf("pq: duplicate key contains secret connection details"))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if strings.Contains(strings.ToLower(recorder.Body.String()), "duplicate key") || strings.Contains(strings.ToLower(recorder.Body.String()), "pq:") {
		t.Fatalf("internal database error leaked: %s", recorder.Body.String())
	}
}

func companyRegistrationFixture(suffix int64, label string) companyRegistrationRequest {
	key := fmt.Sprintf("%d-%s", suffix, label)
	return companyRegistrationRequest{
		Supervisor: companySupervisorRegistrationRequest{
			FullName: " سرپرست آزمون ",
			Email:    fmt.Sprintf("Company-%s@Example.Test", key),
			Password: "Registration123!",
			Phone:    " 09120000000 ",
			JobTitle: " مدیر منابع انسانی ",
		},
		Company: companyRegistrationDetailsRequest{
			Name:         fmt.Sprintf(" شرکت آزمون %s ", key),
			NationalID:   fmt.Sprintf(" national-%s ", key),
			EconomicCode: fmt.Sprintf(" economic-%s ", key),
			Website:      " https://example.test ",
			Phone:        " 02100000000 ",
			Email:        fmt.Sprintf(" Office-%s@Example.Test", key),
			Address:      " تهران، نشانی آزمون ",
		},
	}
}

func performJSONRequest(t *testing.T, router http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func assertAPIError(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, status, recorder.Body.String())
	}
	var response struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response.Code != code || response.Error == "" {
		t.Fatalf("error response = %+v, want code %s", response, code)
	}
}

func assertCompanyDoesNotExist(t *testing.T, db *gorm.DB, nationalID string) {
	t.Helper()
	var count int64
	if err := db.Model(&model.Company{}).Where("national_id = ?", strings.TrimSpace(nationalID)).Count(&count).Error; err != nil {
		t.Fatalf("count attempted company: %v", err)
	}
	if count != 0 {
		t.Fatalf("attempted company persisted after failed registration: national ID %q", nationalID)
	}
}

func assertUserDoesNotExist(t *testing.T, db *gorm.DB, email string) {
	t.Helper()
	var count int64
	if err := db.Model(&model.User{}).Where("email = ?", strings.ToLower(strings.TrimSpace(email))).Count(&count).Error; err != nil {
		t.Fatalf("count attempted supervisor: %v", err)
	}
	if count != 0 {
		t.Fatalf("attempted supervisor persisted after failed registration: email %q", email)
	}
}

func assertNoPasswordData(t *testing.T, body string) {
	t.Helper()
	lower := strings.ToLower(body)
	if strings.Contains(lower, "passwordhash") || strings.Contains(lower, "password_hash") || strings.Contains(body, "Registration123!") {
		t.Fatalf("response exposed password data: %s", body)
	}
}
