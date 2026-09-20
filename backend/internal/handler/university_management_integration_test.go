package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"internship-management-system/backend/internal/auth"
	"internship-management-system/backend/internal/model"
	"internship-management-system/backend/internal/service"
)

func TestUniversityXLSXImportsContinueAfterBadRows(t *testing.T) {
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

	management := service.NewUniversityManagementService(tx)
	handler := NewUniversityManagementHandler(management)
	suffix := time.Now().UnixNano()
	duplicateEmail := fmt.Sprintf("import-duplicate-%d@example.test", suffix)
	if _, _, err := management.CreateManagedUser(model.RoleProfessor, service.ManagedUserInput{FullName: "موجود", Email: duplicateEmail}); err != nil {
		t.Fatalf("create duplicate fixture: %v", err)
	}

	studentEmail := fmt.Sprintf("import-student-%d@example.test", suffix)
	studentResult := performXLSXImport(t, handler.ImportStudents, [][]string{
		{"نام و نام خانوادگی", "ایمیل", "شماره دانشجویی", "رشته"},
		{"دانشجوی واردشده", studentEmail, fmt.Sprintf("xlsx-%d", suffix), "مهندسی کامپیوتر"},
		{"تکراری", duplicateEmail, fmt.Sprintf("xlsx-dup-%d", suffix), "مهندسی کامپیوتر"},
		{"ناقص", "", "", ""},
	})
	studentPassword := assertImportSummaryAndCredential(t, tx, studentResult, studentEmail, 1, 2)
	assertTemporaryCredentialLogin(t, tx, studentEmail, studentPassword)

	professorEmail := fmt.Sprintf("import-professor-%d@example.test", suffix)
	professorResult := performXLSXImport(t, handler.ImportProfessors, [][]string{
		{"نام و نام خانوادگی", "ایمیل"},
		{"استاد واردشده", professorEmail},
		{"تکراری", duplicateEmail},
	})
	professorPassword := assertImportSummaryAndCredential(t, tx, professorResult, professorEmail, 1, 1)
	assertTemporaryCredentialLogin(t, tx, professorEmail, professorPassword)
}

func performXLSXImport(t *testing.T, action gin.HandlerFunc, rows [][]string) importResponse {
	t.Helper()
	workbook := excelize.NewFile()
	sheet := workbook.GetSheetName(0)
	for rowIndex, row := range rows {
		for columnIndex, value := range row {
			cellName, err := excelize.CoordinatesToCellName(columnIndex+1, rowIndex+1)
			if err != nil {
				t.Fatalf("build cell name: %v", err)
			}
			if err := workbook.SetCellValue(sheet, cellName, value); err != nil {
				t.Fatalf("write workbook cell: %v", err)
			}
		}
	}
	var xlsx bytes.Buffer
	if err := workbook.Write(&xlsx); err != nil {
		t.Fatalf("write xlsx: %v", err)
	}
	if err := workbook.Close(); err != nil {
		t.Fatalf("close xlsx: %v", err)
	}

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	part, err := writer.CreateFormFile("file", "import.xlsx")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write(xlsx.Bytes()); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart request: %v", err)
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/import", &requestBody)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	action(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("import status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var result importResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	return result
}

func assertImportSummaryAndCredential(t *testing.T, db *gorm.DB, result importResponse, email string, created, skipped int) string {
	t.Helper()
	if result.Created != created || result.Skipped != skipped {
		t.Fatalf("import summary = created %d skipped %d, want %d/%d: %+v", result.Created, result.Skipped, created, skipped, result)
	}
	var createdRow *importRowResult
	for index := range result.Results {
		if result.Results[index].Email == email && result.Results[index].Status == "CREATED" {
			createdRow = &result.Results[index]
			break
		}
	}
	if createdRow == nil || createdRow.TemporaryPassword == "" {
		t.Fatalf("created row credential missing: %+v", result)
	}
	var user model.User
	if err := db.Where("email = ?", email).First(&user).Error; err != nil {
		t.Fatalf("load imported user: %v", err)
	}
	if !auth.CheckPassword(user.PasswordHash, createdRow.TemporaryPassword) {
		t.Fatal("imported temporary password does not match stored bcrypt hash")
	}
	return createdRow.TemporaryPassword
}

func assertTemporaryCredentialLogin(t *testing.T, db *gorm.DB, email, password string) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"email": email, "password": password})
	if err != nil {
		t.Fatalf("encode login request: %v", err)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	NewAuthHandler(db, "integration-test-secret", time.Hour).Login(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("temporary credential login status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte("passwordHash")) || bytes.Contains(recorder.Body.Bytes(), []byte("password_hash")) {
		t.Fatalf("login response exposed password hash: %s", recorder.Body.String())
	}
}
